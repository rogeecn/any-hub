package integration

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/hubmodule/legacy"
	"github.com/any-hub/any-hub/internal/server"
)

func TestLegacyAdapterRolloutToggle(t *testing.T) {
	const moduleKey = "rollout-toggle-test"
	_ = hubmodule.Register(hubmodule.ModuleMetadata{Key: moduleKey})

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	baseHub := config.HubConfig{
		Name:     "dual-mode",
		Domain:   "dual.local",
		Type:     "docker",
		Upstream: "https://registry.npmjs.org",
		Module:   moduleKey,
	}

	testCases := []struct {
		name        string
		rolloutFlag string
		expectKey   string
		expectFlag  legacy.RolloutFlag
	}{
		{
			name:        "force legacy",
			rolloutFlag: "legacy-only",
			expectKey:   hubmodule.DefaultModuleKey(),
			expectFlag:  legacy.RolloutLegacyOnly,
		},
		{
			name:        "dual mode",
			rolloutFlag: "dual",
			expectKey:   moduleKey,
			expectFlag:  legacy.RolloutDual,
		},
		{
			name:        "full modular",
			rolloutFlag: "modular",
			expectKey:   moduleKey,
			expectFlag:  legacy.RolloutModular,
		},
		{
			name:        "rollback to legacy",
			rolloutFlag: "legacy-only",
			expectKey:   hubmodule.DefaultModuleKey(),
			expectFlag:  legacy.RolloutLegacyOnly,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Global: config.GlobalConfig{
					ListenPort: 6100,
					CacheTTL:   config.Duration(time.Minute),
				},
				Hubs: []config.HubConfig{
					func() config.HubConfig {
						h := baseHub
						h.Rollout = tc.rolloutFlag
						return h
					}(),
				},
			}

			registry, err := server.NewHubRegistry(cfg)
			if err != nil {
				t.Fatalf("failed to build registry: %v", err)
			}

			recorder := &routeRecorder{}
			app := mustNewApp(t, cfg.Global.ListenPort, logger, registry, recorder)

			req := httptest.NewRequest("GET", "http://dual.local/v2/", nil)
			req.Host = "dual.local"
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != fiber.StatusNoContent {
				t.Fatalf("unexpected status: %d", resp.StatusCode)
			}

			if recorder.moduleKey != tc.expectKey {
				t.Fatalf("expected module %s, got %s", tc.expectKey, recorder.moduleKey)
			}
			if recorder.rolloutFlag != tc.expectFlag {
				t.Fatalf("expected rollout flag %s, got %s", tc.expectFlag, recorder.rolloutFlag)
			}
		})
	}
}

type routeRecorder struct {
	moduleKey   string
	rolloutFlag legacy.RolloutFlag
}

func (r *routeRecorder) Handle(c fiber.Ctx, route *server.HubRoute) error {
	r.moduleKey = route.ModuleKey
	r.rolloutFlag = route.RolloutFlag
	return c.SendStatus(fiber.StatusNoContent)
}
