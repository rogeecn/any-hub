# registry.k8s.io Compatibility Fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `registry.k8s.io`-only manifest fallback so single-segment repos like `coredns` retry once as `coredns/coredns` after a final first-attempt `404`.

**Architecture:** Keep the first upstream request unchanged, then add one targeted retry in the proxy after the normal auth flow completes. Persist the effective upstream path for fallback-filled cache entries so later cache revalidation uses the path that actually produced the content instead of rechecking the original `404` path.

**Tech Stack:** Go 1.25, Fiber v3, internal docker hooks, internal proxy handler, filesystem cache store, Go tests

---

## File map

- Modify: `internal/hubmodule/docker/hooks.go`
  - add `registry.k8s.io` host detection and manifest fallback path derivation helpers
- Modify: `internal/hubmodule/docker/hooks_test.go`
  - add unit tests for host gating and manifest fallback derivation rules
- Modify: `internal/cache/store.go`
  - extend cache entry/options to carry optional effective upstream path metadata
- Modify: `internal/cache/fs_store.go`
  - persist and load cache metadata for effective upstream path
- Modify: `internal/cache/store_test.go`
  - verify metadata round-trip behavior and remove behavior
- Modify: `internal/proxy/handler.go`
- Modify: `internal/proxy/handler_test.go`
  - add one-shot `registry.k8s.io` fallback retry after auth retry flow
  - write/read effective upstream path metadata for fallback-backed cache entries
  - use effective upstream path during revalidation
- Modify: `tests/integration/cache_flow_test.go`
  - add integration coverage for fallback success, no-fallback success, and cache/revalidation behavior

### Task 1: Add docker fallback derivation helpers

**Files:**
- Modify: `internal/hubmodule/docker/hooks.go`
- Test: `internal/hubmodule/docker/hooks_test.go`

- [ ] **Step 1: Write the failing unit tests**

```go
func TestIsRegistryK8sHost(t *testing.T) {
	if !isRegistryK8sHost("registry.k8s.io") {
		t.Fatalf("expected registry.k8s.io to match")
	}
	if isRegistryK8sHost("example.com") {
		t.Fatalf("expected non-registry.k8s.io host to be ignored")
	}
}

func TestRegistryK8sManifestFallbackPath(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	path, ok := manifestFallbackPath(ctx, "/v2/coredns/manifests/v1.13.1")
	if !ok || path != "/v2/coredns/coredns/manifests/v1.13.1" {
		t.Fatalf("expected fallback path, got %q ok=%v", path, ok)
	}
}

func TestRegistryK8sManifestFallbackPathRejectsMultiSegmentRepo(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	if _, ok := manifestFallbackPath(ctx, "/v2/coredns/coredns/manifests/v1.13.1"); ok {
		t.Fatalf("expected multi-segment repo to be ignored")
	}
}

func TestRegistryK8sManifestFallbackPathRejectsNonManifest(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "registry.k8s.io"}
	if _, ok := manifestFallbackPath(ctx, "/v2/coredns/blobs/sha256:deadbeef"); ok {
		t.Fatalf("expected non-manifest path to be ignored")
	}
}

func TestRegistryK8sManifestFallbackPathRejectsNonRegistryHost(t *testing.T) {
	ctx := &hooks.RequestContext{UpstreamHost: "mirror.gcr.io"}
	if _, ok := manifestFallbackPath(ctx, "/v2/coredns/manifests/v1.13.1"); ok {
		t.Fatalf("expected non-registry.k8s.io host to be ignored")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hubmodule/docker -run 'TestIsRegistryK8sHost|TestRegistryK8sManifestFallbackPath' -count=1`
Expected: FAIL because `isRegistryK8sHost` and `manifestFallbackPath` do not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
func isRegistryK8sHost(host string) bool {
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return strings.EqualFold(host, "registry.k8s.io")
}

