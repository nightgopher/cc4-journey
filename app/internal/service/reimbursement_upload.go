package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/storage"
)

var receiptPreflightSlots = make(chan struct{}, 2)
var receiptPages = regexp.MustCompile(`(?m)^Pages:\s+([0-9]+)`)

// CreateReimbursement is shared by the authenticated workspace and personal MCP.
func CreateReimbursement(ctx context.Context, actor repo.ClaimActor, raw []byte) (int, error) {
	if actor.ID <= 0 {
		return 0, repo.ErrForbidden
	}
	select {
	case receiptPreflightSlots <- struct{}{}:
		defer func() { <-receiptPreflightSlots }()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	if len(raw) == 0 || len(raw) > storage.MaxBytes {
		return 0, errors.New("票据为空或超过8MiB")
	}
	mime, ext := http.DetectContentType(raw), ""
	switch mime {
	case "image/png", "image/jpeg":
		config, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 20_000_000 {
			return 0, errors.New("图片无效或超过2000万像素")
		}
		if _, _, err = image.Decode(bytes.NewReader(raw)); err != nil {
			return 0, errors.New("图片内容损坏，请重新上传")
		}
		ext = ".png"
		if mime == "image/jpeg" {
			ext = ".jpg"
		}
	case "application/pdf":
		ext = ".pdf"
		file, err := os.CreateTemp("", "cc4-receipt-*.pdf")
		if err != nil {
			return 0, errors.New("PDF校验暂不可用")
		}
		defer os.Remove(file.Name())
		_, err = file.Write(raw)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return 0, errors.New("PDF校验暂不可用")
		}
		check, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(check, "pdfinfo", file.Name())
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		info, err := cmd.Output()
		pages := receiptPages.FindSubmatch(info)
		if err != nil || len(pages) != 2 {
			return 0, errors.New("PDF无法读取，请换成清晰图片或有效单页PDF")
		}
		count, _ := strconv.Atoi(string(pages[1]))
		if count != 1 {
			return 0, errors.New("每笔报销只接收单页票据，请拆分PDF")
		}
	default:
		return 0, errors.New("只支持PNG、JPEG或单页PDF票据")
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return 0, errors.New("文件保存失败")
	}
	name := hex.EncodeToString(random[:]) + ext
	store, err := storage.FromEnv(storage.Root())
	if err != nil {
		return 0, storage.ErrUnavailable
	}
	key := "receipts/" + name
	if err = store.Put(ctx, key, raw, mime); err != nil {
		return 0, err
	}
	keep := false
	defer func() {
		if !keep {
			store.Rollback(key)
		}
	}()
	client, e := PaddleFromEnv()
	model := PaddleModels[0]
	if client != nil {
		model = client.Model
	}
	hash := sha256.Sum256(raw)
	id, err := repo.CreateClaim(ctx, actor, name, mime, hex.EncodeToString(hash[:]), model, e == nil && client.Configured())
	if err != nil {
		return 0, err
	}
	keep = true
	return id, nil
}
