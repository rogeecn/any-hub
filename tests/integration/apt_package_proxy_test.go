package integration

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
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

func TestAptPackagesCachedWithoutRevalidate(t *testing.T) {
	stub := newAptPackageStub(t)
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

	doRequest := func(p string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "http://apt.hub.local"+p, nil)
		req.Host = "apt.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	resp := doRequest(stub.packagePath)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for package, got %d", resp.StatusCode)
	}
	if hit := resp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected cache miss on first package fetch, got %s", hit)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(body, stub.packageBody) {
		t.Fatalf("package body mismatch on first fetch: %d bytes", len(body))
	}

	resp2 := doRequest(stub.packagePath)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached package, got %d", resp2.StatusCode)
	}
	if hit := resp2.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
		t.Fatalf("expected cache hit for package, got %s", hit)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if !bytes.Equal(body2, stub.packageBody) {
		t.Fatalf("package body mismatch on cache hit: %d bytes", len(body2))
	}

	hashResp := doRequest(stub.byHashPath)
	if hashResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for by-hash, got %d", hashResp.StatusCode)
	}
	if hit := hashResp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected cache miss on first by-hash fetch, got %s", hit)
	}
	hashBody, _ := io.ReadAll(hashResp.Body)
	hashResp.Body.Close()
	if !bytes.Equal(hashBody, stub.byHashBody) {
		t.Fatalf("by-hash body mismatch on first fetch: %d bytes", len(hashBody))
	}

	hashResp2 := doRequest(stub.byHashPath)
	if hashResp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached by-hash, got %d", hashResp2.StatusCode)
	}
	if hit := hashResp2.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
		t.Fatalf("expected cache hit for by-hash, got %s", hit)
	}
	hashBody2, _ := io.ReadAll(hashResp2.Body)
	hashResp2.Body.Close()
	if !bytes.Equal(hashBody2, stub.byHashBody) {
		t.Fatalf("by-hash body mismatch on cache hit: %d bytes", len(hashBody2))
	}

	if stub.PackageGets() != 1 {
		t.Fatalf("expected single package GET, got %d", stub.PackageGets())
	}
	if stub.PackageHeads() != 0 {
		t.Fatalf("expected zero package HEAD revalidate, got %d", stub.PackageHeads())
	}
	if stub.ByHashGets() != 1 {
		t.Fatalf("expected single by-hash GET, got %d", stub.ByHashGets())
	}
	if stub.ByHashHeads() != 0 {
		t.Fatalf("expected zero by-hash HEAD revalidate, got %d", stub.ByHashHeads())
	}

	verifyStoredFile(t, storageDir, "apt", stub.packagePath, int64(len(stub.packageBody)))
	verifyStoredFile(t, storageDir, "apt", stub.byHashPath, int64(len(stub.byHashBody)))
}

func verifyStoredFile(t *testing.T, basePath, hubName, locatorPath string, expectedSize int64) {
	t.Helper()
	clean := path.Clean("/" + locatorPath)
	clean = strings.TrimPrefix(clean, "/")
	fullPath := filepath.Join(basePath, hubName, clean)
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatalf("expected cached file at %s: %v", fullPath, err)
	}
	if info.Size() != expectedSize {
		t.Fatalf("cached file %s size mismatch: got %d want %d", fullPath, info.Size(), expectedSize)
	}
}

type aptPackageStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	mu           sync.Mutex
	packagePath  string
	byHashPath   string
	packageBody  []byte
	byHashBody   []byte
	packageGets  int
	packageHeads int
	byHashGets   int
	byHashHeads  int
}

func newAptPackageStub(t *testing.T) *aptPackageStub {
	t.Helper()

	stub := &aptPackageStub{
		packagePath: "/pool/main/h/hello_1.0_amd64.deb",
		byHashPath:  "/dists/bookworm/by-hash/sha256/deadbeef",
		packageBody: bytes.Repeat([]byte("deb-payload-"), 128*1024),
		byHashBody:  []byte("hash-index-body"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(stub.packagePath, stub.handlePackage)
	mux.HandleFunc(stub.byHashPath, stub.handleByHash)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to start apt package stub: %v", err)
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

func (s *aptPackageStub) handlePackage(w http.ResponseWriter, r *http.Request) {
	s.handleImmutable(w, r, &s.packageGets, &s.packageHeads, s.packageBody, "application/vnd.debian.binary-package")
}

func (s *aptPackageStub) handleByHash(w http.ResponseWriter, r *http.Request) {
	s.handleImmutable(w, r, &s.byHashGets, &s.byHashHeads, s.byHashBody, "text/plain")
}

func (s *aptPackageStub) handleImmutable(w http.ResponseWriter, r *http.Request, gets, heads *int, body []byte, contentType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Method == http.MethodHead {
		*heads++
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	*gets++
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

func (s *aptPackageStub) PackageGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packageGets
}

func (s *aptPackageStub) PackageHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packageHeads
}

func (s *aptPackageStub) ByHashGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byHashGets
}

func (s *aptPackageStub) ByHashHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byHashHeads
}

func (s *aptPackageStub) Close() {
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
