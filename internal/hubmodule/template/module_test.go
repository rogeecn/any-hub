package template

import (
	"net/http"
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

// This test shows a full hook lifecycle that module authors can copy when creating a new hook.
func TestTemplateHookFlow(t *testing.T) {
	baseURL := "https://example.com"
	ctx := &hooks.RequestContext{
		HubName:   "demo",
		ModuleKey: "template",
	}

	h := hooks.Hooks{
		NormalizePath: func(_ *hooks.RequestContext, clean string, rawQuery []byte) (string, []byte) {
			return "/normalized" + clean, rawQuery
		},
		ResolveUpstream: func(_ *hooks.RequestContext, upstream string, clean string, rawQuery []byte) string {
			if len(rawQuery) > 0 {
				return upstream + clean + "?" + string(rawQuery)
			}
			return upstream + clean
		},
		CachePolicy: func(_ *hooks.RequestContext, path string, current hooks.CachePolicy) hooks.CachePolicy {
			current.AllowCache = path != ""
			current.AllowStore = true
			return current
		},
		ContentType: func(_ *hooks.RequestContext, path string) string {
			if path == "/normalized/index.json" {
				return "application/json"
			}
			return ""
		},
		RewriteResponse: func(_ *hooks.RequestContext, status int, headers map[string]string, body []byte, _ string) (int, map[string]string, []byte, error) {
			if headers == nil {
				headers = map[string]string{}
			}
			headers["X-Demo"] = "ok"
			return status, headers, body, nil
		},
	}

	normalized, _ := h.NormalizePath(ctx, "/index.json", nil)
	if normalized != "/normalized/index.json" {
		t.Fatalf("expected normalized path, got %s", normalized)
	}
	u := h.ResolveUpstream(ctx, baseURL, normalized, nil)
	if u != baseURL+normalized {
		t.Fatalf("expected upstream %s, got %s", baseURL+normalized, u)
	}
	policy := h.CachePolicy(ctx, normalized, hooks.CachePolicy{})
	if !policy.AllowCache || !policy.AllowStore {
		t.Fatalf("expected policy to allow cache/store, got %#v", policy)
	}
	status, headers, body, err := h.RewriteResponse(ctx, http.StatusOK, map[string]string{}, []byte("ok"), normalized)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if headers["X-Demo"] != "ok" {
		t.Fatalf("expected rewrite to set header, got %s", headers["X-Demo"])
	}
	if status != http.StatusOK || string(body) != "ok" {
		t.Fatalf("expected unchanged status/body, got %d/%s", status, string(body))
	}
	if ct := h.ContentType(ctx, normalized); ct != "application/json" {
		t.Fatalf("expected content type application/json, got %s", ct)
	}
}
