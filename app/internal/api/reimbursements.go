package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
	"github.com/nightgopher/cc4-journey/app/internal/service"
)

var receiptSlots = make(chan struct{}, 2)
var receiptPages = regexp.MustCompile(`(?m)^Pages:\s+([0-9]+)`)
var receiptFileName = regexp.MustCompile(`^[a-f0-9]{32}\.(png|jpg|pdf)$`)

func (h *apiHandler) registerReimbursements(r chi.Router) {
	r.Get("/reimbursements/config", h.reimbursementConfig)
	r.Get("/reimbursements", h.listReimbursements)
	r.Post("/reimbursements", h.uploadReimbursement)
	r.Get("/reimbursements/{id}", h.getReimbursement)
	r.Put("/reimbursements/{id}", h.updateReimbursement)
	r.Get("/reimbursements/{id}/receipt", h.reimbursementReceipt)
	r.Get("/reimbursements/{id}/events", h.reimbursementEvents)
	r.Post("/reimbursements/{id}/recognize", h.retryReimbursement)
	for _, action := range []string{"submit", "approve", "reject", "withdraw", "cancel", "pay"} {
		r.Post("/reimbursements/{id}/"+action, h.reimbursementAction(action))
	}
}

func reimbursementActor(r *http.Request) (repo.ClaimActor, error) {
	uid, role := mustUser(r.Context())
	e, err := repo.GetEmployeeByUserID(r.Context(), uid)
	if err != nil {
		return repo.ClaimActor{}, repo.ErrForbidden
	}
	return repo.ClaimActor{ID: e.ID, Role: role, TeamID: e.TeamID}, nil
}
func reimbursementError(w http.ResponseWriter, err error) {
	if errors.Is(err, repo.ErrClaimConflict) {
		writeErr(w, 409, err.Error())
		return
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && strings.HasPrefix(pg.ConstraintName, "reimbursement_") {
		writeErr(w, 409, "票据文件或票据编码/号码已存在，请勿重复报销")
		return
	}
	dbError(w, err)
}
func (h *apiHandler) reimbursementConfig(w http.ResponseWriter, r *http.Request) {
	client, err := service.PaddleFromEnv()
	configured := err == nil && client.Configured()
	model := service.PaddleModels[0]
	if client != nil {
		model = client.Model
	}
	writeJSON(w, 200, map[string]any{"configured": configured, "provider": "paddleocr", "model": model, "formats": []string{"image/png", "image/jpeg", "application/pdf"}, "maxBytes": 8 << 20, "maxPages": 1})
}
func (h *apiHandler) listReimbursements(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	q := r.URL.Query()
	scope, status := q.Get("scope"), q.Get("status")
	if scope == "" {
		scope = "mine"
	}
	if scope != "mine" && scope != "review" && scope != "all" {
		writeErr(w, 400, "列表范围无效")
		return
	}
	if status != "" && !hasRole(status, "draft", "submitted", "approved", "rejected", "paid", "cancelled") {
		writeErr(w, 400, "报销状态无效")
		return
	}
	page, limit, offset := pageParams(r)
	rows, total, err := repo.Claims(r.Context(), a, scope, status, limit, offset)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"claims": rows, "pagination": pagination(page, limit, total)})
}
func (h *apiHandler) getReimbursement(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效编号")
		return
	}
	row, err := repo.Claim(r.Context(), a, id)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"claim": row})
}
func (h *apiHandler) uploadReimbursement(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	select {
	case receiptSlots <- struct{}{}:
		defer func() { <-receiptSlots }()
	case <-r.Context().Done():
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err = r.ParseMultipartForm(9 << 20); err != nil {
		writeErr(w, 400, "上传内容无效或超过8MiB限制")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("receipt")
	if err != nil {
		writeErr(w, 400, "请选择票据文件")
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil || len(raw) == 0 || len(raw) > 8<<20 {
		writeErr(w, 400, "票据为空或超过8MiB")
		return
	}
	mime := http.DetectContentType(raw)
	extension := ""
	switch mime {
	case "image/png", "image/jpeg":
		config, _, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 20_000_000 {
			writeErr(w, 400, "图片无效或超过2000万像素")
			return
		}
		if _, _, e = image.Decode(bytes.NewReader(raw)); e != nil {
			writeErr(w, 400, "图片内容损坏，请重新上传")
			return
		}
		extension = ".png"
		if mime == "image/jpeg" {
			extension = ".jpg"
		}
	case "application/pdf":
		extension = ".pdf"
	default:
		writeErr(w, 400, "只支持PNG、JPEG或单页PDF票据")
		return
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		writeErr(w, 500, "文件保存失败")
		return
	}
	name := hex.EncodeToString(random[:]) + extension
	directory := filepath.Join(uploadDir(), "receipts")
	if err = os.MkdirAll(directory, 0700); err != nil {
		writeErr(w, 500, "文件保存失败")
		return
	}
	path := filepath.Join(directory, name)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		writeErr(w, 500, "文件保存失败")
		return
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(path)
		}
	}()
	if extension == ".pdf" {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pdfinfo", path)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		info, e := cmd.Output()
		pages := receiptPages.FindSubmatch(info)
		if e != nil || len(pages) != 2 {
			writeErr(w, 400, "PDF无法读取，请换成清晰图片或有效单页PDF")
			return
		}
		count, _ := strconv.Atoi(string(pages[1]))
		if count != 1 {
			writeErr(w, 400, "每笔报销只接收单页票据，请拆分PDF")
			return
		}
	}
	client, e := service.PaddleFromEnv()
	model := service.PaddleModels[0]
	if client != nil {
		model = client.Model
	}
	hash := sha256.Sum256(raw)
	id, err := repo.CreateClaim(r.Context(), a, name, mime, hex.EncodeToString(hash[:]), model, e == nil && client.Configured())
	if err != nil {
		reimbursementError(w, err)
		return
	}
	keep = true
	row, err := repo.Claim(r.Context(), a, id)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"claim": row, "id": id})
}
func (h *apiHandler) updateReimbursement(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效编号")
		return
	}
	var in repo.ClaimInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil {
		writeErr(w, 400, "字段无效，不允许直接修改状态、申请人或财务关联")
		return
	}
	if err = repo.UpdateClaim(r.Context(), a, id, in); err != nil {
		reimbursementError(w, err)
		return
	}
	h.getReimbursement(w, r)
}
func (h *apiHandler) reimbursementAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := reimbursementActor(r)
		if err != nil {
			reimbursementError(w, err)
			return
		}
		id, err := pathID(r)
		if err != nil {
			writeErr(w, 400, "无效编号")
			return
		}
		var in struct {
			Confirmed   bool   `json:"confirmed"`
			Note        string `json:"note"`
			PaymentDate string `json:"paymentDate"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil {
			writeErr(w, 400, "操作参数无效")
			return
		}
		if err = repo.DecideClaim(r.Context(), a, id, action, in.Note, in.PaymentDate, service.BusinessNow().Format("2006-01-02"), in.Confirmed); err != nil {
			reimbursementError(w, err)
			return
		}
		h.getReimbursement(w, r)
	}
}
func (h *apiHandler) retryReimbursement(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效编号")
		return
	}
	client, err := service.PaddleFromEnv()
	if err != nil || !client.Configured() {
		writeErr(w, 503, "识别服务尚未就绪，请联系管理员")
		return
	}
	if err = repo.RetryClaimOCR(r.Context(), a, id, client.Model); err != nil {
		reimbursementError(w, err)
		return
	}
	h.getReimbursement(w, r)
}
func (h *apiHandler) reimbursementReceipt(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效编号")
		return
	}
	file, mime, err := repo.ClaimFile(r.Context(), a, id)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	if !receiptFileName.MatchString(file) {
		writeErr(w, 404, "票据不存在")
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", `inline; filename="receipt`+filepath.Ext(file)+`"`)
	http.ServeFile(w, r, filepath.Join(uploadDir(), "receipts", file))
}
func (h *apiHandler) reimbursementEvents(w http.ResponseWriter, r *http.Request) {
	a, err := reimbursementActor(r)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "无效编号")
		return
	}
	if _, err = repo.Claim(r.Context(), a, id); err != nil {
		reimbursementError(w, err)
		return
	}
	rows, err := repo.ClaimEvents(r.Context(), id)
	if err != nil {
		reimbursementError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"events": rows})
}
