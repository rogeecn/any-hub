package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/hubmodule/legacy"
	"github.com/any-hub/any-hub/internal/proxy/hooks"
	"github.com/any-hub/any-hub/internal/server"
	"github.com/any-hub/any-hub/internal/server/routes"
)

func TestModuleDiagnosticsEndpoints(t *testing.T) {
	const moduleKey = "diagnostics-test"
	_ = hubmodule.Register(hubmodule.ModuleMetadata{
		Key:            moduleKey,
		Description:    "diagnostics test module",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"npm",
		},
	})
	hooks.MustRegister(moduleKey, hooks.Hooks{})

	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort: 6200,
			CacheTTL:   config.Duration(30 * time.Minute),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "legacy-hub",
				Domain:   "legacy.local",
				Type:     "docker",
				Upstream: "https://registry-1.docker.io",
				Module:   hubmodule.DefaultModuleKey(),
				Rollout:  string(legacy.RolloutLegacyOnly),
			},
			{
				Name:     "modern-hub",
				Domain:   "modern.local",
				Type:     "npm",
				Upstream: "https://registry.npmjs.org",
				Module:   moduleKey,
				Rollout:  "dual",
			},
		},
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to build registry: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	app := mustNewApp(t, cfg.Global.ListenPort, logger, registry, server.ProxyHandlerFunc(func(c fiber.Ctx, _ *server.HubRoute) error {
		return c.SendStatus(fiber.StatusNoContent)
	}))
	routes.RegisterModuleRoutes(app, registry)

	t.Run("list modules and hubs", func(t *testing.T) {
		resp := doRequest(t, app, "GET", "/-/modules")
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var payload struct {
			Modules []map[string]any `json:"modules"`
			Hubs    []struct {
				HubName    string `json:"hub_name"`
				ModuleKey  string `json:"module_key"`
				Rollout    string `json:"rollout_flag"`
				Domain     string `json:"domain"`
				Port       int    `json:"port"`
				LegacyOnly bool   `json:"legacy_only"`
			} `json:"hubs"`
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode response: %v\nbody: %s", err, string(body))
		}
		if len(payload.Modules) == 0 {
			t.Fatalf("expected module metadata entries")
		}
		found := false
		for _, module := range payload.Modules {
			if module["key"] == moduleKey {
				if module["hook_status"] != "registered" {
					t.Fatalf("expected module %s hook_status registered, got %v", moduleKey, module["hook_status"])
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected module %s in diagnostics payload", moduleKey)
		}
		if len(payload.Hubs) != 2 {
			t.Fatalf("expected 2 hubs, got %d", len(payload.Hubs))
		}
		for _, hub := range payload.Hubs {
			switch hub.HubName {
			case "legacy-hub":
				if hub.ModuleKey != hubmodule.DefaultModuleKey() {
					t.Fatalf("legacy hub should expose legacy module, got %s", hub.ModuleKey)
				}
				if !hub.LegacyOnly {
					t.Fatalf("legacy hub should be marked legacy_only")
				}
			case "modern-hub":
				if hub.ModuleKey != moduleKey {
					t.Fatalf("modern hub should expose %s, got %s", moduleKey, hub.ModuleKey)
				}
				if hub.Rollout != "dual" {
					t.Fatalf("modern hub rollout flag should be dual, got %s", hub.Rollout)
				}
				if hub.LegacyOnly {
					t.Fatalf("modern hub should not be marked legacy_only")
				}
			default:
				t.Fatalf("unexpected hub %s", hub.HubName)
			}
		}
	})

	t.Run("inspect module by key", func(t *testing.T) {
		resp := doRequest(t, app, "GET", "/-/modules/"+moduleKey)
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var module map[string]any
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(body, &module); err != nil {
			t.Fatalf("module inspect decode failed: %v", err)
		}
		if module["key"] != moduleKey {
			t.Fatalf("expected module key %s, got %v", moduleKey, module["key"])
		}
	})

	t.Run("unknown module returns 404", func(t *testing.T) {
		resp := doRequest(t, app, "GET", "/-/modules/missing-module")
		if resp.StatusCode != fiber.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

func doRequest(t *testing.T, app *fiber.App, method, url string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, url, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, url, err)
	}
	return resp
}
