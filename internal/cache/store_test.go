package cache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorePutAndGet(t *testing.T) {
	store := newTestStore(t)
	locator := Locator{HubName: "docker", Path: "/v2/library/sample/manifests/latest"}

	modTime := time.Now().Add(-time.Hour).UTC()
	payload := []byte("payload")
	if _, err := store.Put(context.Background(), locator, bytes.NewReader(payload), PutOptions{ModTime: modTime}); err != nil {
		t.Fatalf("put error: %v", err)
	}

	result, err := store.Get(context.Background(), locator)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	defer result.Reader.Close()

	body, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("read cached body error: %v", err)
	}
	if string(body) != string(payload) {
		t.Fatalf("cached payload mismatch: %s", string(body))
	}
	if result.Entry.SizeBytes != int64(len(payload)) {
		t.Fatalf("size mismatch: %d", result.Entry.SizeBytes)
	}
	if !result.Entry.ModTime.Equal(modTime) {
		t.Fatalf("modtime mismatch: expected %v got %v", modTime, result.Entry.ModTime)
	}
	if !strings.HasSuffix(result.Entry.FilePath, cacheFileSuffix) {
		t.Fatalf("expected cache file suffix %s, got %s", cacheFileSuffix, result.Entry.FilePath)
	}
}

func TestStoreGetMissing(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Get(context.Background(), Locator{HubName: "docker", Path: "/missing"})
	if err == nil || err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStoreRemove(t *testing.T) {
	store := newTestStore(t)
	locator := Locator{HubName: "docker", Path: "/cache/remove"}
	if _, err := store.Put(context.Background(), locator, bytes.NewReader([]byte("data")), PutOptions{}); err != nil {
		t.Fatalf("put error: %v", err)
	}
	if err := store.Remove(context.Background(), locator); err != nil {
		t.Fatalf("remove error: %v", err)
	}
	if _, err := store.Get(context.Background(), locator); err == nil || err != ErrNotFound {
		t.Fatalf("expected not found after remove, got %v", err)
	}
}

func TestStoreIgnoresDirectories(t *testing.T) {
	store := newTestStore(t)
	locator := Locator{HubName: "ghcr", Path: "/v2"}

	fs, ok := store.(*fileStore)
	if !ok {
		t.Fatalf("unexpected store type %T", store)
	}

	filePath, err := fs.path(locator)
	if err != nil {
		t.Fatalf("path error: %v", err)
	}
	if err := os.MkdirAll(filePath+cacheFileSuffix, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}

	if _, err := store.Get(context.Background(), locator); err == nil || err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for directory, got %v", err)
	}
}

func TestStoreMigratesLegacyEntryOnGet(t *testing.T) {
	store := newTestStore(t)
	fs, ok := store.(*fileStore)
	if !ok {
		t.Fatalf("unexpected store type %T", store)
	}
	locator := Locator{HubName: "npm", Path: "/pkg"}
	legacyPath, err := fs.path(locator)
	if err != nil {
		t.Fatalf("path error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write legacy error: %v", err)
	}

	result, err := store.Get(context.Background(), locator)
	if err != nil {
		t.Fatalf("get legacy error: %v", err)
	}
	body, err := io.ReadAll(result.Reader)
	if err != nil {
		t.Fatalf("read legacy error: %v", err)
	}
	result.Reader.Close()
	if string(body) != "legacy" {
		t.Fatalf("unexpected legacy body: %s", string(body))
	}
	if !strings.HasSuffix(result.Entry.FilePath, cacheFileSuffix) {
		t.Fatalf("expected migrated file suffix, got %s", result.Entry.FilePath)
	}
	if _, statErr := os.Stat(legacyPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("expected legacy path removed, got %v", statErr)
	}
}

func TestStoreHandlesAncestorFileConflict(t *testing.T) {
	store := newTestStore(t)
	fs, ok := store.(*fileStore)
	if !ok {
		t.Fatalf("unexpected store type %T", store)
	}
	metaLocator := Locator{HubName: "npm", Path: "/pkg"}
	legacyPath, err := fs.path(metaLocator)
	if err != nil {
		t.Fatalf("path error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write legacy error: %v", err)
	}

	tarLocator := Locator{HubName: "npm", Path: "/pkg/-/pkg-1.0.0.tgz"}
	if _, err := store.Put(context.Background(), tarLocator, bytes.NewReader([]byte("tar")), PutOptions{}); err != nil {
		t.Fatalf("put tar error: %v", err)
	}

	if _, err := os.Stat(legacyPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected legacy metadata renamed, got %v", err)
	}
	if _, err := os.Stat(legacyPath + cacheFileSuffix); err != nil {
		t.Fatalf("expected migrated legacy cache, got %v", err)
	}
	primary, _, err := fs.entryPaths(tarLocator)
	if err != nil {
		t.Fatalf("entry path error: %v", err)
	}
	if _, err := os.Stat(primary); err != nil {
		t.Fatalf("expected tar cache file, got %v", err)
	}
}

// newTestStore returns a Store backed by a temporary directory.
func newTestStore(t *testing.T) Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store
}
