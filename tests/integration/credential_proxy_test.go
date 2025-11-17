package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestCredentialProxy(t *testing.T) {
	t.Run("fails without credentials", func(t *testing.T) {
		stub := newCredentialAuthStub(t, "ci-user", "ci-pass")
		defer stub.Close()

		app := newCredentialProxyApp(t, stub, false, nil)
		resp := performCredentialRequest(t, app)
		if resp.StatusCode != http.StatusUnauthorized {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 401 when hub lacks credentials, got %d (body=%s)", resp.StatusCode, string(body))
		}
		resp.Body.Close()
		if stub.SuccessCount() != 0 {
			t.Fatalf("expected no successful upstream hits without credentials")
		}
	})

	t.Run("succeeds with credentials", func(t *testing.T) {
		stub := newCredentialAuthStub(t, "ci-user", "ci-pass")
		defer stub.Close()

		app := newCredentialProxyApp(t, stub, true, nil)
		resp := performCredentialRequest(t, app)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 when credentials configured, got %d (body=%s)", resp.StatusCode, string(body))
		}
		resp.Body.Close()
		if stub.SuccessCount() == 0 {
			t.Fatalf("expected at least one authorized upstream hit")
		}
		if last := stub.LastAuthorization(); last != stub.ExpectedAuthorization() {
			t.Fatalf("expected upstream to receive header %s, got %s", stub.ExpectedAuthorization(), last)
		}
	})

	t.Run("logs credentialed auth_mode for anonymous client", func(t *testing.T) {
		stub := newCredentialAuthStub(t, "ci-user", "ci-pass")
		defer stub.Close()

		logBuf := &bytes.Buffer{}
		app := newCredentialProxyApp(t, stub, true, logBuf)

		resp := performCredentialRequest(t, app)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 for first request, got %d (body=%s)", resp.StatusCode, string(body))
		}
		resp.Body.Close()

		entry := findLogEntry(t, logBuf, "proxy_complete")
		assertLogField(t, entry, "auth_mode", "credentialed")
		assertLogField(t, entry, "hub_type", "npm")
		assertLogField(t, entry, "cache_hit", false)
		assertLogField(t, entry, "upstream_status", float64(200))

		logBuf.Reset()
		resp2 := performCredentialRequest(t, app)
		if resp2.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp2.Body)
			t.Fatalf("expected 200 for cache hit, got %d (body=%s)", resp2.StatusCode, string(body))
		}
		resp2.Body.Close()
		entry = findLogEntry(t, logBuf, "proxy_complete")
		assertLogField(t, entry, "cache_hit", true)
	})

	t.Run("retries once when upstream temporarily rejects auth", func(t *testing.T) {
		stub := newCredentialAuthStub(t, "ci-user", "ci-pass")
		defer stub.Close()
		stub.FailNextAuthorizedRequests(1)

		app := newCredentialProxyApp(t, stub, true, nil)
		resp := performCredentialRequest(t, app)
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 after retry, got %d (body=%s)", resp.StatusCode, string(body))
		}
		resp.Body.Close()
		if stub.SuccessCount() != 1 {
			t.Fatalf("expected single successful upstream call after retry, got %d", stub.SuccessCount())
		}
		if stub.UnauthorizedCount() != 1 {
			t.Fatalf("expected one unauthorized response before retry, got %d", stub.UnauthorizedCount())
		}
		if stub.TotalRequests() != 2 {
			t.Fatalf("expected exactly two upstream attempts, got %d", stub.TotalRequests())
		}
	})

	t.Run("stops after single retry when upstream keeps failing", func(t *testing.T) {
		stub := newCredentialAuthStub(t, "ci-user", "ci-pass")
		defer stub.Close()
		stub.FailNextAuthorizedRequests(2)

		app := newCredentialProxyApp(t, stub, true, nil)
		resp := performCredentialRequest(t, app)
		if resp.StatusCode != http.StatusUnauthorized {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 401 after retry exhaustion, got %d (body=%s)", resp.StatusCode, string(body))
		}
		resp.Body.Close()
		if stub.TotalRequests() != 2 {
			t.Fatalf("expected two attempts (original + retry), got %d", stub.TotalRequests())
		}
	})
}

