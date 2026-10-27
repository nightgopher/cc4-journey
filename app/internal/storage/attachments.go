// Package storage keeps attachment URLs stable while the bytes live in a private
// TOS prefix or a local development directory. Authorization belongs to callers.
package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

const MaxBytes = 8 << 20

var (
	ErrNotFound    = errors.New("附件不存在")
	ErrUnavailable = errors.New("附件存储暂不可用，请稍后重试")
	ErrExists      = errors.New("附件已存在，禁止覆盖")
	validKey       = regexp.MustCompile(`^(?:receipts/)?[a-f0-9]{32}\.(?:png|jpg|webp|pdf)$`)
	clientCache    struct {
		sync.Mutex
		fingerprint [32]byte
		client      *tos.ClientV2
	}
)

type Store struct {
	root, bucket, prefix string
	client               *tos.ClientV2
}

func Root() string {
	if value := os.Getenv("UPLOAD_DIR"); value != "" {
		return value
	}
	return "uploads"
}

// FromEnv does not silently downgrade an invalid cloud configuration to disk.
// Clients are shared to avoid creating a DNS cache/connection pool per request.
func FromEnv(root string) (*Store, error) {
	s := &Store{root: root}
	mode := os.Getenv("ATTACHMENT_STORAGE")
	if mode == "" || mode == "local" {
		return s, nil
	}
	if mode != "tos" {
		return nil, errors.New("ATTACHMENT_STORAGE只能为local或tos")
	}
	endpoint, region := os.Getenv("TOS_ENDPOINT"), os.Getenv("TOS_REGION")
	s.bucket, s.prefix = os.Getenv("TOS_BUCKET"), os.Getenv("TOS_KEY_PREFIX")
	ak, sk, token := os.Getenv("TOS_ACCESS_KEY"), os.Getenv("TOS_SECRET_KEY"), os.Getenv("TOS_SESSION_TOKEN")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		(!strings.HasSuffix(u.Hostname(), ".volces.com") && !strings.HasSuffix(u.Hostname(), ".ivolces.com")) ||
		region == "" || s.bucket == "" || ak == "" || sk == "" || s.prefix == "" || !strings.HasSuffix(s.prefix, "/") ||
		strings.HasPrefix(s.prefix, "/") || strings.Contains(s.prefix, "..") || strings.ContainsAny(s.prefix, "\\\r\n\x00") {
		return nil, errors.New("TOS配置不完整或无效")
	}
	fingerprint := sha256.Sum256([]byte(strings.Join([]string{endpoint, region, ak, sk, token}, "\x00")))
	clientCache.Lock()
	defer clientCache.Unlock()
	if clientCache.client == nil || clientCache.fingerprint != fingerprint {
		credentials := tos.NewStaticCredentials(ak, sk)
		credentials.WithSecurityToken(token)
		client, err := tos.NewClientV2(endpoint, tos.WithRegion(region), tos.WithCredentials(credentials), tos.WithRequestTimeout(25*time.Second), tos.WithMaxRetryCount(1))
		if err != nil {
			return nil, errors.New("TOS客户端配置无效")
		}
		if clientCache.client != nil {
			clientCache.client.Close()
		}
		clientCache.client, clientCache.fingerprint = client, fingerprint
	}
	s.client = clientCache.client
	return s, nil
}

func (s *Store) Remote() bool                { return s.client != nil }
func (s *Store) ObjectKey(key string) string { return s.prefix + key }

func storageError(err error) error {
	if err == nil {
		return nil
	}
	switch tos.StatusCode(err) {
	case 404:
		return ErrNotFound
	case 409, 412:
		return ErrExists
	default:
		return ErrUnavailable
	}
}

func (s *Store) Put(ctx context.Context, key string, body []byte, mime string) error {
	if !validKey.MatchString(key) || len(body) == 0 || len(body) > MaxBytes {
		return errors.New("附件名称或大小无效")
	}
	if s.Remote() {
		hash := sha256.Sum256(body)
		_, err := s.client.PutObjectV2(ctx, &tos.PutObjectV2Input{PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket: s.bucket, Key: s.ObjectKey(key), ContentLength: int64(len(body)), ContentType: mime,
			ACL: enum.ACLPrivate, CacheControl: "private, no-store", ForbidOverwrite: true,
			Meta: map[string]string{"cc4-sha256": hex.EncodeToString(hash[:])},
		}, Content: bytes.NewReader(body)})
		return storageError(err)
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ErrUnavailable
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return ErrExists
	}
	if err != nil {
		return ErrUnavailable
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return ErrUnavailable
	}
	return nil
}

func (s *Store) remoteRead(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObjectV2(ctx, &tos.GetObjectV2Input{Bucket: s.bucket, Key: s.ObjectKey(key)})
	if err != nil {
		return nil, storageError(err)
	}
	defer result.Content.Close()
	body, err := io.ReadAll(io.LimitReader(result.Content, MaxBytes+1))
	if err != nil || len(body) > MaxBytes {
		return nil, ErrUnavailable
	}
	return body, nil
}

func (s *Store) Read(ctx context.Context, key string) ([]byte, error) {
	if !validKey.MatchString(key) {
		return nil, ErrNotFound
	}
	if s.Remote() {
		body, err := s.remoteRead(ctx, key)
		// Only a genuine missing object may use a pre-migration local copy.
		// An outage/authentication failure is never hidden by stale disk data.
		if !errors.Is(err, ErrNotFound) {
			return body, err
		}
	}
	path := filepath.Join(s.root, filepath.FromSlash(key))
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	if !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, ErrNotFound
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil || len(body) > MaxBytes {
		return nil, ErrUnavailable
	}
	return body, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if !validKey.MatchString(key) {
		return ErrNotFound
	}
	if s.Remote() {
		_, err := s.client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{Bucket: s.bucket, Key: s.ObjectKey(key)})
		return storageError(err)
	}
	err := os.Remove(filepath.Join(s.root, filepath.FromSlash(key)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// Rollback is bounded and still runs when an upload's HTTP request was canceled.
func (s *Store) Rollback(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.Delete(ctx, key)
}

// Migrate copies regular application attachments and verifies full remote bytes.
// It never deletes local data or overwrites a conflicting object.
func (s *Store) Migrate(ctx context.Context, apply bool) (map[string]int, error) {
	if !s.Remote() {
		return nil, errors.New("迁移需要TOS配置")
	}
	counts := map[string]int{"copied": 0, "verified": 0, "planned": 0, "skipped": 0}
	err := filepath.WalkDir(s.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrUnavailable
		}
		if entry.IsDir() {
			return nil
		}
		key, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		key = filepath.ToSlash(key)
		if !validKey.MatchString(key) || entry.Type()&os.ModeSymlink != 0 {
			counts["skipped"]++
			return nil
		}
		if !apply {
			counts["planned"]++
			return nil
		}
		local, err := FromLocal(s.root).Read(ctx, key)
		if err != nil {
			return err
		}
		remote, err := s.remoteRead(ctx, key)
		if errors.Is(err, ErrNotFound) {
			mime := "image/png"
			switch filepath.Ext(key) {
			case ".jpg":
				mime = "image/jpeg"
			case ".webp":
				mime = "image/webp"
			case ".pdf":
				mime = "application/pdf"
			}
			if err = s.Put(ctx, key, local, mime); err != nil {
				return err
			}
			counts["copied"]++
			remote, err = s.remoteRead(ctx, key)
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(local, remote) {
			return errors.New("迁移发现同名对象内容不一致，已停止")
		}
		counts["verified"]++
		return nil
	})
	return counts, err
}

func FromLocal(root string) *Store { return &Store{root: root} }
