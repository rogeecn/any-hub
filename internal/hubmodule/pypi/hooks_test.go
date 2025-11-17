package pypi

import (
	"strings"
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestNormalizePathAddsSimplePrefix(t *testing.T) {
	ctx := &hooks.RequestContext{HubType: "pypi"}
	path, _ := normalizePath(ctx, "/requests", nil)
	if path != "/simple/requests/" {
		t.Fatalf("expected /simple prefix, got %s", path)
	}
}

func TestResolveFilesUpstream(t *testing.T) {
	ctx := &hooks.RequestContext{}
	target := resolveFilesUpstream(ctx, "", "/files/https/example.com/pkg.tgz", nil)
	if target != "https://example.com/pkg.tgz" {
		t.Fatalf("unexpected upstream target: %s", target)
	}
}

func TestRewriteResponseAdjustsLinks(t *testing.T) {
	ctx := &hooks.RequestContext{Domain: "cache.example"}
	body := []byte(`<html><body><a href="https://files.pythonhosted.org/package.whl">link</a></body></html>`)
	_, headers, rewritten, err := rewriteResponse(ctx, 200, map[string]string{"Content-Type": "text/html"}, body, "/simple/requests/")
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if string(rewritten) == string(body) {
		t.Fatalf("expected rewrite to modify HTML")
	}
	if headers["Content-Type"] == "" {
		t.Fatalf("expected content type to be set")
	}
	if !strings.Contains(string(rewritten), "/files/https/files.pythonhosted.org/package.whl") {
		t.Fatalf("expected rewritten link, got %s", string(rewritten))
	}
}
