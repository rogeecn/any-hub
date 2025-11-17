package golang

import (
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestCachePolicyForModuleFiles(t *testing.T) {
	policy := cachePolicy(nil, "/example/@v/v1.0.0.zip", hooks.CachePolicy{})
	if !policy.AllowCache || policy.RequireRevalidate {
		t.Fatalf("expected immutable go artifacts to be cacheable without revalidate")
	}

	policy = cachePolicy(nil, "/example/@latest", hooks.CachePolicy{})
	if !policy.RequireRevalidate {
		t.Fatalf("expected non-artifacts to require revalidate")
	}
}
