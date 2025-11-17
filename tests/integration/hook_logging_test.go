package integration

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/proxy"
	"github.com/any-hub/any-hub/internal/server"
)

func TestDockerHookEmitsLogFields(t *testing.T) {
	stub := newCacheFlowStub(t, dockerManifestPath)
	defer stub.Close()

	env := newHookTestEnv(t, config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5300,
			CacheTTL:    config.Duration(time.Minute),
			StoragePath: t.TempDir(),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hook.local",
				Type:     "docker",
				Upstream: stub.URL,
			},
		},
	})
	defer env.Close()

	assertCacheMissThenHit(t, env, "docker.hook.local", dockerManifestPath)
	env.AssertLogContains(t, `"module_key":"docker"`)
	env.AssertLogContains(t, `"cache_hit":false`)
	env.AssertLogContains(t, `"cache_hit":true`)
}

func TestNPMHookEmitsLogFields(t *testing.T) {
	stub := newUpstreamStub(t, upstreamNPM)
	defer stub.Close()

	env := newHookTestEnv(t, config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5310,
			CacheTTL:    config.Duration(time.Minute),
			StoragePath: t.TempDir(),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "npm",
				Domain:   "npm.hook.local",
				Type:     "npm",
				Upstream: stub.URL,
			},
		},
	})
	defer env.Close()

	assertCacheMissThenHit(t, env, "npm.hook.local", "/lodash")
	env.AssertLogContains(t, `"module_key":"npm"`)
}

func TestPyPIHookEmitsLogFields(t *testing.T) {
	stub := newPyPIStub(t)
	defer stub.Close()

	env := newHookTestEnv(t, config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5320,
			CacheTTL:    config.Duration(time.Minute),
			StoragePath: t.TempDir(),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "pypi",
				Domain:   "pypi.hook.local",
				Type:     "pypi",
				Upstream: stub.URL,
			},
		},
	})
	defer env.Close()

	assertCacheMissThenHit(t, env, "pypi.hook.local", "/simple/pkg/")
	env.AssertLogContains(t, `"module_key":"pypi"`)
}

func TestComposerHookEmitsLogFields(t *testing.T) {
	stub := newComposerStub(t)
	defer stub.Close()

	env := newHookTestEnv(t, config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5330,
			CacheTTL:    config.Duration(time.Minute),
			StoragePath: t.TempDir(),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "composer",
				Domain:   "composer.hook.local",
				Type:     "composer",
				Upstream: stub.URL,
			},
		},
	})
	defer env.Close()

	assertCacheMissThenHit(t, env, "composer.hook.local", "/p2/example/package.json")
	env.AssertLogContains(t, `"module_key":"composer"`)
}

func TestGoHookEmitsLogFields(t *testing.T) {
	stub := newGoStub(t)
	defer stub.Close()

	env := newHookTestEnv(t, config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5340,
			CacheTTL:    config.Duration(time.Minute),
			StoragePath: t.TempDir(),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "gomod",
				Domain:   "go.hook.local",
				Type:     "go",
				Upstream: stub.URL,
			},
		},
	})
	defer env.Close()

	assertCacheMissThenHit(t, env, "go.hook.local", goZipPath)
	env.AssertLogContains(t, `"module_key":"go"`)
}

func assertCacheMissThenHit(t *testing.T, env hookTestEnv, host, path string) {
	t.Helper()
	resp := env.DoRequest(t, host, path)
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200 for %s%s, got %d body=%s", host, path, resp.StatusCode, string(body))
	}
	if hit := resp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		resp.Body.Close()
		t.Fatalf("expected cache miss header, got %s", hit)
	}
	resp.Body.Close()

	resp2 := env.DoRequest(t, host, path)
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		resp2.Body.Close()
		t.Fatalf("expected cache hit header on second request, got %s", resp2.Header.Get("X-Any-Hub-Cache-Hit"))
	}
	resp2.Body.Close()
}

type hookTestEnv struct {
	app  *fiber.App
	logs *bytes.Buffer
}

func newHookTestEnv(t *testing.T, cfg config.Config) hookTestEnv {
	t.Helper()

	registry, err := server.NewHubRegistry(&cfg)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}

	logger := logrus.New()
	buf := &bytes.Buffer{}
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(buf)

	store, err := cache.NewStore(cfg.Global.StoragePath)
	if err != nil {
		t.Fatalf("store error: %v", err)
	}
	handler := proxy.NewHandler(server.NewUpstreamClient(&cfg), logger, store)

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: cfg.Global.ListenPort,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}
	return hookTestEnv{app: app, logs: buf}
}

func (env hookTestEnv) DoRequest(t *testing.T, host, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
	req.Host = host
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	return resp
}

func (env hookTestEnv) AssertLogContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(env.logs.String(), substr) {
		t.Fatalf("expected logs to contain %s, got %s", substr, env.logs.String())
	}
}

func (env hookTestEnv) Close() {
	_ = env.app.Shutdown()
}

const (
	goZipPath  = "/mod.example/@v/v1.0.0.zip"
	goInfoPath = "/mod.example/@v/v1.0.0.info"
)

type goStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	mu   sync.Mutex
	hits map[string]int
}

func newGoStub(t *testing.T) *goStub {
	t.Helper()
	stub := &goStub{hits: make(map[string]int)}
	mux := http.NewServeMux()
	mux.HandleFunc(goZipPath, stub.handleZip)
	mux.HandleFunc(goInfoPath, stub.handleInfo)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start go stub: %v", err)
	}
	server := &http.Server{Handler: mux}
	stub.listener = listener
	stub.server = server
	stub.URL = "http://" + listener.Addr().String()

	go func() {
		_ = server.Serve(listener)
	}()
	return stub
}

func (s *goStub) handleZip(w http.ResponseWriter, r *http.Request) {
	s.record(r.URL.Path)
	w.Header().Set("Content-Type", "application/zip")
	_, _ = w.Write([]byte("zip-bytes"))
}

func (s *goStub) handleInfo(w http.ResponseWriter, r *http.Request) {
	s.record(r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"Version":"v1.0.0"}`))
}

func (s *goStub) record(path string) {
	s.mu.Lock()
	s.hits[path]++
	s.mu.Unlock()
}

func (s *goStub) Close() {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if s.server != nil {
		_ = s.server.Shutdown(ctx)
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}
