package integration

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestComposerProxyCachesMetadataAndDists(t *testing.T) {
	stub := newComposerStub(t)
	defer stub.Close()

	storageDir := t.TempDir()
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5000,
			CacheTTL:    config.Duration(time.Hour),
			StoragePath: storageDir,
		},
		Hubs: []config.HubConfig{
			{
				Name:     "composer",
				Domain:   "composer.hub.local",
				Type:     "composer",
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

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      proxy.NewHandler(server.NewUpstreamClient(cfg), logger, store),
		ListenPort: 5000,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	doRequest := func(path string) *http.Response {
		req := httptest.NewRequest("GET", "http://composer.hub.local"+path, nil)
		req.Host = "composer.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	metaPath := "/p2/example/package.json"
	resp := doRequest(metaPath)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for composer metadata, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected metadata content-type json, got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected metadata miss on first request")
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var meta composerMetadataPayload
	if err := json.Unmarshal(body, &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	distURL := meta.FindDistURL("example/package")
	if distURL == "" {
		t.Fatalf("metadata missing dist url: %s", string(body))
	}
	parsedDist, err := url.Parse(distURL)
	if err != nil {
		t.Fatalf("parse dist url: %v", err)
	}
	if parsedDist.Host != "composer.hub.local" {
		t.Fatalf("expected dist url rewritten to proxy host, got %s", parsedDist.Host)
	}

	resp2 := doRequest(metaPath)
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected metadata cache hit on second request")
	}
	resp2.Body.Close()

	distResp := doRequest(parsedDist.RequestURI())
	if distResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected dist 200, got %d", distResp.StatusCode)
	}
	if distResp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected dist miss on first download")
	}
	distBody, _ := io.ReadAll(distResp.Body)
	distResp.Body.Close()
	if string(distBody) != stub.DistContent() {
		t.Fatalf("unexpected dist body, got %s", string(distBody))
	}

	distResp2 := doRequest(parsedDist.RequestURI())
	if distResp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cached dist response")
	}
	distResp2.Body.Close()

	if stub.MetadataHits() != 1 {
		t.Fatalf("expected single upstream metadata GET, got %d", stub.MetadataHits())
	}
	if stub.DistHits() != 1 {
		t.Fatalf("expected single upstream dist GET, got %d", stub.DistHits())
	}
}

type composerMetadataPayload struct {
	Packages map[string][]composerMetadataVersion `json:"packages"`
}

type composerMetadataVersion struct {
	Dist struct {
		URL string `json:"url"`
	} `json:"dist"`
}

func (m composerMetadataPayload) FindDistURL(pkg string) string {
	versions, ok := m.Packages[pkg]
	if !ok || len(versions) == 0 {
		return ""
	}
	return versions[0].Dist.URL
}

type composerStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	mu            sync.Mutex
	metadataHits  int
	distHits      int
	distBody      string
	metadataBody  []byte
	metadataPath  string
	distPath      string
}

func newComposerStub(t *testing.T) *composerStub {
	t.Helper()
	stub := &composerStub{
		distBody:     "zip-bytes",
		metadataPath: "/p2/example/package.json",
		distPath:     "/downloads/example-package-1.0.0.zip",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/packages.json", stub.handlePackages)
	mux.HandleFunc(stub.metadataPath, stub.handleMetadata)
	mux.HandleFunc(stub.distPath, stub.handleDist)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start composer stub: %v", err)
	}

	server := &http.Server{Handler: mux}
	stub.server = server
	stub.listener = listener
	stub.URL = "http://" + listener.Addr().String()
	stub.metadataBody = stub.buildMetadata()

	go func() {
		_ = server.Serve(listener)
	}()

	return stub
}

func (s *composerStub) buildMetadata() []byte {
	payload := map[string]any{
		"packages": map[string][]map[string]any{
			"example/package": {
				{
					"name":    "example/package",
					"version": "1.0.0",
					"dist": map[string]any{
						"type": "zip",
						"url":  s.URL + s.distPath,
					},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

func (s *composerStub) handlePackages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"packages":{}}`))
}

func (s *composerStub) handleMetadata(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.metadataHits++
	body := s.metadataBody
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func (s *composerStub) handleDist(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.distHits++
	body := s.distBody
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/zip")
	_, _ = w.Write([]byte(body))
}

func (s *composerStub) MetadataHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metadataHits
}

func (s *composerStub) DistHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.distHits
}

func (s *composerStub) DistContent() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.distBody
}

func (s *composerStub) Close() {
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
