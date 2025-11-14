package integration

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/any-hub/any-hub/internal/cache"
)

func TestCacheWriteCleanupOnInterruptedStream(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := cache.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("store init error: %v", err)
	}

	loc := cache.Locator{HubName: "docker", Path: "/interrupt/blob.tar"}

	reader := &flakyReader{
		payload:   []byte("partial_data"),
		failAfter: 5,
	}

	if _, err := store.Put(context.Background(), loc, reader, cache.PutOptions{}); err == nil {
		t.Fatalf("expected error from interrupted reader")
	}

	target := filepath.Join(tmpDir, "docker", "interrupt", "blob.tar")
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no final file, got err=%v", err)
	}
	pattern := filepath.Join(tmpDir, "docker", "interrupt", ".cache-*")
	matches, _ := filepath.Glob(pattern)
	if len(matches) != 0 {
		t.Fatalf("temporary files should be cleaned up, found %v", matches)
	}
}

type flakyReader struct {
	payload   []byte
	failAfter int
	readBytes int
}

func (f *flakyReader) Read(p []byte) (int, error) {
	if f.readBytes >= f.failAfter {
		return 0, io.ErrUnexpectedEOF
	}
	remaining := f.failAfter - f.readBytes
	if remaining > len(p) {
		remaining = len(p)
	}
	copy(p[:remaining], f.payload[f.readBytes:f.readBytes+remaining])
	f.readBytes += remaining
	return remaining, nil
}
