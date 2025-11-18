package apk

import (
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestCachePolicyIndexAndSignatureRevalidate(t *testing.T) {
	paths := []string{
		"/v3.19/main/x86_64/APKINDEX.tar.gz",
		"/v3.19/main/x86_64/APKINDEX.tar.gz.asc",
		"/v3.19/community/aarch64/apkindex.tar.gz.sig",
	}
	for _, p := range paths {
		current := cachePolicy(nil, p, hooks.CachePolicy{})
		if !current.AllowCache || !current.AllowStore || !current.RequireRevalidate {
			t.Fatalf("expected index/signature to require revalidate for %s", p)
		}
	}
}

func TestCachePolicyPackageImmutable(t *testing.T) {
	tests := []string{
		"/v3.19/main/x86_64/packages/hello-1.0.apk",
		"/v3.18/testing/aarch64/packages/../packages/hello-1.0-r1.APK",
		"/v3.22/community/x86_64/tini-static-0.19.0-r3.apk", // 路径不含 /packages/ 也应视作包体
	}
	for _, p := range tests {
		current := cachePolicy(nil, p, hooks.CachePolicy{})
		if !current.AllowCache || !current.AllowStore || current.RequireRevalidate {
			t.Fatalf("expected immutable cache for %s", p)
		}
	}
}

func TestCachePolicyNonAPKPath(t *testing.T) {
	current := cachePolicy(nil, "/other/path", hooks.CachePolicy{})
	if current.AllowCache || current.AllowStore || current.RequireRevalidate {
		t.Fatalf("expected non-APK path to disable cache/store")
	}
}

func TestNormalizePath(t *testing.T) {
	p, _ := normalizePath(nil, "v3.19/main/x86_64/APKINDEX.tar.gz", nil)
	if p != "/v3.19/main/x86_64/APKINDEX.tar.gz" {
		t.Fatalf("unexpected normalized path: %s", p)
	}
}

func TestContentType(t *testing.T) {
	if ct := contentType(nil, "/v3.19/main/x86_64/APKINDEX.tar.gz"); ct != "application/gzip" {
		t.Fatalf("expected gzip content type, got %s", ct)
	}
	if ct := contentType(nil, "/v3.19/main/x86_64/APKINDEX.tar.gz.asc"); ct != "application/pgp-signature" {
		t.Fatalf("expected signature content type, got %s", ct)
	}
	if ct := contentType(nil, "/v3.19/main/x86_64/packages/hello.apk"); ct != "application/vnd.android.package-archive" {
		t.Fatalf("expected apk content type, got %s", ct)
	}
}
