package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/proxy"
	"github.com/any-hub/any-hub/internal/server"
)

func TestCacheStrategyOverrides(t *testing.T) {
	t.Run("ttl defers revalidation until expired", func(t *testing.T) {
		stub := newUpstreamStub(t, upstreamNPM)
		defer stub.Close()

		storageDir := t.TempDir()
		ttl := 50 * time.Millisecond
		cfg := &config.Config{
			Global: config.GlobalConfig{
				ListenPort:  6100,
				CacheTTL:    config.Duration(time.Second),
				StoragePath: storageDir,
			},
			Hubs: []config.HubConfig{
				{
					Name:     "npm-ttl",
					Domain:   "ttl.npm.local",
					Type:     "npm",
					Module:   "npm",
					Upstream: stub.URL,
					CacheTTL: config.Duration(ttl),
				},
			},
		}

		app := newStrategyTestApp(t, cfg)

		doRequest := func() *http.Response {
			req := httptest.NewRequest(http.MethodGet, "http://ttl.npm.local/lodash", nil)
			req.Host = "ttl.npm.local"
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test error: %v", err)
			}
			return resp
		}

		resp := doRequest()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if hit := resp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
			t.Fatalf("first request should be miss, got %s", hit)
		}
		resp.Body.Close()

		resp2 := doRequest()
		if hit := resp2.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
			t.Fatalf("second request should hit cache before TTL, got %s", hit)
		}
		resp2.Body.Close()

		if headCount := countRequests(stub.Requests(), http.MethodHead, "/lodash"); headCount != 0 {
			t.Fatalf("expected no HEAD before TTL expiry, got %d", headCount)
		}
		if getCount := countRequests(stub.Requests(), http.MethodGet, "/lodash"); getCount != 1 {
			t.Fatalf("upstream should be hit once before TTL expiry, got %d", getCount)
		}

		time.Sleep(ttl * 2)

		resp3 := doRequest()
		if hit := resp3.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
			body, _ := io.ReadAll(resp3.Body)
			resp3.Body.Close()
			t.Fatalf("expected cached response after HEAD revalidation, got %s body=%s", hit, string(body))
		}
		resp3.Body.Close()

		if headCount := countRequests(stub.Requests(), http.MethodHead, "/lodash"); headCount != 1 {
			t.Fatalf("expected single HEAD after TTL expiry, got %d", headCount)
		}
		if getCount := countRequests(stub.Requests(), http.MethodGet, "/lodash"); getCount != 1 {
			t.Fatalf("upstream GET count should remain 1, got %d", getCount)
		}
	})

	t.Run("validation disabled falls back to refetch", func(t *testing.T) {
		stub := newUpstreamStub(t, upstreamNPM)
		defer stub.Close()

		storageDir := t.TempDir()
		ttl := 25 * time.Millisecond
		cfg := &config.Config{
			Global: config.GlobalConfig{
				ListenPort:  6200,
				CacheTTL:    config.Duration(time.Second),
				StoragePath: storageDir,
			},
			Hubs: []config.HubConfig{
				{
					Name:           "npm-novalidation",
					Domain:         "novalidation.npm.local",
					Type:           "npm",
					Module:         "npm",
					Upstream:       stub.URL,
					CacheTTL:       config.Duration(ttl),
					ValidationMode: string(hubmodule.ValidationModeNever),
				},
			},
		}

		app := newStrategyTestApp(t, cfg)

		doRequest := func() *http.Response {
			req := httptest.NewRequest(http.MethodGet, "http://novalidation.npm.local/lodash", nil)
			req.Host = "novalidation.npm.local"
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test error: %v", err)
			}
			return resp
		}

		first := doRequest()
		if first.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
			t.Fatalf("expected miss on first request")
		}
		first.Body.Close()

		time.Sleep(ttl * 2)

		second := doRequest()
		if second.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
			body, _ := io.ReadAll(second.Body)
			second.Body.Close()
			t.Fatalf("expected cache miss when validation disabled, got hit body=%s", string(body))
		}
		second.Body.Close()

		if headCount := countRequests(stub.Requests(), http.MethodHead, "/lodash"); headCount != 0 {
			t.Fatalf("validation mode never should avoid HEAD, got %d", headCount)
		}
		if getCount := countRequests(stub.Requests(), http.MethodGet, "/lodash"); getCount != 2 {
			t.Fatalf("expected two upstream GETs due to forced refetch, got %d", getCount)
		}
	})
}

func newStrategyTestApp(t *testing.T, cfg *config.Config) *fiber.App {
	t.Helper()

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
	return app
}

func countRequests(reqs []RecordedRequest, method, path string) int {
	count := 0
	for _, req := range reqs {
		if req.Method == method && req.Path == path {
			count++
		}
	}
	return count
}