func manifestFallbackPath(ctx *hooks.RequestContext, clean string) (string, bool) {
	if ctx == nil || !isRegistryK8sHost(ctx.UpstreamHost) {
		return "", false
	}
	repo, rest, ok := splitDockerRepoPath(clean)
	if !ok || strings.Count(repo, "/") != 0 || !strings.HasPrefix(rest, "/manifests/") {
		return "", false
	}
	return "/v2/" + repo + "/" + repo + rest, true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/hubmodule/docker -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hubmodule/docker/hooks.go internal/hubmodule/docker/hooks_test.go
git commit -m "test: cover registry k8s fallback path derivation"
```

### Task 2: Persist effective upstream path in cache metadata

**Files:**
- Modify: `internal/cache/store.go`
- Modify: `internal/cache/fs_store.go`
- Test: `internal/cache/store_test.go`

- [ ] **Step 1: Write the failing cache metadata test**

```go
func TestStorePersistsEffectiveUpstreamPath(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	loc := Locator{HubName: "docker", Path: "/v2/coredns/manifests/v1.13.1"}
	_, err = store.Put(context.Background(), loc, strings.NewReader("body"), PutOptions{
		EffectiveUpstreamPath: "/v2/coredns/coredns/manifests/v1.13.1",
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := store.Get(context.Background(), loc)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Entry.EffectiveUpstreamPath != "/v2/coredns/coredns/manifests/v1.13.1" {
		t.Fatalf("unexpected effective path: %q", got.Entry.EffectiveUpstreamPath)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cache -run TestStorePersistsEffectiveUpstreamPath -count=1`
Expected: FAIL because cache entry metadata does not yet include effective upstream path.

- [ ] **Step 3: Write minimal implementation**

```go
type PutOptions struct {
	ModTime time.Time
	EffectiveUpstreamPath string
}

type Entry struct {
	Locator Locator `json:"locator"`
	FilePath string `json:"file_path"`
	SizeBytes int64 `json:"size_bytes"`
	ModTime time.Time `json:"mod_time"`
	EffectiveUpstreamPath string `json:"effective_upstream_path,omitempty"`
}
```

Implementation notes:
- store metadata next to the cached body as a small JSON file such as `<entry>.meta`
- on `Get`, load metadata if present and populate `Entry.EffectiveUpstreamPath`
- on `Put`, write metadata atomically only when `EffectiveUpstreamPath` is non-empty
- on `Remove`, delete both body and metadata files

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cache -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cache/store.go internal/cache/fs_store.go internal/cache/store_test.go
git commit -m "feat: persist cache effective upstream path"
```

### Task 3: Add one-shot fallback retry in proxy fetch flow

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `internal/hubmodule/docker/hooks.go`
- Test: `tests/integration/cache_flow_test.go`

- [ ] **Step 1: Write the failing integration test for fallback success**

```go
func TestRegistryK8sManifestFallbackRetry(t *testing.T) {
	// stub returns 404 for /v2/coredns/manifests/v1.13.1
	// stub returns 200 for /v2/coredns/coredns/manifests/v1.13.1
	// request /v2/coredns/manifests/v1.13.1 through proxy
	// expect 200 and exactly two upstream hits
}

func TestRegistryK8sManifestFallbackNotAttemptedWhenOriginalSucceeds(t *testing.T) {
	// stub returns 200 for original path
	// request original path
	// expect 200 and only one upstream hit
}

func TestRegistryK8sManifestFallbackNotAttemptedForNonRegistryHost(t *testing.T) {
	// non-registry.k8s.io upstream returns 404 for original path
	// request original path
	// expect final 404 and no derived path hit
}

func TestRegistryK8sManifestFallbackSecondRequestHitsCache(t *testing.T) {
	// first GET falls back and succeeds
	// second identical GET should be cache hit
	// expect no new hit to original 404 path on second request
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/integration -run 'TestRegistryK8sManifestFallbackRetry|TestRegistryK8sManifestFallbackNotAttemptedWhenOriginalSucceeds|TestRegistryK8sManifestFallbackNotAttemptedForNonRegistryHost|TestRegistryK8sManifestFallbackSecondRequestHitsCache' -count=1`
Expected: FAIL because the fallback tests were written before the retry logic exists.

- [ ] **Step 3: Write minimal implementation**

```go
resp, upstreamURL, err := h.executeRequest(c, route, hook)
resp, upstreamURL, err = h.retryOnAuthFailure(c, route, requestID, started, resp, upstreamURL, hook)

if resp.StatusCode == http.StatusNotFound {
	if fallbackHook, fallbackURL, ok := h.registryK8sFallbackAttempt(c, route, hook); ok {
		resp.Body.Close()
		resp, upstreamURL, err = h.executeRequest(c, route, fallbackHook)
		if err == nil {
			resp, upstreamURL, err = h.retryOnAuthFailure(c, route, requestID, started, resp, upstreamURL, fallbackHook)
		}
		fallbackEffectivePath = fallbackURL.Path
	}
}
```

Implementation notes:
- in `internal/hubmodule/docker/hooks.go`, keep only pure path helpers such as `isRegistryK8sHost` and `manifestFallbackPath`
- in `internal/proxy/handler.go`, add the retry orchestration helper that clones the current hook state with the derived fallback path when `manifestFallbackPath` returns true
- only evaluate fallback after `retryOnAuthFailure` completes and only for final `404`
- close the first response body before the retry
- if fallback succeeds with `200`, pass `EffectiveUpstreamPath` into cache `PutOptions`
- if fallback returns non-`200`, pass that second upstream response through unchanged
- emit a structured fallback log event from `internal/proxy/handler.go` with hub name, domain, upstream host, original path, fallback path, original status, and request method

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests/integration -run 'TestRegistryK8sManifestFallbackRetry|TestRegistryK8sManifestFallbackNotAttemptedWhenOriginalSucceeds|TestRegistryK8sManifestFallbackNotAttemptedForNonRegistryHost|TestRegistryK8sManifestFallbackSecondRequestHitsCache' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/handler.go tests/integration/cache_flow_test.go internal/hubmodule/docker/hooks.go
git commit -m "feat: retry registry k8s manifest fallback"
```

### Task 4: Revalidate cached fallback entries against the effective upstream path

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `tests/integration/cache_flow_test.go`

- [ ] **Step 1: Write the failing revalidation regression test**

```go
func TestRegistryK8sFallbackCacheRevalidatesEffectivePath(t *testing.T) {
	// first GET: original path 404, fallback path 200, response cached
	// second GET: cache hit path should revalidate against fallback path only
	// assert original 404 path is not re-requested during revalidation
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/integration -run TestRegistryK8sFallbackCacheRevalidatesEffectivePath -count=1`
Expected: FAIL because revalidation still targets the original client-facing path.

- [ ] **Step 3: Write minimal implementation**

```go
func effectiveRevalidateURL(route *server.HubRoute, c fiber.Ctx, entry cache.Entry, hook *hookState) *url.URL {
	if entry.EffectiveUpstreamPath == "" {
		return resolveUpstreamURL(route, route.UpstreamURL, c, hook)
	}
	clone := *route.UpstreamURL
	clone.Path = entry.EffectiveUpstreamPath
	clone.RawPath = entry.EffectiveUpstreamPath
	return &clone
}
```

Implementation notes:
- use `Entry.EffectiveUpstreamPath` in both `isCacheFresh` and cached `HEAD` handling inside `serveCache`
- do not re-derive fallback during revalidation when metadata already specifies the effective path
- keep client-visible cache key unchanged

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./tests/integration -run TestRegistryK8sFallbackCacheRevalidatesEffectivePath -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/handler.go tests/integration/cache_flow_test.go
git commit -m "fix: revalidate registry k8s fallback cache entries"
```

### Task 5: Emit and verify fallback structured logging

**Files:**
- Modify: `internal/proxy/handler.go`
- Test: `internal/proxy/handler_test.go`

- [ ] **Step 1: Write the failing logging test**

```go
func TestRegistryK8sManifestFallbackLogsStructuredEvent(t *testing.T) {
	// configure handler with a bytes.Buffer-backed logrus logger
	// trigger fallback success for /v2/coredns/manifests/v1.13.1
	// assert log output contains event name plus fields:
	// hub, domain, upstream host, original path, fallback path, original status, method
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy -run TestRegistryK8sManifestFallbackLogsStructuredEvent -count=1`
Expected: FAIL because the fallback log event does not exist yet.

- [ ] **Step 3: Write minimal implementation**

```go
func (h *Handler) logRegistryK8sFallback(route *server.HubRoute, requestID string, originalPath string, fallbackPath string, originalStatus int, method string) {
	fields := logging.RequestFields(
		route.Config.Name,
		route.Config.Domain,
		route.Config.Type,
		route.Config.AuthMode(),
		route.Module.Key,
		false,
	)
	fields["action"] = "proxy_fallback"
	fields["upstream_host"] = route.UpstreamURL.Host
	fields["original_path"] = originalPath
	fields["fallback_path"] = fallbackPath
	fields["original_status"] = originalStatus
	fields["method"] = method
	h.logger.WithFields(fields).Info("proxy_registry_k8s_fallback")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/proxy -run TestRegistryK8sManifestFallbackLogsStructuredEvent -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/handler.go internal/proxy/handler_test.go
git commit -m "test: cover registry k8s fallback logging"
```

### Task 6: Final verification

**Files:**
- Verify: `internal/hubmodule/docker/hooks.go`
- Verify: `internal/cache/store.go`
- Verify: `internal/cache/fs_store.go`
- Verify: `internal/proxy/handler.go`
- Verify: `tests/integration/cache_flow_test.go`

- [ ] **Step 1: Run focused package tests**

Run: `go test ./internal/hubmodule/docker ./internal/cache ./tests/integration -count=1`
Expected: PASS

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Inspect git diff**

Run: `git diff --stat`
Expected: only the planned files changed for this feature.

- [ ] **Step 4: Verify fallback logging is present**

Run: `go test ./internal/proxy -run TestRegistryK8sManifestFallbackLogsStructuredEvent -count=1`
Expected: PASS, and the test explicitly checks the structured fallback log event fields.

- [ ] **Step 5: Commit final polish if needed**

```bash
git add internal/hubmodule/docker/hooks.go internal/hubmodule/docker/hooks_test.go internal/cache/store.go internal/cache/fs_store.go internal/cache/store_test.go internal/proxy/handler.go internal/proxy/handler_test.go tests/integration/cache_flow_test.go
git commit -m "feat: add registry k8s manifest fallback compatibility"
```
