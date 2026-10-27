package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
)

// Real-bucket checks, skipped unless CC4_TOS_SMOKE=1 with ATTACHMENT_STORAGE=tos
// and a throwaway TOS_KEY_PREFIX (it must contain "smoke"; everything under it is deleted).
func smokeStore(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("CC4_TOS_SMOKE") != "1" {
		t.Skip("set CC4_TOS_SMOKE=1 to run against a real TOS bucket")
	}
	if !strings.Contains(os.Getenv("TOS_KEY_PREFIX"), "smoke") {
		t.Fatal("TOS_KEY_PREFIX must be a dedicated smoke-test prefix")
	}
	store, err := FromEnv(t.TempDir())
	if err != nil || !store.Remote() {
		t.Fatalf("TOS configuration: %v", err)
	}
	return store
}

func randomKey(t *testing.T, ext string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return "receipts/" + hex.EncodeToString(b[:]) + ext
}

func TestTOSRoundTrip(t *testing.T) {
	store := smokeStore(t)
	ctx := context.Background()
	key := randomKey(t, ".png")
	body := []byte("\x89PNG\r\n\x1a\ncc4 smoke " + key)
	if err := store.Put(ctx, key, body, "image/png"); err != nil {
		t.Fatalf("put: %v", err)
	}
	defer store.Rollback(key)
	if err := store.Put(ctx, key, []byte("overwrite"), "image/png"); !errors.Is(err, ErrExists) {
		t.Fatalf("overwrite must be refused by TOS, got %v", err)
	}
	got, err := store.Read(ctx, key)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("read back %d bytes, %v", len(got), err)
	}
	acl, err := store.client.GetObjectACL(ctx, &tos.GetObjectACLInput{Bucket: store.bucket, Key: store.ObjectKey(key)})
	if err != nil {
		t.Fatalf("get acl: %v", err)
	}
	for _, grant := range acl.Grants {
		if grant.GranteeV2.Canned == "AllUsers" || grant.GranteeV2.Canned == "AuthenticatedUsers" {
			t.Fatalf("object must stay private, got grant %+v", grant)
		}
	}
	if err = store.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err = store.Read(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted object must be not found, got %v", err)
	}
}

func TestTOSMigrateVerifiesBytesAndKeepsLocalFiles(t *testing.T) {
	store := smokeStore(t)
	ctx := context.Background()
	key := randomKey(t, ".pdf")
	local := filepath.Join(store.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(local), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("%PDF-1.4 cc4 smoke"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.root, "notes.txt"), []byte("not an attachment"), 0600); err != nil {
		t.Fatal(err)
	}
	defer store.Rollback(key)
	planned, err := store.Migrate(ctx, false)
	if err != nil || planned["planned"] != 1 || planned["skipped"] != 1 {
		t.Fatalf("dry run %v, %v", planned, err)
	}
	if _, err = store.remoteRead(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("dry run must not upload, got %v", err)
	}
	counts, err := store.Migrate(ctx, true)
	if err != nil || counts["copied"] != 1 || counts["verified"] != 1 {
		t.Fatalf("apply %v, %v", counts, err)
	}
	if again, err := store.Migrate(ctx, true); err != nil || again["copied"] != 0 || again["verified"] != 1 {
		t.Fatalf("second run must only verify, got %v, %v", again, err)
	}
	if _, err = os.Stat(local); err != nil {
		t.Fatalf("migration must keep the local file: %v", err)
	}
}

// TestTOSCleanPrefix removes every object under the smoke prefix (e.g. claims left by integration tests).
func TestTOSCleanPrefix(t *testing.T) {
	store := smokeStore(t)
	if os.Getenv("CC4_TOS_CLEAN") != "1" {
		t.Skip("set CC4_TOS_CLEAN=1 to empty the smoke prefix")
	}
	ctx := context.Background()
	deleted := 0
	for {
		page, err := store.client.ListObjectsType2(ctx, &tos.ListObjectsType2Input{Bucket: store.bucket, Prefix: store.prefix, MaxKeys: 1000})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(page.Contents) == 0 {
			break
		}
		for _, object := range page.Contents {
			if _, err = store.client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{Bucket: store.bucket, Key: object.Key}); err != nil {
				t.Fatalf("delete %s: %v", object.Key, err)
			}
			deleted++
		}
	}
	t.Logf("deleted %d objects under %s", deleted, store.prefix)
}
