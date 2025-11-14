package integration

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/proxy"
	"github.com/any-hub/any-hub/internal/server"
)

func TestDockerSampleConfigWithStub(t *testing.T) {
	stub := newCacheFlowStub(t, dockerManifestPath)
	defer stub.Close()

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "storage")

	data, err := os.ReadFile(repoPath(t, "configs", "docker.sample.toml"))
	if err != nil {
		t.Fatalf("read sample config: %v", err)
	}

	content := strings.ReplaceAll(string(data), "./storage/docker", storageDir)
	content = strings.Replace(content, "https://registry-1.docker.io", stub.URL, 1)

	tempConfig := filepath.Join(tempDir, "docker.sample.toml")
	if err := os.WriteFile(tempConfig, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := config.Load(tempConfig)
	if err != nil {
		t.Fatalf("config load error: %v", err)
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	store, err := cache.NewStore(cfg.Global.StoragePath)
	if err != nil {
		t.Fatalf("store error: %v", err)
	}

	client := server.NewUpstreamClient(cfg)
	handler := proxy.NewHandler(client, logger, store)

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: cfg.Global.ListenPort,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	req := httptest.NewRequest("GET", "http://docker.hub.local"+dockerManifestPath, nil)
	req.Host = "docker.hub.local"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if hit := resp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected miss header, got %s", hit)
	}
	resp.Body.Close()

	// second request should hit cache
	req2 := httptest.NewRequest("GET", "http://docker.hub.local"+dockerManifestPath, nil)
	req2.Host = "docker.hub.local"
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("app test error: %v", err)
	}
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cache hit on second request")
	}
	resp2.Body.Close()
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(append([]string{dir}, elems...)...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("unable to locate repository root from %s", dir)
	return ""
}
