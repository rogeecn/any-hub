package integration

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/server"
)

func TestModuleRoutingIsolation(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: 6000,
			CacheTTL:   config.Duration(3600),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker-hub",
				Domain:   "legacy.hub.local",
				Type:     "docker",
				Upstream: "https://registry-1.docker.io",
			},
			{
				Name:     "npm-hub",
				Domain:   "test.hub.local",
				Type:     "npm",
				Upstream: "https://registry.example.com",
			},
		},
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	recorder := &moduleRecorder{}
	app := mustNewApp(t, cfg.Global.ListenPort, logger, registry, recorder)

	legacyReq := httptest.NewRequest("GET", "http://legacy.hub.local/v2/", nil)
	legacyReq.Host = "legacy.hub.local"
	legacyReq.Header.Set("Host", "legacy.hub.local")
	resp, err := app.Test(legacyReq)
	if err != nil {
		t.Fatalf("legacy request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("legacy hub should return 204, got %d", resp.StatusCode)
	}
	if recorder.moduleKey != "docker" {
		t.Fatalf("expected docker module, got %s", recorder.moduleKey)
	}

	testReq := httptest.NewRequest("GET", "http://test.hub.local/v2/", nil)
	testReq.Host = "test.hub.local"
	testReq.Header.Set("Host", "test.hub.local")
	resp2, err := app.Test(testReq)
	if err != nil {
		t.Fatalf("test request failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusNoContent {
		t.Fatalf("test hub should return 204, got %d", resp2.StatusCode)
	}
	if recorder.moduleKey != "npm" {
		t.Fatalf("expected npm module, got %s", recorder.moduleKey)
	}
}

func mustNewApp(t *testing.T, port int, logger *logrus.Logger, registry *server.HubRegistry, handler server.ProxyHandler) *fiber.App {
	t.Helper()
	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: port,
	})
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	return app
}

type moduleRecorder struct {
	routeName string
	moduleKey string
}

func (p *moduleRecorder) Handle(c fiber.Ctx, route *server.HubRoute) error {
	p.routeName = route.Config.Name
	p.moduleKey = route.ModuleKey
	return c.SendStatus(fiber.StatusNoContent)
}
