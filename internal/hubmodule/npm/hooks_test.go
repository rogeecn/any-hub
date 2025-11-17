package npm

import (
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestCachePolicyForTarball(t *testing.T) {
	policy := cachePolicy(nil, "/pkg/-/pkg-1.0.0.tgz", hooks.CachePolicy{})
	if policy.RequireRevalidate {
		t.Fatalf("tarball should not require revalidate")
	}
	if !policy.AllowCache {
		t.Fatalf("tarball should allow cache")
	}

	policy = cachePolicy(nil, "/pkg", hooks.CachePolicy{})
	if !policy.RequireRevalidate {
		t.Fatalf("metadata should require revalidate")
	}
}
