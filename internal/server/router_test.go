package server

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/config"
)

func TestRouterRoutesRequestWhenHostMatches(t *testing.T) {
	app := newTestApp(t, 5000)

	req := httptest.NewRequest("GET", "http://docker.hub.local/v2/", nil)
	req.Host = "docker.hub.local"
	req.Header.Set("Host", "docker.hub.local")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 204 status, got %d (body=%s, hostHeader=%s)", resp.StatusCode, string(body), resp.Header.Get("X-Any-Hub-Host"))
	}

	if app.storage.routeName != "docker" {
		t.Fatalf("expected docker route, got %s", app.storage.routeName)
	}

	if reqID := resp.Header.Get("X-Request-ID"); reqID == "" {
		t.Fatalf("expected X-Request-ID header to be set")
	}
}

func TestRouterReturns404WhenHostUnknown(t *testing.T) {
	app := newTestApp(t, 5000)

	req := httptest.NewRequest("GET", "http://unknown.local/v2/", nil)
	req.Host = "unknown.local"
	req.Header.Set("Host", "unknown.local")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 status, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte(`"host_unmapped"`)) {
		t.Fatalf("expected host_unmapped error, got %s", string(body))
	}
}

type testApp struct {
	*fiber.App
	storage *proxyRecorder
}

func newTestApp(t *testing.T, port int) *testApp {
	t.Helper()

	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: port,
			CacheTTL:   config.Duration(3600),
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
		t.Fatalf("failed to create registry: %v", err)
	}
	if _, ok := registry.Lookup("docker.hub.local"); !ok {
		t.Fatalf("registry lookup failed for docker")
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	recorder := &proxyRecorder{}
	app, err := NewApp(AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      recorder,
		ListenPort: port,
	})
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	return &testApp{App: app, storage: recorder}
}

type proxyRecorder struct {
	lastRoute *HubRoute
	routeName string
}

func (p *proxyRecorder) Handle(c fiber.Ctx, route *HubRoute) error {
	p.lastRoute = route
	p.routeName = route.Config.Name
	return c.SendStatus(fiber.StatusNoContent)
}
