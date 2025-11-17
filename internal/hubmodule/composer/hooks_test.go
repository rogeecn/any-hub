package composer

import (
	"strings"
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestNormalizePathDropsDistQuery(t *testing.T) {
	path, raw := normalizePath(nil, "/dist/https/example.com/file.zip", []byte("token=1"))
	if raw != nil {
		t.Fatalf("expected query to be dropped")
	}
	if path != "/dist/https/example.com/file.zip" {
		t.Fatalf("unexpected path %s", path)
	}
}

func TestResolveDistUpstream(t *testing.T) {
	url := resolveDistUpstream(nil, "", "/dist/https/example.com/file.zip", []byte("token=1"))
	if url != "https://example.com/file.zip?token=1" {
		t.Fatalf("unexpected upstream %s", url)
	}
}

func TestRewriteResponseUpdatesURLs(t *testing.T) {
	ctx := &hooks.RequestContext{Domain: "cache.example"}
	body := []byte(`{"packages":{"a/b":{"1.0.0":{"dist":{"url":"https://pkg.example/dist.zip"}}}}}`)
	_, headers, rewritten, err := rewriteResponse(ctx, 200, map[string]string{}, body, "/p2/a/b.json")
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if string(rewritten) == string(body) {
		t.Fatalf("expected rewrite to modify payload")
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("expected json content type")
	}
	if !strings.Contains(string(rewritten), "https://cache.example/dist/https/pkg.example/dist.zip") {
		t.Fatalf("expected rewritten URL, got %s", string(rewritten))
	}
}
