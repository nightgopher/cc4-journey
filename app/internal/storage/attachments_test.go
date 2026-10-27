package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const receipt = "receipts/0123456789abcdef0123456789abcdef.png"

func TestLocalStoreWritesOnceAndReadsBack(t *testing.T) {
	store := FromLocal(t.TempDir())
	ctx := context.Background()
	if store.Remote() {
		t.Fatal("local store reported remote")
	}
	if err := store.Put(ctx, receipt, []byte("first"), "image/png"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, receipt, []byte("second"), "image/png"); !errors.Is(err, ErrExists) {
		t.Fatalf("overwrite must be refused, got %v", err)
	}
	body, err := store.Read(ctx, receipt)
	if err != nil || !bytes.Equal(body, []byte("first")) {
		t.Fatalf("read back %q, %v", body, err)
	}
	if err = store.Delete(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	if err = store.Delete(ctx, receipt); err != nil {
		t.Fatalf("deleting a missing attachment is idempotent, got %v", err)
	}
	if _, err = store.Read(ctx, receipt); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted attachment must be not found, got %v", err)
	}
}

func TestLocalStoreRejectsUnsafeKeysAndSizes(t *testing.T) {
	root := t.TempDir()
	store := FromLocal(root)
	ctx := context.Background()
	for _, key := range []string{"../escape.png", "receipts/../../x.png", "abc.png", "receipts/0123456789abcdef0123456789abcdef.exe", "/etc/passwd"} {
		if err := store.Put(ctx, key, []byte("x"), "image/png"); err == nil {
			t.Fatalf("put accepted unsafe key %q", key)
		}
		if _, err := store.Read(ctx, key); !errors.Is(err, ErrNotFound) {
			t.Fatalf("read of unsafe key %q returned %v", key, err)
		}
	}
	if err := store.Put(ctx, receipt, nil, "image/png"); err == nil {
		t.Fatal("empty attachment accepted")
	}
	if err := store.Put(ctx, receipt, make([]byte, MaxBytes+1), "image/png"); err == nil {
		t.Fatal("oversized attachment accepted")
	}
}

func TestLocalStoreDoesNotFollowSymlinks(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "receipts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, filepath.FromSlash(receipt))); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if _, err := FromLocal(root).Read(context.Background(), receipt); !errors.Is(err, ErrNotFound) {
		t.Fatalf("symlinked attachment must not be served, got %v", err)
	}
}

func TestFromEnvRefusesIncompleteCloudConfiguration(t *testing.T) {
	valid := map[string]string{
		"ATTACHMENT_STORAGE": "tos", "TOS_ENDPOINT": "https://tos-cn-beijing.volces.com", "TOS_REGION": "cn-beijing",
		"TOS_BUCKET": "cc4-test", "TOS_KEY_PREFIX": "cc4/", "TOS_ACCESS_KEY": "test-ak", "TOS_SECRET_KEY": "test-sk", "TOS_SESSION_TOKEN": "",
	}
	set := func(overrides map[string]string) {
		for key, value := range valid {
			if override, ok := overrides[key]; ok {
				value = override
			}
			t.Setenv(key, value)
		}
	}
	for name, overrides := range map[string]map[string]string{
		"unknown mode":       {"ATTACHMENT_STORAGE": "s3"},
		"plain http":         {"TOS_ENDPOINT": "http://tos-cn-beijing.volces.com"},
		"foreign host":       {"TOS_ENDPOINT": "https://tos-cn-beijing.example.com"},
		"endpoint with path": {"TOS_ENDPOINT": "https://tos-cn-beijing.volces.com/bucket"},
		"missing region":     {"TOS_REGION": ""},
		"missing bucket":     {"TOS_BUCKET": ""},
		"missing secret":     {"TOS_SECRET_KEY": ""},
		"prefix no slash":    {"TOS_KEY_PREFIX": "cc4"},
		"absolute prefix":    {"TOS_KEY_PREFIX": "/cc4/"},
		"traversal prefix":   {"TOS_KEY_PREFIX": "cc4/../x/"},
	} {
		set(overrides)
		if store, err := FromEnv(t.TempDir()); err == nil {
			t.Fatalf("%s: accepted invalid configuration (remote=%v)", name, store.Remote())
		}
	}
	set(nil)
	store, err := FromEnv(t.TempDir())
	if err != nil || !store.Remote() || store.ObjectKey(receipt) != "cc4/"+receipt {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	t.Setenv("ATTACHMENT_STORAGE", "")
	if store, err = FromEnv(t.TempDir()); err != nil || store.Remote() {
		t.Fatalf("default must be local storage, got remote=%v err=%v", store != nil && store.Remote(), err)
	}
}

func TestMigrateRequiresCloudConfiguration(t *testing.T) {
	if _, err := FromLocal(t.TempDir()).Migrate(context.Background(), false); err == nil {
		t.Fatal("migration without TOS must fail")
	}
}
