package server

import (
	"testing"
	"time"

	"github.com/any-hub/any-hub/internal/config"
)

func TestHubRegistryLookupByHost(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: 5000,
			CacheTTL:   config.Duration(2 * time.Hour),
		},
		Hubs: []config.HubConfig{
			{
				Name:            "docker",
				Domain:          "docker.hub.local",
				Type:            "docker",
				Upstream:        "https://registry-1.docker.io",
				EnableHeadCheck: true,
			},
			{
				Name:     "npm",
				Domain:   "npm.hub.local",
				Type:     "npm",
				Upstream: "https://registry.npmjs.org",
				CacheTTL: config.Duration(30 * time.Minute),
			},
		},
	}

	registry, err := NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	route, ok := registry.Lookup("docker.hub.local")
	if !ok {
		t.Fatalf("expected docker route")
	}

	if route.Config.Name != "docker" {
		t.Errorf("wrong hub returned: %s", route.Config.Name)
	}

	if route.CacheTTL != cfg.EffectiveCacheTTL(route.Config) {
		t.Errorf("cache ttl mismatch: got %s", route.CacheTTL)
	}
	if route.CacheStrategy.TTLHint != 0 {
		t.Errorf("cache strategy ttl mismatch: %s vs %s", route.CacheStrategy.TTLHint, time.Duration(0))
	}
	if route.CacheStrategy.ValidationMode == "" {
		t.Fatalf("cache strategy validation mode should not be empty")
	}
	if route.ModuleKey != "docker" {
		t.Fatalf("expected docker module, got %s", route.ModuleKey)
	}

	if route.UpstreamURL.String() != "https://registry-1.docker.io" {
		t.Errorf("unexpected upstream URL: %s", route.UpstreamURL)
	}

	if route.ProxyURL != nil {
		t.Errorf("expected nil proxy")
	}

	if route.ListenPort != cfg.Global.ListenPort {
		t.Fatalf("route listen port mismatch: %d", route.ListenPort)
	}

	if got := len(registry.List()); got != 2 {
		t.Fatalf("expected 2 routes in list, got %d", got)
	}
}

func TestHubRegistryParsesHostHeaderPort(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: 5000,
			CacheTTL:   config.Duration(time.Hour),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Upstream: "https://registry-1.docker.io",
			},
		},
	}

	registry, err := NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := registry.Lookup("docker.hub.local:6000"); !ok {
		t.Fatalf("expected lookup to ignore host header port")
	}
}

func TestHubRegistryRejectsDuplicateDomains(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: 5000,
			CacheTTL:   config.Duration(time.Hour),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Upstream: "https://registry-1.docker.io",
			},
			{
				Name:     "docker-alt",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Upstream: "https://mirror.registry-1.docker.io",
			},
		},
	}

	if _, err := NewHubRegistry(cfg); err == nil {
		t.Fatalf("expected duplicate domain error")
	}
}
