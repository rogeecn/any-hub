package proxy

import (
	"net/url"
	"testing"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/server"
)

func TestApplyDockerHubNamespaceFallback(t *testing.T) {
	route := dockerHubRoute(t, "https://registry-1.docker.io")

	path, changed := applyDockerHubNamespaceFallback(route, "/v2/nginx/manifests/latest")
	if !changed {
		t.Fatalf("expected fallback to apply")
	}
	if path != "/v2/library/nginx/manifests/latest" {
		t.Fatalf("unexpected normalized path: %s", path)
	}

	path, changed = applyDockerHubNamespaceFallback(route, "/v2/library/nginx/manifests/latest")
	if changed {
		t.Fatalf("expected no changes for already-namespaced repo")
	}

	path, changed = applyDockerHubNamespaceFallback(route, "/v2/rogee/nginx/manifests/latest")
	if changed {
		t.Fatalf("expected no changes for custom namespace")
	}

	path, changed = applyDockerHubNamespaceFallback(route, "/v2/_catalog")
	if changed {
		t.Fatalf("expected no changes for _catalog endpoint")
	}

	otherRoute := dockerHubRoute(t, "https://registry.example.com")
	path, changed = applyDockerHubNamespaceFallback(otherRoute, "/v2/nginx/manifests/latest")
	if changed || path != "/v2/nginx/manifests/latest" {
		t.Fatalf("expected no changes for non-docker-hub upstream")
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

func dockerHubRoute(t *testing.T, upstream string) *server.HubRoute {
	t.Helper()
	parsed, err := url.Parse(upstream)
	if err != nil {
		t.Fatalf("invalid upstream: %v", err)
	}
	return &server.HubRoute{
		Config: config.HubConfig{
			Name: "docker",
			Type: "docker",
		},
		UpstreamURL: parsed,
	}
}