func TestDockerProxyHandlesBearerTokenExchange(t *testing.T) {
	stub := newDockerBearerStub(t, "ci-user", "ci-pass")
	defer stub.Close()

	app := newDockerProxyApp(t, stub)

	req := httptest.NewRequest("GET", "http://docker.hub.local/v2/library/alpine/manifests/latest", nil)
	req.Host = "docker.hub.local"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 after token exchange, got %d (body=%s)", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	if stub.TokenHits() != 1 {
		t.Fatalf("expected single token request, got %d", stub.TokenHits())
	}
	if stub.ManifestHits() != 2 {
		t.Fatalf("expected manifest retried after token, got %d hits", stub.ManifestHits())
	}
	expectedBearer := "Bearer " + stub.tokenValue
	if stub.ManifestAuth() != expectedBearer {
		t.Fatalf("expected manifest Authorization %s, got %s", expectedBearer, stub.ManifestAuth())
	}
	if stub.TokenAuth() != stub.ExpectedBasic() {
		t.Fatalf("expected token endpoint to receive basic auth %s, got %s", stub.ExpectedBasic(), stub.TokenAuth())
	}
}

func TestDockerProxyCachesAfterBearerRevalidation(t *testing.T) {
	stub := newDockerBearerStub(t, "ci-user", "ci-pass")
	defer stub.Close()

	app := newDockerProxyApp(t, stub)

	req := httptest.NewRequest("GET", "http://docker.hub.local/v2/library/alpine/manifests/latest", nil)
	req.Host = "docker.hub.local"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 after token exchange, got %d (body=%s)", resp.StatusCode, string(body))
	}
	if resp.Header.Get("X-Any-Hub-Cache-Hit") != "false" {
		t.Fatalf("expected first request to miss cache")
	}
	resp.Body.Close()

	req2 := httptest.NewRequest("GET", "http://docker.hub.local/v2/library/alpine/manifests/latest", nil)
	req2.Host = "docker.hub.local"
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 after cache revalidation, got %d (body=%s)", resp2.StatusCode, string(body))
	}
	if resp2.Header.Get("X-Any-Hub-Cache-Hit") != "true" {
		t.Fatalf("expected second request to be served from cache")
	}
	resp2.Body.Close()

	if hits := stub.ManifestHits(); hits != 4 {
		t.Fatalf("expected 4 manifest hits (2 GET + 2 HEAD), got %d", hits)
	}
	if tokens := stub.TokenHits(); tokens != 2 {
		t.Fatalf("expected token endpoint to be called twice, got %d", tokens)
	}
}

func performCredentialRequest(t *testing.T, app *fiber.App) *http.Response {
	t.Helper()
	req := httptest.NewRequest("GET", "http://secure.hub.local/private/data", nil)
	req.Host = "secure.hub.local"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	return resp
}

func newCredentialProxyApp(t *testing.T, stub *credentialAuthStub, withCredentials bool, logSink io.Writer) *fiber.App {
	t.Helper()

	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:      5000,
			StoragePath:     t.TempDir(),
			CacheTTL:        config.Duration(time.Hour),
			MaxMemoryCache:  1,
			MaxRetries:      0,
			InitialBackoff:  config.Duration(time.Second),
			UpstreamTimeout: config.Duration(30 * time.Second),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "secure",
				Domain:   "secure.hub.local",
				Type:     "npm",
				Upstream: stub.URL,
			},
		},
	}
	if withCredentials {
		cfg.Hubs[0].Username = stub.username
		cfg.Hubs[0].Password = stub.password
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}

	logger := logrus.New()
	if logSink != nil {
		logger.SetOutput(logSink)
	} else {
		logger.SetOutput(io.Discard)
	}
	logger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})

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

func newDockerProxyApp(t *testing.T, stub *dockerBearerStub) *fiber.App {
	t.Helper()
	cfg := &config.Config{
		Global: config.GlobalConfig{
			ListenPort:      5000,
			StoragePath:     t.TempDir(),
			CacheTTL:        config.Duration(time.Hour),
			MaxMemoryCache:  1,
			MaxRetries:      0,
			InitialBackoff:  config.Duration(time.Second),
			UpstreamTimeout: config.Duration(30 * time.Second),
		},
		Hubs: []config.HubConfig{
			{
				Name:     "docker",
				Domain:   "docker.hub.local",
				Type:     "docker",
				Upstream: stub.URL,
				Username: stub.username,
				Password: stub.password,
			},
		},
	}

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

type credentialAuthStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	username string
	password string

	mu             sync.Mutex
	lastAuth       string
	successCount   int
	unauthCount    int
	totalRequests  int
	expectedBasic  string
	forceFailures  int
	initialFailure int
}

