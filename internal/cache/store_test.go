package cache

import (
	"bytes"
	"context"
	"io"
	"os"
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

	filePath, err := fs.entryPath(locator)
	if err != nil {
		t.Fatalf("path error: %v", err)
	}
	if err := os.MkdirAll(filePath, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}

	if _, err := store.Get(context.Background(), locator); err == nil || err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for directory, got %v", err)
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
