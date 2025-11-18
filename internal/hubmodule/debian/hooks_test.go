package debian

import (
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestCachePolicyIndexesRevalidate(t *testing.T) {
	current := cachePolicy(nil, "/dists/bookworm/Release", hooks.CachePolicy{})
	if !current.AllowCache || !current.AllowStore || !current.RequireRevalidate {
		t.Fatalf("expected index to allow cache/store and revalidate")
	}
	current = cachePolicy(nil, "/dists/bookworm/main/binary-amd64/Packages.gz", hooks.CachePolicy{})
	if !current.AllowCache || !current.AllowStore || !current.RequireRevalidate {
		t.Fatalf("expected packages index to revalidate")
	}
	current = cachePolicy(nil, "/dists/bookworm/main/Contents-amd64.gz", hooks.CachePolicy{})
	if !current.AllowCache || !current.AllowStore || !current.RequireRevalidate {
		t.Fatalf("expected contents index to revalidate")
	}
	current = cachePolicy(nil, "/debian-security/dists/trixie/Contents-amd64.gz", hooks.CachePolicy{})
	if !current.AllowCache || !current.AllowStore || !current.RequireRevalidate {
		t.Fatalf("expected prefixed contents index to revalidate")
	}
}

func TestCachePolicyImmutable(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "by-hash index snapshot", path: "/dists/bookworm/by-hash/sha256/abc"},
		{name: "by-hash nested", path: "/dists/bookworm/main/binary-amd64/by-hash/SHA256/def"},
		{name: "pool package", path: "/pool/main/h/hello.deb"},
		{name: "pool canonicalized", path: " /PoOl/main/../main/h/hello_1.0_amd64.DeB "},
		{name: "mirror prefix pool", path: "/debian/pool/main/h/hello.deb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := cachePolicy(nil, tt.path, hooks.CachePolicy{})
			if !current.AllowCache || !current.AllowStore || current.RequireRevalidate {
				t.Fatalf("expected immutable cache for %s", tt.path)
			}
		})
	}
}

func TestCachePolicyNonAptPath(t *testing.T) {
	current := cachePolicy(nil, "/other/path", hooks.CachePolicy{})
	if current.AllowCache || current.AllowStore || current.RequireRevalidate {
		t.Fatalf("expected non-APT path to disable cache/store")
	}
}

func TestNormalizePath(t *testing.T) {
	path, _ := normalizePath(nil, "dists/bookworm/Release", nil)
	if path != "/dists/bookworm/Release" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestContentType(t *testing.T) {
	if ct := contentType(nil, "/dists/bookworm/Release"); ct != "text/plain" {
		t.Fatalf("expected text/plain for Release, got %s", ct)
	}
	if ct := contentType(nil, "/dists/bookworm/Release.gpg"); ct != "application/pgp-signature" {
		t.Fatalf("expected signature content-type, got %s", ct)
	}
	if ct := contentType(nil, "/dists/bookworm/main/binary-amd64/Packages.gz"); ct != "application/gzip" {
		t.Fatalf("expected gzip content-type, got %s", ct)
	}
}
