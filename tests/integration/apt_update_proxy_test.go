package integration

import (
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

func TestAptUpdateCachesIndexes(t *testing.T) {
	stub := newAptStub(t)
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
				Name:     "apt",
				Domain:   "apt.hub.local",
				Type:     "debian",
				Module:   "debian",
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
		req := httptest.NewRequest("GET", "http://apt.hub.local"+path, nil)
		req.Host = "apt.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	releasePath := "/dists/bookworm/Release"
	packagesPath := "/dists/bookworm/main/binary-amd64/Packages.gz"
	contentsPath := "/dists/bookworm/main/Contents-amd64.gz"

	resp := doRequest(releasePath)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for release, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected cache miss for first release fetch")
	}
	resp.Body.Close()

	resp2 := doRequest(releasePath)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached release, got %d", resp2.StatusCode)
	}
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cache hit for release")
	}
	resp2.Body.Close()

	pkgResp := doRequest(packagesPath)
	if pkgResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for packages, got %d", pkgResp.StatusCode)
	}
	if pkgResp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected cache miss for packages")
	}
	pkgResp.Body.Close()

	pkgResp2 := doRequest(packagesPath)
	if pkgResp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached packages, got %d", pkgResp2.StatusCode)
	}
	if pkgResp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cache hit for packages")
	}
	pkgResp2.Body.Close()

	contentsResp := doRequest(contentsPath)
	if contentsResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for contents, got %d", contentsResp.StatusCode)
	}
	if contentsResp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected cache miss for contents")
	}
	contentsResp.Body.Close()

	contentsResp2 := doRequest(contentsPath)
	if contentsResp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached contents, got %d", contentsResp2.StatusCode)
	}
	if contentsResp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cache hit for contents")
	}
	contentsResp2.Body.Close()

	if stub.ReleaseGets() != 1 {
		t.Fatalf("expected single release GET, got %d", stub.ReleaseGets())
	}
	if stub.ReleaseHeads() != 1 {
		t.Fatalf("expected single release HEAD revalidate, got %d", stub.ReleaseHeads())
	}
	if stub.PackagesGets() != 1 {
		t.Fatalf("expected single packages GET, got %d", stub.PackagesGets())
	}
	if stub.PackagesHeads() != 1 {
		t.Fatalf("expected single packages HEAD revalidate, got %d", stub.PackagesHeads())
	}
	if stub.ContentsGets() != 1 {
		t.Fatalf("expected single contents GET, got %d", stub.ContentsGets())
	}
	if stub.ContentsHeads() != 1 {
		t.Fatalf("expected single contents HEAD revalidate, got %d", stub.ContentsHeads())
	}
}

type aptStub struct {
	server        *http.Server
	listener      net.Listener
	URL           string
	mu            sync.Mutex
	releaseBody   string
	packagesBody  string
	contentsBody  string
	releaseETag   string
	packagesETag  string
	contentsETag  string
	releaseGets   int
	releaseHeads  int
	packagesGets  int
	packagesHeads int
	contentsGets  int
	contentsHeads int
	releasePath   string
	packagesPath  string
	contentsPath  string
}

func newAptStub(t *testing.T) *aptStub {
	t.Helper()
	stub := &aptStub{
		releaseBody:  "Release-body",
		packagesBody: "Packages-body",
		contentsBody: "Contents-body",
		releaseETag:  "r1",
		packagesETag: "p1",
		contentsETag: "c1",
		releasePath:  "/dists/bookworm/Release",
		packagesPath: "/dists/bookworm/main/binary-amd64/Packages.gz",
		contentsPath: "/dists/bookworm/main/Contents-amd64.gz",
	}

	mux := http.NewServeMux()
	mux.HandleFunc(stub.releasePath, stub.handleRelease)
	mux.HandleFunc(stub.packagesPath, stub.handlePackages)
	mux.HandleFunc(stub.contentsPath, stub.handleContents)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to start apt stub: %v", err)
	}

	srv := &http.Server{Handler: mux}
	stub.server = srv
	stub.listener = listener
	stub.URL = "http://" + listener.Addr().String()

	go func() {
		_ = srv.Serve(listener)
	}()
	return stub
}

func (s *aptStub) handleRelease(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodHead {
		s.releaseHeads++
		if matchETag(r, s.releaseETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeHeaders(w, s.releaseETag)
		return
	}
	s.releaseGets++
	writeHeaders(w, s.releaseETag)
	_, _ = w.Write([]byte(s.releaseBody))
}

func (s *aptStub) handlePackages(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodHead {
		s.packagesHeads++
		if matchETag(r, s.packagesETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeHeaders(w, s.packagesETag)
		w.Header().Set("Content-Type", "application/gzip")
		return
	}
	s.packagesGets++
	writeHeaders(w, s.packagesETag)
	w.Header().Set("Content-Type", "application/gzip")
	_, _ = w.Write([]byte(s.packagesBody))
}

func (s *aptStub) handleContents(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodHead {
		s.contentsHeads++
		if matchETag(r, s.contentsETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeHeaders(w, s.contentsETag)
		w.Header().Set("Content-Type", "application/gzip")
		return
	}
	s.contentsGets++
	writeHeaders(w, s.contentsETag)
	w.Header().Set("Content-Type", "application/gzip")
	_, _ = w.Write([]byte(s.contentsBody))
}

func matchETag(r *http.Request, etag string) bool {
	for _, candidate := range r.Header.Values("If-None-Match") {
		c := strings.Trim(candidate, "\"")
		if c == etag || candidate == etag {
			return true
		}
	}
	return false
}

func writeHeaders(w http.ResponseWriter, etag string) {
	w.Header().Set("ETag", "\""+etag+"\"")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
}

func (s *aptStub) ReleaseGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.releaseGets
}

func (s *aptStub) ReleaseHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.releaseHeads
}

func (s *aptStub) PackagesGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packagesGets
}

func (s *aptStub) PackagesHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packagesHeads
}

func (s *aptStub) ContentsGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contentsGets
}

func (s *aptStub) ContentsHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contentsHeads
}

func (s *aptStub) Close() {
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
