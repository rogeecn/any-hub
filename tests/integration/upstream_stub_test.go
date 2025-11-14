package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path"
	"sync"
	"testing"
	"time"
)

type upstreamMode string

const (
	upstreamDocker upstreamMode = "docker"
	upstreamNPM    upstreamMode = "npm"
)

// upstreamStub 暴露简单的 Docker/NPM 上游模拟器，供集成测试复用。
type upstreamStub struct {
	server   *http.Server
	listener net.Listener
	URL      string

	mu        sync.Mutex
	requests  []RecordedRequest
	mode      upstreamMode
	blobBytes []byte
}

// RecordedRequest 捕获每次请求的方法/路径/Host/Headers，便于断言代理行为。
type RecordedRequest struct {
	Method  string
	Path    string
	Host    string
	Headers http.Header
	Body    []byte
}

func newUpstreamStub(t *testing.T, mode upstreamMode) *upstreamStub {
	t.Helper()

	mux := http.NewServeMux()
	stub := &upstreamStub{
		mode:      mode,
		blobBytes: []byte("stub-layer-payload"),
	}

	switch mode {
	case upstreamDocker:
		registerDockerHandlers(mux, stub.blobBytes)
	case upstreamNPM:
		registerNPMHandlers(mux)
	default:
		t.Fatalf("unsupported stub mode: %s", mode)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.recordRequest(r)
		mux.ServeHTTP(w, r)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to start upstream stub listener: %v", err)
	}
	server := &http.Server{Handler: handler}

	stub.server = server
	stub.listener = listener
	stub.URL = "http://" + listener.Addr().String()

	go func() {
		_ = server.Serve(listener)
	}()

	return stub
}

func (s *upstreamStub) Close() {
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

func (s *upstreamStub) recordRequest(r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	s.mu.Lock()
	s.requests = append(s.requests, RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Host:    r.Host,
		Headers: cloneHeader(r.Header),
		Body:    body,
	})
	s.mu.Unlock()
	r.Body = io.NopCloser(bytes.NewReader(body))
}

func (s *upstreamStub) Requests() []RecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]RecordedRequest, len(s.requests))
	copy(result, s.requests)
	return result
}

func registerDockerHandlers(mux *http.ServeMux, blob []byte) {
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"docker":"ok"}`))
	})

	mux.HandleFunc("/v2/library/sample/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		resp := map[string]any{
			"schemaVersion": 2,
			"name":          "library/sample",
			"tag":           "latest",
			"layers": []map[string]any{
				{
					"digest":    "sha256:deadbeef",
					"size":      len(blob),
					"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/v2/library/sample/blobs/", func(w http.ResponseWriter, r *http.Request) {
		digest := path.Base(r.URL.Path)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Docker-Content-Digest", digest)
		_, _ = w.Write(blob)
	})
}

func registerNPMHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/lodash", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"name": "lodash",
			"dist-tags": map[string]string{
				"latest": "4.17.21",
			},
			"versions": map[string]any{
				"4.17.21": map[string]any{
					"dist": map[string]any{
						"tarball": r.Host + "/lodash/-/lodash-4.17.21.tgz",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/lodash/-/lodash-4.17.21.tgz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("tarball-bytes"))
	})
}

func cloneHeader(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for k, values := range src {
		cp := make([]string, len(values))
		copy(cp, values)
		dst[k] = cp
	}
	return dst
}

func TestDockerStubServesManifestAndBlob(t *testing.T) {
	stub := newUpstreamStub(t, upstreamDocker)
	defer stub.Close()

	pingResp, err := http.Get(stub.URL + "/v2/")
	if err != nil {
		t.Fatalf("docker ping failed: %v", err)
	}
	pingResp.Body.Close()
	if pingResp.StatusCode != http.StatusOK {
		t.Fatalf("docker ping unexpected status: %d", pingResp.StatusCode)
	}

	resp, err := http.Get(stub.URL + "/v2/library/sample/manifests/latest")
	if err != nil {
		t.Fatalf("manifest request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected manifest status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte(`"name":"library/sample"`)) {
		t.Fatalf("manifest body unexpected: %s", string(body))
	}

	layerResp, err := http.Get(stub.URL + "/v2/library/sample/blobs/sha256:deadbeef")
	if err != nil {
		t.Fatalf("layer request failed: %v", err)
	}
	defer layerResp.Body.Close()
	layer, _ := io.ReadAll(layerResp.Body)
	if !bytes.Equal(layer, stub.blobBytes) {
		t.Fatalf("layer bytes mismatch: %s", string(layer))
	}

	if got := len(stub.Requests()); got != 3 {
		t.Fatalf("expected 3 recorded requests, got %d", got)
	}
}

func TestNPMStubServesMetadataAndTarball(t *testing.T) {
	stub := newUpstreamStub(t, upstreamNPM)
	defer stub.Close()

	resp, err := http.Get(stub.URL + "/lodash")
	if err != nil {
		t.Fatalf("metadata request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte(`"latest":"4.17.21"`)) {
		t.Fatalf("metadata unexpected: %s", string(body))
	}

	tarballResp, err := http.Get(stub.URL + "/lodash/-/lodash-4.17.21.tgz")
	if err != nil {
		t.Fatalf("tarball request failed: %v", err)
	}
	defer tarballResp.Body.Close()
	data, _ := io.ReadAll(tarballResp.Body)
	if !bytes.Equal(data, []byte("tarball-bytes")) {
		t.Fatalf("tarball payload mismatch: %s", string(data))
	}

	if got := len(stub.Requests()); got != 2 {
		t.Fatalf("expected 2 recorded requests, got %d", got)
	}
}

func TestUpstreamStubSupportsAnonymousCurlHostHeader(t *testing.T) {
	stub := newUpstreamStub(t, upstreamDocker)
	defer stub.Close()

	req, err := http.NewRequest(http.MethodGet, stub.URL+"/v2/", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	req.Host = "docker.hub.local"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("curl-style request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 from curl-style request, got %d body=%s", resp.StatusCode, string(body))
	}

	if got := stub.Requests(); len(got) != 1 || got[0].Host != "docker.hub.local" {
		t.Fatalf("expected recorded host docker.hub.local, got %v", got)
	}
}
