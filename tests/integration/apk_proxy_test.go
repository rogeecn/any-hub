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

func TestAPKProxyCachesIndexAndPackages(t *testing.T) {
	stub := newAPKStub(t)
	defer stub.Close()

	storageDir := t.TempDir()
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:  5400,
			CacheTTL:    config.Duration(time.Hour),
			StoragePath: storageDir,
		},
		Hubs: []config.HubConfig{
			{
				Name:     "apk",
				Domain:   "apk.hub.local",
				Type:     "apk",
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
		ListenPort: 5400,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	doRequest := func(p string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "http://apk.hub.local"+p, nil)
		req.Host = "apk.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	resp := doRequest(stub.indexPath)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for index, got %d", resp.StatusCode)
	}
	if hit := resp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected cache miss on first index fetch, got %s", hit)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(body, stub.indexBody) {
		t.Fatalf("index body mismatch on first fetch: %d bytes", len(body))
	}

	resp2 := doRequest(stub.indexPath)
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached index, got %d", resp2.StatusCode)
	}
	if hit := resp2.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
		t.Fatalf("expected cache hit for index, got %s", hit)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if !bytes.Equal(body2, stub.indexBody) {
		t.Fatalf("index body mismatch on cache hit: %d bytes", len(body2))
	}

	sigResp := doRequest(stub.signaturePath)
	if sigResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for signature, got %d", sigResp.StatusCode)
	}
	if hit := sigResp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected cache miss on first signature fetch, got %s", hit)
	}
	_, _ = io.ReadAll(sigResp.Body)
	sigResp.Body.Close()

	sigResp2 := doRequest(stub.signaturePath)
	if sigResp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached signature, got %d", sigResp2.StatusCode)
	}
	if hit := sigResp2.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
		t.Fatalf("expected cache hit for signature, got %s", hit)
	}
	sigResp2.Body.Close()

	pkgResp := doRequest(stub.packagePath)
	if pkgResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for package, got %d", pkgResp.StatusCode)
	}
	if hit := pkgResp.Header.Get("X-Any-Hub-Cache-Hit"); hit != "false" {
		t.Fatalf("expected cache miss on first package fetch, got %s", hit)
	}
	pkgBody, _ := io.ReadAll(pkgResp.Body)
	pkgResp.Body.Close()
	if !bytes.Equal(pkgBody, stub.packageBody) {
		t.Fatalf("package body mismatch on first fetch: %d bytes", len(pkgBody))
	}

	pkgResp2 := doRequest(stub.packagePath)
	if pkgResp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for cached package, got %d", pkgResp2.StatusCode)
	}
	if hit := pkgResp2.Header.Get("X-Any-Hub-Cache-Hit"); hit != "true" {
		t.Fatalf("expected cache hit for package, got %s", hit)
	}
	pkgBody2, _ := io.ReadAll(pkgResp2.Body)
	pkgResp2.Body.Close()
	if !bytes.Equal(pkgBody2, stub.packageBody) {
		t.Fatalf("package body mismatch on cache hit: %d bytes", len(pkgBody2))
	}

	if stub.IndexGets() != 1 {
		t.Fatalf("expected single index GET, got %d", stub.IndexGets())
	}
	if stub.IndexHeads() != 1 {
		t.Fatalf("expected single index HEAD revalidate, got %d", stub.IndexHeads())
	}
	if stub.SignatureGets() != 1 {
		t.Fatalf("expected single signature GET, got %d", stub.SignatureGets())
	}
	if stub.SignatureHeads() != 1 {
		t.Fatalf("expected single signature HEAD revalidate, got %d", stub.SignatureHeads())
	}
	if stub.PackageGets() != 1 {
		t.Fatalf("expected single package GET, got %d", stub.PackageGets())
	}
	if stub.PackageHeads() != 0 {
		t.Fatalf("expected zero package HEAD revalidate, got %d", stub.PackageHeads())
	}

	verifyAPKStored(t, storageDir, "apk", stub.indexPath, int64(len(stub.indexBody)))
	verifyAPKStored(t, storageDir, "apk", stub.signaturePath, int64(len(stub.signatureBody)))
	verifyAPKStored(t, storageDir, "apk", stub.packagePath, int64(len(stub.packageBody)))
}

func verifyAPKStored(t *testing.T, basePath, hubName, locatorPath string, expectedSize int64) {
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

type apkStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	mu             sync.Mutex
	indexPath      string
	signaturePath  string
	packagePath    string
	indexBody      []byte
	signatureBody  []byte
	packageBody    []byte
	indexGets      int
	indexHeads     int
	signatureGets  int
	signatureHeads int
	packageGets    int
	packageHeads   int
}

func newAPKStub(t *testing.T) *apkStub {
	t.Helper()
	stub := &apkStub{
		indexPath:     "/v3.19/main/x86_64/APKINDEX.tar.gz",
		signaturePath: "/v3.19/main/x86_64/APKINDEX.tar.gz.asc",
		packagePath:   "/v3.22/community/x86_64/tini-static-0.19.0-r3.apk",
		indexBody:     []byte("apk-index-body"),
		signatureBody: []byte("apk-index-signature"),
		packageBody:   bytes.Repeat([]byte("apk-payload-"), 64*1024),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(stub.indexPath, stub.handleIndex)
	mux.HandleFunc(stub.signaturePath, stub.handleSignature)
	mux.HandleFunc(stub.packagePath, stub.handlePackage)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("unable to start apk stub: %v", err)
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

func (s *apkStub) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.handleWithETag(w, r, &s.indexGets, &s.indexHeads, s.indexBody, "application/gzip")
}

func (s *apkStub) handleSignature(w http.ResponseWriter, r *http.Request) {
	s.handleWithETag(w, r, &s.signatureGets, &s.signatureHeads, s.signatureBody, "application/pgp-signature")
}

func (s *apkStub) handlePackage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Method == http.MethodHead {
		s.packageHeads++
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
		w.WriteHeader(http.StatusOK)
		return
	}

	s.packageGets++
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Length", strconv.Itoa(len(s.packageBody)))
	_, _ = w.Write(s.packageBody)
}

func (s *apkStub) handleWithETag(w http.ResponseWriter, r *http.Request, gets, heads *int, body []byte, contentType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	etag := "\"apk-etag\""
	if r.Method == http.MethodHead {
		*heads++
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		if matchETag(r, strings.Trim(etag, `"`)) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		return
	}

	*gets++
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	_, _ = w.Write(body)
}

func (s *apkStub) IndexGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.indexGets
}

func (s *apkStub) IndexHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.indexHeads
}

func (s *apkStub) SignatureGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signatureGets
}

func (s *apkStub) SignatureHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signatureHeads
}

func (s *apkStub) PackageGets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packageGets
}

func (s *apkStub) PackageHeads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packageHeads
}

func (s *apkStub) Close() {
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
