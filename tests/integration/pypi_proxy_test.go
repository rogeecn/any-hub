package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestPyPICachePolicies(t *testing.T) {
	stub := newPyPIStub(t)
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
				Name:     "pypi",
				Domain:   "pypi.hub.local",
				Type:     "pypi",
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

	handler := proxy.NewHandler(server.NewUpstreamClient(cfg), logger, store)

	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      handler,
		ListenPort: 5000,
	})
	if err != nil {
		t.Fatalf("app error: %v", err)
	}

	doRequest := func(path string) *http.Response {
		req := httptest.NewRequest("GET", "http://pypi.hub.local"+path, nil)
		req.Host = "pypi.hub.local"
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test error: %v", err)
		}
		return resp
	}

	simplePath := "/simple/pkg/"
	resp := doRequest(simplePath)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for simple index, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected miss for first simple request")
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "/files/") {
		t.Fatalf("simple response should rewrite file links, got %s", string(body))
	}

	resp2 := doRequest(simplePath)
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cached simple response after HEAD revalidation")
	}
	resp2.Body.Close()

	if stub.simpleHeadHits != 1 {
		t.Fatalf("expected single HEAD for simple index, got %d", stub.simpleHeadHits)
	}

	stub.UpdateSimple([]byte("<html>updated</html>"))
	resp3 := doRequest(simplePath)
	if resp3.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected miss after simple index update")
	}
	resp3.Body.Close()

	if stub.simpleHits != 2 {
		t.Fatalf("expected second GET for updated index, got %d", stub.simpleHits)
	}
	if stub.simpleHeadHits != 2 {
		t.Fatalf("expected second HEAD before refresh, got %d", stub.simpleHeadHits)
	}

	wheelURL := fmt.Sprintf("%s/packages/foo/foo-1.0-py3-none-any.whl", stub.URL)
	parsedWheel, err := url.Parse(wheelURL)
	if err != nil {
		t.Fatalf("wheel url parse: %v", err)
	}
	wheelPath := fmt.Sprintf("/files/%s/%s%s", parsedWheel.Scheme, parsedWheel.Host, parsedWheel.Path)
	respWheel := doRequest(wheelPath)
	if respWheel.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for wheel, got %d", respWheel.StatusCode)
	}
	if respWheel.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected miss for first wheel request")
	}
	respWheel.Body.Close()

	respWheel2 := doRequest(wheelPath)
	if respWheel2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected cached wheel response without revalidation")
	}
	respWheel2.Body.Close()

	if stub.wheelHeadHits != 0 {
		t.Fatalf("wheel path should not perform HEAD, got %d", stub.wheelHeadHits)
	}

	// bare project path should fallback to /simple/<name>/.
	bareResp := doRequest("/pkg/")
	if bareResp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(bareResp.Body)
		t.Fatalf("expected fallback success for bare path, got %d body=%s", bareResp.StatusCode, string(body))
	}
	bareResp.Body.Close()
}

type pypiStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	mu             sync.Mutex
	simpleHits     int
	simpleHeadHits int
	wheelHits      int
	wheelHeadHits  int
	simpleBody     []byte
	wheelBody      []byte
	lastSimpleMod  string
	wheelPath      string
}

func newPyPIStub(t *testing.T) *pypiStub {
	t.Helper()
	stub := &pypiStub{
		wheelPath:     "/packages/foo/foo-1.0-py3-none-any.whl",
		wheelBody:     []byte("wheel-bytes"),
		lastSimpleMod: time.Now().UTC().Format(http.TimeFormat),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/simple/pkg/", stub.handleSimple)
	mux.HandleFunc(stub.wheelPath, stub.handleWheel)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start pypi stub: %v", err)
	}

	server := &http.Server{Handler: mux}
	stub.server = server
	stub.listener = listener
	stub.URL = "http://" + listener.Addr().String()
	stub.simpleBody = stub.defaultSimpleHTML()

	go func() {
		_ = server.Serve(listener)
	}()

	return stub
}

func (s *pypiStub) handleSimple(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if r.Method == http.MethodHead {
		s.simpleHeadHits++
	} else {
		s.simpleHits++
	}
	body := append([]byte(nil), s.simpleBody...)
	lastMod := s.lastSimpleMod
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Last-Modified", lastMod)
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *pypiStub) handleWheel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if r.Method == http.MethodHead {
		s.wheelHeadHits++
	} else {
		s.wheelHits++
	}
	body := append([]byte(nil), s.wheelBody...)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *pypiStub) UpdateSimple(body []byte) {
	s.mu.Lock()
	s.simpleBody = append([]byte(nil), body...)
	s.lastSimpleMod = time.Now().UTC().Format(http.TimeFormat)
	s.mu.Unlock()
}

func (s *pypiStub) defaultSimpleHTML() []byte {
	return []byte(fmt.Sprintf(`<html><body><a href="%s%s">wheel</a></body></html>`, s.URL, s.wheelPath))
}

func (s *pypiStub) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if s.server != nil {
		_ = s.server.Shutdown(ctx)
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}
