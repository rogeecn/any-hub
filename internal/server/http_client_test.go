package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/any-hub/any-hub/internal/config"
)

func TestNewUpstreamClientUsesConfigTimeout(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			UpstreamTimeout: config.Duration(45 * time.Second),
		},
	}

	client := NewUpstreamClient(cfg)
	if client.Timeout != 45*time.Second {
		t.Fatalf("expected timeout 45s, got %s", client.Timeout)
	}
}

func TestCopyHeadersSkipsHopByHop(t *testing.T) {
	src := http.Header{}
	src.Add("Connection", "keep-alive")
	src.Add("Keep-Alive", "timeout=5")
	src.Add("X-Test-Header", "1")
	src.Add("x-test-header", "2")

	dst := http.Header{}
	CopyHeaders(dst, src)

	if _, exists := dst["Connection"]; exists {
		t.Fatalf("connection header should not be copied")
	}
	if _, exists := dst["Keep-Alive"]; exists {
		t.Fatalf("keep-alive header should not be copied")
	}

	got := dst.Values("X-Test-Header")
	if len(got) != 2 {
		t.Fatalf("expected 2 values, got %v", got)
	}
}
