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

func TestIsRegistryK8sHost(t *testing.T) {
	if !isRegistryK8sHost("registry.k8s.io") {
		t.Fatalf("expected registry.k8s.io to match")
	}
	if isRegistryK8sHost("example.com") {
		t.Fatalf("expected non-registry.k8s.io host to be ignored")
	}
}

func TestRegistryK8sManifestFallbackPath(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	path, ok := manifestFallbackPath(ctx, "/v2/coredns/manifests/v1.13.1")
	if !ok || path != "/v2/coredns/coredns/manifests/v1.13.1" {
		t.Fatalf("expected fallback path, got %q ok=%v", path, ok)
	}
}

func TestRegistryK8sManifestFallbackPathRejectsMultiSegmentRepo(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	if _, ok := manifestFallbackPath(ctx, "/v2/coredns/coredns/manifests/v1.13.1"); ok {
		t.Fatalf("expected multi-segment repo to be ignored")
	}
}

func TestRegistryK8sManifestFallbackPathRejectsNonManifest(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	if _, ok := manifestFallbackPath(ctx, "/v2/coredns/blobs/sha256:deadbeef"); ok {
		t.Fatalf("expected non-manifest path to be ignored")
	}
}

func TestRegistryK8sManifestFallbackPathRejectsNonRegistryHost(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "mirror.gcr.io"}
	if _, ok := manifestFallbackPath(ctx, "/v2/coredns/manifests/v1.13.1"); ok {
		t.Fatalf("expected non-registry.k8s.io host to be ignored")
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
