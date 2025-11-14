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

func TestHostRoutingDistinguishesDomainsOnSinglePort(t *testing.T) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: 5000,
			CacheTTL:   config.Duration(3600),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Upstream: "https://registry-1.docker.io",
			},
			{
				Name:     "npm",
				Domain:   "npm.hub.local",
				Type:     "npm",
				Upstream: "https://registry.npmjs.org",
			},
		},
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	app := newIntegrationApp(t, cfg.Global.ListenPort, logger, registry, &proxyRecorder{})
	recorder := app.recorder

	req := httptest.NewRequest("GET", "http://docker.hub.local/v2/", nil)
	req.Host = "docker.hub.local"
	req.Header.Set("Host", "docker.hub.local")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 204 from docker hub, got %d (body=%s)", resp.StatusCode, string(body))
	}
	if recorder.routeName != "docker" {
		t.Fatalf("expected docker route, got %s", recorder.routeName)
	}

	req2 := httptest.NewRequest("GET", "http://npm.hub.local/v2/", nil)
	req2.Host = "npm.hub.local"
	req2.Header.Set("Host", "npm.hub.local")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("app test failed: %v", err)
	}
	if resp2.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 204 from npm hub, got %d (body=%s)", resp2.StatusCode, string(body))
	}
	if recorder.routeName != "npm" {
		t.Fatalf("expected npm route, got %s", recorder.routeName)
	}

	req3 := httptest.NewRequest("GET", "http://unknown.hub.local/v2/", nil)
	req3.Host = "unknown.hub.local"
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("app test failed: %v", err)
	}
	if resp3.StatusCode != fiber.StatusNotFound {
		body, _ := io.ReadAll(resp3.Body)
		t.Fatalf("expected 404 for unmapped host, got %d (body=%s)", resp3.StatusCode, string(body))
	}
}

type integrationApp struct {
	*fiber.App
	recorder *proxyRecorder
}

func newIntegrationApp(t *testing.T, port int, logger *logrus.Logger, registry *server.HubRegistry, proxy server.ProxyHandler) *integrationApp {
	t.Helper()
	recorder, ok := proxy.(*proxyRecorder)
	if !ok {
		recorder = &proxyRecorder{}
	}
	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		ListenPort: port,
		Proxy:      recorder,
	})
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	return &integrationApp{App: app, recorder: recorder}
}

type proxyRecorder struct {
	routeName string
}

func (p *proxyRecorder) Handle(c fiber.Ctx, route *server.HubRoute) error {
	p.routeName = route.Config.Name
	return c.SendStatus(fiber.StatusNoContent)
}