func newCredentialAuthStub(t *testing.T, username, password string) *credentialAuthStub {
	t.Helper()

	stub := &credentialAuthStub{
		username:      username,
		password:      password,
		expectedBasic: "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/private/data", stub.handle)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start upstream stub listener: %v", err)
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

func (s *credentialAuthStub) handle(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	s.mu.Lock()
	s.totalRequests++
	s.lastAuth = auth

	shouldForceFail := false
	if auth == s.expectedBasic && s.forceFailures > 0 {
		s.forceFailures--
		shouldForceFail = true
	}

	if auth == s.expectedBasic && !shouldForceFail {
		s.successCount++
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}

	s.unauthCount++
	s.mu.Unlock()

	if auth != s.expectedBasic || shouldForceFail {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("missing or invalid auth"))
		return
	}
}

func (s *credentialAuthStub) Close() {
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

func (s *credentialAuthStub) LastAuthorization() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAuth
}

func (s *credentialAuthStub) SuccessCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.successCount
}

func (s *credentialAuthStub) ExpectedAuthorization() string {
	return s.expectedBasic
}

func (s *credentialAuthStub) UnauthorizedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unauthCount
}

func (s *credentialAuthStub) TotalRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalRequests
}

func (s *credentialAuthStub) FailNextAuthorizedRequests(n int) {
	s.mu.Lock()
	s.forceFailures = n
	s.mu.Unlock()
}

type dockerBearerStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	username     string
	password     string
	expectedBasic string
	tokenValue    string

	mu           sync.Mutex
	manifestAuth string
	tokenAuth    string
	manifestHits int
	tokenHits    int
	lastModified time.Time
}

func newDockerBearerStub(t *testing.T, username, password string) *dockerBearerStub {
	t.Helper()
	stub := &dockerBearerStub{
		username:      username,
		password:      password,
		expectedBasic: "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))),
		tokenValue:    "test-token",
		lastModified:  time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", stub.handleProbe)
	mux.HandleFunc("/v2/library/alpine/manifests/latest", stub.handleManifest)
	mux.HandleFunc("/token", stub.handleToken)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start docker stub listener: %v", err)
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

func (s *dockerBearerStub) handleManifest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.manifestHits++
	s.manifestAuth = r.Header.Get("Authorization")
	authHeader := fmt.Sprintf(`Bearer realm="%s/token",service="registry.test",scope="repository:library/alpine:pull"`, s.URL)
	expectBearer := "Bearer " + s.tokenValue
	success := s.manifestAuth == expectBearer
	s.mu.Unlock()

	if success {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Last-Modified", s.lastModified.Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte(`{"schemaVersion":2}`))
		return
	}

	w.Header().Set("Www-Authenticate", authHeader)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("token required"))
}

func (s *dockerBearerStub) handleToken(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.tokenHits++
	s.tokenAuth = r.Header.Get("Authorization")
	service := r.URL.Query().Get("service")
	scope := r.URL.Query().Get("scope")
	expectAuth := s.expectedBasic
	valid := s.tokenAuth == expectAuth && service == "registry.test" && scope == "repository:library/alpine:pull"
	s.mu.Unlock()

	if !valid {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid credentials"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{"token": s.tokenValue}
	data, _ := json.Marshal(resp)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *dockerBearerStub) handleProbe(w http.ResponseWriter, r *http.Request) {
	authHeader := fmt.Sprintf(`Bearer realm="%s/token",service="registry.test",scope="repository:library/alpine:pull"`, s.URL)
	if r.Header.Get("Authorization") == "Bearer "+s.tokenValue {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	w.Header().Set("Www-Authenticate", authHeader)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("probe auth required"))
}

func (s *dockerBearerStub) Close() {
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

func (s *dockerBearerStub) ManifestAuth() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manifestAuth
}

func (s *dockerBearerStub) TokenAuth() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenAuth
}

func (s *dockerBearerStub) ManifestHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manifestHits
}

func (s *dockerBearerStub) TokenHits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenHits
}

func (s *dockerBearerStub) ExpectedBasic() string {
	return s.expectedBasic
}

func findLogEntry(t *testing.T, buf *bytes.Buffer, msg string) map[string]any {
	t.Helper()
	entries := parseLogBuffer(t, buf)
	for _, entry := range entries {
		if entry["msg"] == msg {
			return entry
		}
	}
	t.Fatalf("log entry with msg=%s not found; entries=%v", msg, entries)
	return nil
}

func parseLogBuffer(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var result []map[string]any
	for {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read log buffer: %v", err)
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(line), &entry); err != nil {
			t.Fatalf("parse log entry: %v (line=%s)", err, string(line))
		}
		result = append(result, entry)
	}
	return result
}

func assertLogField(t *testing.T, entry map[string]any, key string, expected any) {
	t.Helper()
	value, ok := entry[key]
	if !ok {
		t.Fatalf("log entry missing %s field: %v", key, entry)
	}
	if value != expected {
		t.Fatalf("log entry %s mismatch: expected %v got %v", key, expected, value)
	}
}
