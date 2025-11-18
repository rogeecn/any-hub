package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

const (
	dockerManifestPath            = "/v2/library/cache-flow/manifests/latest"
	dockerManifestNoNamespacePath = "/v2/cache-flow/manifests/latest"
)

func TestCacheFlowWithConditionalRequest(t *testing.T) {
	upstream := newCacheFlowStub(t, dockerManifestPath)
	defer upstream.Close()

	storageDir := t.TempDir()
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5000,
			CacheTTL:    config.Duration(30 * time.Second),
			StoragePath: storageDir,
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Module:   "docker",
				Upstream: upstream.URL,
			},
		},
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	store, err := cache.NewStore(storageDir)
	if err != nil {
		t.Fatalf("store error: %v", err)
	}

	client := server.NewUpstreamClient(cfg)
	handler := proxy.NewHandler(client, logger, store)

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: 5000,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	doRequest := func() *http.Response {
		req := httptest.NewRequest("GET", "http://docker.hub.local"+dockerManifestPath, nil)
		req.Host = "docker.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	// Miss -> upstream fetch
	resp := doRequest()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if hit := resp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected cache miss header, got %s", hit)
	}
	resp.Body.Close()

	// Cache hit with upstream HEAD revalidation
	resp2 := doRequest()
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cache hit on second request")
	}
	resp2.Body.Close()

	if upstream.hits != 1 {
		t.Fatalf("expected single upstream GET, got %d", upstream.hits)
	}
	if upstream.headHits != 1 {
		t.Fatalf("expected single upstream HEAD, got %d", upstream.headHits)
	}

	// Simulate upstream update and ensure cache refreshes.
	upstream.UpdateBody([]byte("upstream v2"))
	resp3 := doRequest()
	if resp3.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected refresh when upstream changes")
	}
	body, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	if string(body) != "upstream v2" {
		t.Fatalf("unexpected body after refresh: %s", string(body))
	}

	if upstream.hits != 2 {
		t.Fatalf("expected upstream GET refresh, got %d hits", upstream.hits)
	}
	if upstream.headHits != 2 {
		t.Fatalf("expected second HEAD before refresh, got %d", upstream.headHits)
	}
}

func TestDockerManifestHeadDoesNotOverwriteCache(t *testing.T) {
	upstream := newCacheFlowStub(t, dockerManifestPath)
	defer upstream.Close()

	storageDir := t.TempDir()
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5000,
			CacheTTL:    config.Duration(time.Minute),
			StoragePath: storageDir,
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Module:   "docker",
				Upstream: upstream.URL,
			},
		},
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	store, err := cache.NewStore(storageDir)
	if err != nil {
		t.Fatalf("store error: %v", err)
	}

	client := server.NewUpstreamClient(cfg)
	handler := proxy.NewHandler(client, logger, store)

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: 5000,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	doRequest := func(method string) *http.Response {
		req := httptest.NewRequest(method, "http://docker.hub.local"+dockerManifestPath, nil)
		req.Host = "docker.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	resp := doRequest(http.MethodGet)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	headResp := doRequest(http.MethodHead)
	if headResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for HEAD, got %d", headResp.StatusCode)
	}
	headResp.Body.Close()

	if upstream.hits != 1 {
		t.Fatalf("expected upstream hit only for initial GET, got %d", upstream.hits)
	}
	if upstream.headHits != 2 {
		t.Fatalf("expected two upstream HEAD calls (explicit + revalidation), got %d", upstream.headHits)
	}

	cachedPath := filepath.Join(storageDir, "docker", "v2", "library", "cache-flow", "manifests", "latest")
	info, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatalf("stat cached manifest: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected cached manifest to remain non-empty")
	}

	resp2 := doRequest(http.MethodGet)
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if string(body) != string(upstream.body) {
		t.Fatalf("unexpected cached body after HEAD: %s", string(body))
	}
}

func TestDockerNamespaceFallbackAddsLibrary(t *testing.T) {
	stub := newCacheFlowStub(t, dockerManifestPath)
	defer stub.Close()

	storageDir := t.TempDir()
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5000,
			CacheTTL:    config.Duration(30 * time.Second),
			StoragePath: storageDir,
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Upstream: stub.URL,
			},
		},
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	store, err := cache.NewStore(storageDir)
	if err != nil {
		t.Fatalf("store error: %v", err)
	}

	client := server.NewUpstreamClient(cfg)
	handler := proxy.NewHandler(client, logger, store)

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: 5000,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	req := httptest.NewRequest("GET", "http://docker.hub.local"+dockerManifestNoNamespacePath, nil)
	req.Host = "docker.hub.local"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 when fallback applies, got %d (body=%s)", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	if stub.hits != 1 {
		t.Fatalf("expected single upstream hit, got %d", stub.hits)
	}
}

type cacheFlowStub struct {
	server      *http.Server
	listener    net.Listener
	URL         string
	mu          sync.Mutex
	hits        int
	headHits    int
	lastRequest *http.Request
	body        []byte
	etag        string
	etagVer     int
	lastMod     string
}

func newCacheFlowStub(t *testing.T, paths ...string) *cacheFlowStub {
	t.Helper()
	stub := &cacheFlowStub{
		body:    []byte("upstream payload"),
		etag:    `"etag-v1"`,
		etagVer: 1,
		lastMod: time.Now().UTC().Format(http.TimeFormat),
	}

	if len(paths) == 0 {
		paths = []string{"/pkg"}
	}

	mux := http.NewServeMux()
	for _, p := range paths {
		mux.HandleFunc(p, stub.handle)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start stub listener: %v", err)
	}

	server := &http.Server{Handler: mux}
	stub.server = server
	stub.listener = listener
	stub.URL = "http://" + listener.Addr().String()

	go func() {
		_ = server.Serve(listener)
	}()

	return stub
}

func (s *cacheFlowStub) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if s.server != nil {
		_ = s.server.Shutdown(ctx)
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

func (s *cacheFlowStub) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	etag := s.etag
	lastMod := s.lastMod
	if r.Method == http.MethodHead {
		s.headHits++
	} else {
		s.hits++
	}
	s.lastRequest = r.Clone(context.Background())
	s.mu.Unlock()

	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Etag", etag)
		w.Header().Set("Last-Modified", lastMod)
		for _, candidate := range r.Header.Values("If-None-Match") {
			if strings.Trim(candidate, `"`) == strings.Trim(etag, `"`) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Etag", etag)
	w.Header().Set("Last-Modified", lastMod)
	_, _ = w.Write(s.body)
}

func (s *cacheFlowStub) UpdateBody(body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body = body
	s.etagVer++
	s.etag = fmt.Sprintf(`"etag-v%d"`, s.etagVer)
	s.lastMod = time.Now().UTC().Add(2 * time.Second).Format(http.TimeFormat)
}
