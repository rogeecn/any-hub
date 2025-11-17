package composer

import (
	"encoding/json"
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

func TestResolveMirrorDistUpstream(t *testing.T) {
	resetComposerDistRegistry()
	composerDists.remember("cache.example", "vendor/pkg", "abc123", "zip", "https://github.com/org/repo.zip")
	ctx := &hooks.RequestContext{Domain: "cache.example"}
	url := resolveDistUpstream(ctx, "", "/dists/vendor/pkg/abc123.zip", nil)
	if url != "https://github.com/org/repo.zip" {
		t.Fatalf("unexpected upstream %s", url)
	}
}

func TestRewriteResponseUpdatesURLs(t *testing.T) {
	resetComposerDistRegistry()
	ctx := &hooks.RequestContext{Domain: "cache.example"}
	body := []byte(`{"packages":{"a/b":{"1.0.0":{"dist":{"url":"https://api.github.com/repos/org/repo/zipball/ref","reference":"abc123","type":"zip"}}}}}`)
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
	var payload map[string]any
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pkgs := payload["packages"].(map[string]any)
	versions := pkgs["a/b"].(map[string]any)
	version := versions["1.0.0"].(map[string]any)
	dist := version["dist"].(map[string]any)
	distURL := dist["url"].(string)
	expected := "https://cache.example/dist/https/api.github.com/repos/org/repo/zipball/ref"
	if distURL != expected {
		t.Fatalf("expected dist url %s, got %s", expected, distURL)
	}
}

func TestRewritePackagesRoot(t *testing.T) {
	resetComposerDistRegistry()
	ctx := &hooks.RequestContext{Domain: "cache.example"}
	body := []byte(`{"metadata-url":"https://repo.packagist.org/p2/%package%.json","providers-url":"/p/%package%$%hash%.json"}`)
	_, headers, rewritten, err := rewriteResponse(ctx, 200, map[string]string{}, body, "/packages.json")
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatalf("expected json content type")
	}
	var payload map[string]any
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["metadata-url"] != "https://cache.example/p2/%package%.json" {
		t.Fatalf("metadata URL not rewritten: %v", payload["metadata-url"])
	}
	if payload["providers-url"] != "https://cache.example/p/%package%$%hash%.json" {
		t.Fatalf("providers URL not rewritten: %v", payload["providers-url"])
	}
	mirrors, _ := payload["mirrors"].([]any)
	if len(mirrors) == 0 {
		t.Fatalf("mirrors missing")
	}
	entry, _ := mirrors[0].(map[string]any)
	if entry["dist-url"] != "https://cache.example/dists/%package%/%reference%.%type%" {
		t.Fatalf("unexpected mirror dist-url: %v", entry["dist-url"])
	}
	if pref, _ := entry["preferred"].(bool); !pref {
		t.Fatalf("mirror preferred flag missing")
	}
}
