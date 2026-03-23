package docker

import (
	"testing"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestNormalizePathAddsLibraryForDockerHub(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry-1.docker.io"}
	path, _ := normalizePath(ctx, "/v2/nginx/manifests/latest", nil)
	if path != "/v2/library/nginx/manifests/latest" {
		t.Fatalf("expected library namespace, got %s", path)
	}

	path, _ = normalizePath(ctx, "/v2/library/nginx/manifests/latest", nil)
	if path != "/v2/library/nginx/manifests/latest" {
		t.Fatalf("unexpected rewrite for existing namespace")
	}
}

func TestNormalizePathSkipsLibraryForNonDockerHub(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	path, _ := normalizePath(ctx, "/v2/kube-apiserver/manifests/v1.35.3", nil)
	if path != "/v2/kube-apiserver/manifests/v1.35.3" {
		t.Fatalf("expected non-docker hub path to remain unchanged, got %s", path)
	}
}

func TestSplitDockerRepoPath(t *testing.T) {
	repo, rest, ok := splitDockerRepoPath("/v2/library/nginx/manifests/latest")
	if !ok || repo != "library/nginx" || rest != "/manifests/latest" {
		t.Fatalf("unexpected split result repo=%s rest=%s ok=%v", repo, rest, ok)
	}

	if _, _, ok := splitDockerRepoPath("/v2/_catalog"); ok {
		t.Fatalf("expected catalog path to be ignored")
	}
}
