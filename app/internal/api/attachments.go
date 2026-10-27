package api

import (
	"bytes"
	"errors"
	"net/http"
	"path"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/storage"
)

func writeUploadError(w http.ResponseWriter, err error) {
	status := 400
	if errors.Is(err, storage.ErrUnavailable) {
		status = 503
	}
	writeErr(w, status, err.Error())
}

func serveAttachment(w http.ResponseWriter, r *http.Request, key, mime string) {
	store, err := storage.FromEnv(storage.Root())
	if err != nil {
		writeErr(w, 503, "附件存储配置无效")
		return
	}
	raw, err := store.Read(r.Context(), key)
	if errors.Is(err, storage.ErrNotFound) {
		writeErr(w, 404, "文件不存在")
		return
	}
	if err != nil {
		writeErr(w, 503, storage.ErrUnavailable.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	http.ServeContent(w, r, path.Base(key), time.Time{}, bytes.NewReader(raw))
}
