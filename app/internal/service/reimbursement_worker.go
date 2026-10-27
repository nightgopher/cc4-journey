package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/storage"
)

func receiptDirectory() string {
	base := os.Getenv("UPLOAD_DIR")
	if base == "" {
		base = "uploads"
	}
	return filepath.Join(base, "receipts")
}

func StartReimbursementWorker(ctx context.Context) {
	client, err := PaddleFromEnv()
	if err != nil || !client.Configured() {
		return
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				work, cancel := context.WithTimeout(ctx, 60*time.Second)
				if _, err := RunReimbursementOCR(work, client, receiptDirectory()); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("reimbursement OCR worker: %v", err)
				}
				cancel()
			}
		}
	}()
}

// One durable step per invocation: POST is never automatically replayed.
func RunReimbursementOCR(ctx context.Context, client *PaddleClient, directory string) (bool, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return false, err
	}
	j, err := repo.AcquireOCRJob(ctx, hex.EncodeToString(token[:]))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fail := func(message string) error { return repo.SaveOCRJob(ctx, j, "failed", nil, nil, nil, message) }
	if j.Started != nil && time.Since(*j.Started) > 10*time.Minute {
		return true, fail("识别超过10分钟，请稍后重试")
	}
	if j.State == "submitting" {
		return true, fail("上次识别提交中断，未自动重发；请核对后点击重新识别")
	}
	copyClient := *client
	copyClient.Model = j.Model
	if j.State == "queued" {
		store, err := storage.FromEnv(filepath.Dir(directory))
		if err != nil {
			return true, fail("附件存储配置无效，未提交识别")
		}
		raw, err := store.Read(ctx, "receipts/"+j.File)
		if err != nil {
			return true, fail("票据无法从附件存储读取，未提交识别")
		}
		id, err := copyClient.SubmitBytes(ctx, j.File, raw)
		if err != nil {
			return true, fail(paddleErrorText(err) + "；未自动重发")
		}
		return true, repo.SaveOCRJob(ctx, j, "running", &id, repo.Object{"state": "pending"}, nil, "")
	}
	if j.JobID == nil {
		return true, fail("识别任务编号缺失，请重试")
	}
	progress, err := copyClient.Poll(ctx, *j.JobID)
	if err != nil {
		var provider *PaddleError
		if errors.As(err, &provider) && provider.Retryable && j.Attempts < 4 {
			return true, repo.SaveOCRJob(ctx, j, "running", nil, nil, nil, "识别服务暂时不可用，正在重试查询")
		}
		return true, fail(paddleErrorText(err))
	}
	state := "running"
	if progress.Recognition != nil {
		state = "ready"
	}
	return true, repo.SaveOCRJob(ctx, j, state, nil, repo.Object{"state": progress.State, "totalPages": progress.Total, "extractedPages": progress.Extracted}, progress.Recognition, "")
}
