package proxy

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/logging"
	"github.com/any-hub/any-hub/internal/proxy/hooks"
	"github.com/any-hub/any-hub/internal/server"
)

// Handler 负责 orchestrate “缓存命中 → revalidate → 回源写缓存” 的全流程，
// 对外暴露 Fiber handler，内部复用共享 http.Client 与磁盘缓存。
type Handler struct {
	client *http.Client
	logger *logrus.Logger
	store  cache.Store
	etags  sync.Map // key: hub+path, value: etag/digest string
}

type hookState struct {
	ctx      *hooks.RequestContext
	def      hooks.Hooks
	hasHooks bool
	clean    string
	rawQuery []byte
}

// NewHandler constructs a proxy handler with shared HTTP client/logger/store.
func NewHandler(client *http.Client, logger *logrus.Logger, store cache.Store) *Handler {
	return &Handler{
		client: client,
		logger: logger,
		store:  store,
	}
}

func buildHookContext(route *server.HubRoute, c fiber.Ctx) *hooks.RequestContext {
	if route == nil {
		return &hooks.RequestContext{Method: c.Method()}
	}
	baseHost := ""
	if route.UpstreamURL != nil {
		baseHost = route.UpstreamURL.Host
	}
	return &hooks.RequestContext{
		HubName:      route.Config.Name,
		Domain:       route.Config.Domain,
		HubType:      route.Config.Type,
		ModuleKey:    route.ModuleKey,
		RolloutFlag:  string(route.RolloutFlag),
		UpstreamHost: baseHost,
		Method:       c.Method(),
	}
}

func hasHook(def hooks.Hooks) bool {
	return def.NormalizePath != nil ||
		def.ResolveUpstream != nil ||
		def.RewriteResponse != nil ||
		def.CachePolicy != nil ||
		def.ContentType != nil
}

// Handle 执行缓存查找、条件回源和最终 streaming 逻辑，任何阶段出错都会输出结构化日志。
func (h *Handler) Handle(c fiber.Ctx, route *server.HubRoute) error {
	started := time.Now()
	requestID := server.RequestID(c)
	hooksDef, ok := hooks.Fetch(route.ModuleKey)
	hookCtx := buildHookContext(route, c)
	rawQuery := append([]byte(nil), c.Request().URI().QueryString()...)
	cleanPath := normalizeRequestPath(route, string(c.Request().URI().Path()))
	if hasHook(hooksDef) && hooksDef.NormalizePath != nil {
		newPath, newQuery := hooksDef.NormalizePath(hookCtx, cleanPath, rawQuery)
		if newPath != "" {
			cleanPath = newPath
		}
		rawQuery = newQuery
	}
	locator := buildLocator(route, c, cleanPath, rawQuery)
	policy := determineCachePolicyWithHook(route, locator, c.Method(), hooksDef, ok, hookCtx)
	hookState := hookState{
		ctx:      hookCtx,
		def:      hooksDef,
		hasHooks: ok && hasHook(hooksDef),
		clean:    cleanPath,
		rawQuery: rawQuery,
	}
	strategyWriter := cache.NewStrategyWriter(h.store, route.CacheStrategy)

	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var cached *cache.ReadResult
	if strategyWriter.Enabled() && policy.allowCache {
		result, err := h.store.Get(ctx, locator)
		switch {
		case err == nil:
			cached = result
		case errors.Is(err, cache.ErrNotFound):
			// miss, continue
		default:
			h.logger.WithError(err).
				WithFields(logrus.Fields{"hub": route.Config.Name, "module_key": route.ModuleKey}).
				Warn("cache_get_failed")
		}
	}

	if cached != nil {
		serve := true
		if policy.requireRevalidate {
			if strategyWriter.ShouldBypassValidation(cached.Entry) {
				serve = true
			} else if strategyWriter.SupportsValidation() {
				fresh, err := h.isCacheFresh(c, route, locator, cached.Entry, &hookState)
				if err != nil {
					h.logger.WithError(err).
						WithFields(logrus.Fields{"hub": route.Config.Name, "module_key": route.ModuleKey}).
						Warn("cache_revalidate_failed")
					serve = false
				} else if !fresh {
					serve = false
				}
			} else {
				serve = false
			}
		}
		if serve {
			defer cached.Reader.Close()
			return h.serveCache(c, route, cached, requestID, started, &hookState)
		}
		cached.Reader.Close()
	}

	return h.fetchAndStream(c, route, locator, policy, strategyWriter, requestID, started, ctx, &hookState)
}

func (h *Handler) serveCache(
	c fiber.Ctx,
	route *server.HubRoute,
	result *cache.ReadResult,
	requestID string,
	started time.Time,
	hook *hookState,
) error {
	var readSeeker io.ReadSeeker
	switch reader := result.Reader.(type) {
	case io.ReadSeeker:
		readSeeker = reader
		_, _ = readSeeker.Seek(0, io.SeekStart)
	case io.Seeker:
		_, _ = reader.Seek(0, io.SeekStart)
	}

	method := c.Method()

	contentType := resolveContentType(route, result.Entry.Locator, hook)
	if contentType == "" && shouldSniffDockerManifest(result.Entry.Locator) {
		if sniffed := sniffDockerManifestContentType(readSeeker); sniffed != "" {
			contentType = sniffed
		}
	}
	if contentType != "" {
		c.Set("Content-Type", contentType)
	} else {
		c.Response().Header.Del("Content-Type")
	}

	length := result.Entry.SizeBytes
	if length > 0 {
		c.Response().Header.SetContentLength(int(length))
	} else {
		c.Response().Header.Del("Content-Length")
	}

	c.Set("X-Any-Hub-Upstream", route.UpstreamURL.String())
	c.Set("X-Any-Hub-Cache-Hit", "true")
	if requestID != "" {
		c.Set("X-Request-ID", requestID)
	}

	status := fiber.StatusOK
	c.Status(status)

	if method == http.MethodHead {
		result.Reader.Close()
		h.logResult(route, route.UpstreamURL.String(), requestID, status, true, started, nil)
		return nil
	}

	_, err := io.Copy(c.Response().BodyWriter(), result.Reader)
	result.Reader.Close()
	h.logResult(route, route.UpstreamURL.String(), requestID, status, true, started, err)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, fmt.Sprintf("read cache failed: %v", err))
	}
	return nil
}

func (h *Handler) fetchAndStream(
	c fiber.Ctx,
	route *server.HubRoute,
	locator cache.Locator,
	policy cachePolicy,
	writer cache.StrategyWriter,
	requestID string,
	started time.Time,
	ctx context.Context,
	hook *hookState,
) error {
	resp, upstreamURL, err := h.executeRequest(c, route, hook)
	if err != nil {
		h.logResult(route, upstreamURL.String(), requestID, 0, false, started, err)
		return h.writeError(c, fiber.StatusBadGateway, "upstream_failed")
	}

	resp, upstreamURL, err = h.retryOnAuthFailure(c, route, requestID, started, resp, upstreamURL, hook)
	if err != nil {
		h.logResult(route, upstreamURL.String(), requestID, 0, false, started, err)
		return h.writeError(c, fiber.StatusBadGateway, "upstream_failed")
	}
	if hook != nil && hook.hasHooks && hook.def.RewriteResponse != nil {
		if rewritten, rewriteErr := applyHookRewrite(hook, resp, requestPath(c)); rewriteErr == nil {
			resp = rewritten
		} else {
			h.logger.WithError(rewriteErr).WithFields(logrus.Fields{
				"action": "hook_rewrite",
				"hub":    route.Config.Name,
			}).Warn("hook_rewrite_failed")
		}
	}
	defer resp.Body.Close()

	shouldStore := policy.allowStore && writer.Enabled() && isCacheableStatus(resp.StatusCode) &&
		c.Method() == http.MethodGet
	return h.consumeUpstream(c, route, locator, resp, shouldStore, writer, requestID, started, ctx)
}

func applyHookRewrite(hook *hookState, resp *http.Response, path string) (*http.Response, error) {
	if hook == nil || hook.def.RewriteResponse == nil {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp, err
	}
	headers := make(map[string]string, len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	status, newHeaders, newBody, rewriteErr := hook.def.RewriteResponse(hook.ctx, resp.StatusCode, headers, body, path)
	if rewriteErr != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		return resp, rewriteErr
	}
	if newHeaders == nil {
		newHeaders = headers
	}
	if newBody == nil {
		newBody = body
	}
	cloned := *resp
	cloned.StatusCode = status
	cloned.Header = make(http.Header, len(newHeaders))
	for key, value := range newHeaders {
		cloned.Header.Set(key, value)
	}
	cloned.Body = io.NopCloser(bytes.NewReader(newBody))
	cloned.ContentLength = int64(len(newBody))
	return &cloned, nil
}

func (h *Handler) consumeUpstream(
	c fiber.Ctx,
	route *server.HubRoute,
	locator cache.Locator,
	resp *http.Response,
	shouldStore bool,
	writer cache.StrategyWriter,
	requestID string,
	started time.Time,
	ctx context.Context,
) error {
	upstreamURL := resp.Request.URL.String()
	method := c.Method()
	authFailure := isAuthFailure(resp.StatusCode) && route.Config.HasCredentials()

	if shouldStore {
		return h.cacheAndStream(c, route, locator, resp, writer, requestID, started, ctx, upstreamURL)
	}

	copyResponseHeaders(c, resp.Header)
	c.Set("X-Any-Hub-Upstream", upstreamURL)
	c.Set("X-Any-Hub-Cache-Hit", "false")
	if requestID != "" {
		c.Set("X-Request-ID", requestID)
	}
	c.Status(resp.StatusCode)

	if authFailure {
		h.logAuthFailure(route, upstreamURL, requestID, resp.StatusCode)
	}

	if method == http.MethodHead {
		h.logResult(route, upstreamURL, requestID, resp.StatusCode, false, started, nil)
		return nil
	}

	_, err := io.Copy(c.Response().BodyWriter(), resp.Body)
	h.logResult(route, upstreamURL, requestID, resp.StatusCode, false, started, err)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, fmt.Sprintf("proxy stream failed: %v", err))
	}
	return nil
}

func (h *Handler) cacheAndStream(
	c fiber.Ctx,
	route *server.HubRoute,
	locator cache.Locator,
	resp *http.Response,
	writer cache.StrategyWriter,
	requestID string,
	started time.Time,
	ctx context.Context,
	upstreamURL string,
) error {
	copyResponseHeaders(c, resp.Header)
	c.Set("X-Any-Hub-Upstream", upstreamURL)
	c.Set("X-Any-Hub-Cache-Hit", "false")
	if requestID != "" {
		c.Set("X-Request-ID", requestID)
	}
	c.Status(resp.StatusCode)

	reader := io.TeeReader(resp.Body, c.Response().BodyWriter())

	opts := cache.PutOptions{ModTime: extractModTime(resp.Header)}
	entry, err := writer.Put(ctx, locator, reader, opts)
	h.logResult(route, upstreamURL, requestID, resp.StatusCode, false, started, err)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, fmt.Sprintf("cache_write_failed: %v", err))
	}
	h.rememberETag(route, locator, resp)
	_ = entry
	return nil
}

func (h *Handler) retryOnAuthFailure(
	c fiber.Ctx,
	route *server.HubRoute,
	requestID string,
	started time.Time,
	resp *http.Response,
	upstreamURL *url.URL,
	hook *hookState,
) (*http.Response, *url.URL, error) {
	if !shouldRetryAuth(route, resp.StatusCode) {
		return resp, upstreamURL, nil
	}

	challenge, ok := parseBearerChallenge(resp.Header.Values("Www-Authenticate"))
	h.logAuthRetry(route, upstreamURL.String(), requestID, resp.StatusCode)
	resp.Body.Close()

	if ok {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		token, err := h.fetchBearerToken(ctx, challenge, route)
		if err != nil {
			return nil, upstreamURL, err
		}
		authHeader := "Bearer " + token
		retryResp, retryURL, err := h.executeRequestWithAuth(c, route, hook, authHeader)
		if err != nil {
			return nil, upstreamURL, err
		}
		return retryResp, retryURL, nil
	}

	retryResp, retryURL, err := h.executeRequest(c, route, hook)
	if err != nil {
		return nil, upstreamURL, err
	}
	return retryResp, retryURL, nil
}

func (h *Handler) executeRequest(c fiber.Ctx, route *server.HubRoute, hook *hookState) (*http.Response, *url.URL, error) {
	return h.executeRequestWithAuth(c, route, hook, "")
}

func (h *Handler) executeRequestWithAuth(
	c fiber.Ctx,
	route *server.HubRoute,
	hook *hookState,
	authHeader string,
) (*http.Response, *url.URL, error) {
	upstreamURL := resolveUpstreamURL(route, route.UpstreamURL, c, hook)
	body := bytesReader(c.Body())
	req, err := h.buildUpstreamRequest(c, upstreamURL, route, c.Method(), body, authHeader)
	if err != nil {
		return nil, upstreamURL, err
	}

	resp, err := h.doRequest(req, route)
	return resp, upstreamURL, err
}

func (h *Handler) buildUpstreamRequest(
	c fiber.Ctx,
	upstream *url.URL,
	route *server.HubRoute,
	method string,
	body io.Reader,
	overrideAuth string,
) (*http.Request, error) {
	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if body == nil {
		body = http.NoBody
	}

	req, err := http.NewRequestWithContext(ctx, method, upstream.String(), body)
	if err != nil {
		return nil, err
	}

	requestHeaders := fiberHeadersAsHTTP(c)
	server.CopyHeaders(req.Header, requestHeaders)
	req.Header.Del("Accept-Encoding")
	req.Host = upstream.Host
	req.Header.Set("Host", upstream.Host)
	req.Header.Set("X-Forwarded-Host", c.Hostname())
	if ip := c.IP(); ip != "" {
		if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
			req.Header.Set("X-Forwarded-For", prior+", "+ip)
		} else {
			req.Header.Set("X-Forwarded-For", ip)
		}
	}
	req.Header.Set("X-Forwarded-Proto", c.Protocol())
	req.Header.Set("X-Forwarded-Port", routePort(route))

	if overrideAuth != "" {
		req.Header.Set("Authorization", overrideAuth)
	} else if authHeader := buildCredentialHeader(route.Config.Username, route.Config.Password); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	return req, nil
}

func (h *Handler) doRequest(req *http.Request, route *server.HubRoute) (*http.Response, error) {
	if route.ProxyURL == nil {
		return h.client.Do(req)
	}
	transport := http.Transport{}
	if base, ok := h.client.Transport.(*http.Transport); ok && base != nil {
		transport = *base.Clone()
	}
	transport.Proxy = http.ProxyURL(route.ProxyURL)
	client := *h.client
	client.Transport = &transport
	return client.Do(req)
}

func (h *Handler) writeError(c fiber.Ctx, status int, code string) error {
	return c.Status(status).JSON(fiber.Map{"error": code})
}

func (h *Handler) logResult(
	route *server.HubRoute,
	upstream string,
	requestID string,
	status int,
	cacheHit bool,
	started time.Time,
	err error,
) {
	fields := logging.RequestFields(
		route.Config.Name,
		route.Config.Domain,
		route.Config.Type,
		route.Config.AuthMode(),
		route.ModuleKey,
		string(route.RolloutFlag),
		cacheHit,
		route.ModuleKey == hubmodule.DefaultModuleKey(),
	)
	fields["action"] = "proxy"
	fields["upstream"] = upstream
	fields["upstream_status"] = status
	fields["elapsed_ms"] = time.Since(started).Milliseconds()
	if requestID != "" {
		fields["request_id"] = requestID
	}
	if err != nil {
		fields["error"] = err.Error()
		h.logger.WithFields(fields).Error("proxy_failed")
		return
	}
	h.logger.WithFields(fields).Info("proxy_complete")
}

func inferCachedContentType(route *server.HubRoute, locator cache.Locator) string {
	clean := stripQueryMarker(locator.Path)
	switch {
	case strings.HasSuffix(clean, ".zip"):
		return "application/zip"
	case strings.HasSuffix(clean, ".json"):
		return "application/json"
	case strings.HasSuffix(clean, ".mod"):
		return "text/plain"
	case strings.HasSuffix(clean, ".info"):
		return "application/json"
	case strings.HasSuffix(clean, ".tgz"):
		return "application/octet-stream"
	case strings.HasSuffix(clean, "/@v/list"):
		return "text/plain"
	case strings.HasSuffix(clean, ".whl"):
		return "application/octet-stream"
	case strings.HasSuffix(clean, ".tar.gz"), strings.HasSuffix(clean, ".tar.bz2"):
		return "application/x-tar"
	}

	return ""
}

func resolveContentType(route *server.HubRoute, locator cache.Locator, hook *hookState) string {
	if hook != nil && hook.hasHooks && hook.def.ContentType != nil {
		if ct := hook.def.ContentType(hook.ctx, stripQueryMarker(locator.Path)); ct != "" {
			return ct
		}
	}
	return inferCachedContentType(route, locator)
}

func buildLocator(route *server.HubRoute, c fiber.Ctx, clean string, rawQuery []byte) cache.Locator {
	query := rawQuery
	if len(query) > 0 {
		sum := sha1.Sum(query)
		clean = fmt.Sprintf("%s/__qs/%s", clean, hex.EncodeToString(sum[:]))
	}
	loc := cache.Locator{
		HubName: route.Config.Name,
		Path:    clean,
	}
	rewrite := route.Module.LocatorRewrite
	if rewrite == nil {
		rewrite = hubmodule.DefaultLocatorRewrite(route.Config.Type)
	}
	if rewrite != nil {
		rewritten := rewrite(hubmodule.Locator{
			HubName: loc.HubName,
			Path:    loc.Path,
			HubType: route.Config.Type,
		})
		loc = cache.Locator{
			HubName: rewritten.HubName,
			Path:    rewritten.Path,
		}
	}
	return loc
}

func stripQueryMarker(p string) string {
	if idx := strings.Index(p, "/__qs/"); idx >= 0 {
		return p[:idx]
	}
	return p
}

func shouldSniffDockerManifest(locator cache.Locator) bool {
	clean := stripQueryMarker(locator.Path)
	return strings.Contains(clean, "/manifests/")
}

func sniffDockerManifestContentType(reader io.ReadSeeker) string {
	if reader == nil {
		return ""
	}
	const maxInspectBytes = 512 * 1024
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxInspectBytes))
	if _, seekErr := reader.Seek(0, io.SeekStart); seekErr != nil {
		return ""
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return ""
	}
	var manifest struct {
		MediaType string `json:"mediaType"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}
	return strings.TrimSpace(manifest.MediaType)
}

func requestPath(c fiber.Ctx) string {
	if c == nil {
		return "/"
	}
	uri := c.Request().URI()
	if uri == nil {
		return "/"
	}
	pathVal := string(uri.Path())
	if pathVal == "" {
		return "/"
	}
	return pathVal
}

func normalizeRequestPath(route *server.HubRoute, raw string) string {
	if raw == "" {
		raw = "/"
	}
	clean := path.Clean("/" + raw)
	return clean
}

func bytesReader(b []byte) io.Reader {
	if len(b) == 0 {
		return http.NoBody
	}
	return bytes.NewReader(b)
}

func resolveUpstreamURL(route *server.HubRoute, base *url.URL, c fiber.Ctx, hook *hookState) *url.URL {
	uri := c.Request().URI()
	rawQuery := append([]byte(nil), uri.QueryString()...)
	clean := normalizeRequestPath(route, string(uri.Path()))
	if hook != nil {
		if hook.clean != "" {
			clean = hook.clean
		}
		if hook.rawQuery != nil {
			rawQuery = hook.rawQuery
		}
		if hook.hasHooks && hook.def.ResolveUpstream != nil {
			if u := hook.def.ResolveUpstream(hook.ctx, base.String(), clean, rawQuery); u != "" {
				if parsed, err := url.Parse(u); err == nil {
					return parsed
				}
			}
		}
	}
	relative := &url.URL{Path: clean, RawPath: clean}
	if len(rawQuery) > 0 {
		relative.RawQuery = string(rawQuery)
	}
	return base.ResolveReference(relative)
}

func fiberHeadersAsHTTP(c fiber.Ctx) http.Header {
	header := http.Header{}
	c.Request().Header.VisitAll(func(key, value []byte) {
		header.Add(string(key), string(value))
	})
	return header
}

func copyResponseHeaders(c fiber.Ctx, headers http.Header) {
	for key, values := range headers {
		if server.IsHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			c.Set(key, value)
		}
	}
}

func routePort(route *server.HubRoute) string {
	if route == nil || route.ListenPort <= 0 {
		return "0"
	}
	return fmt.Sprintf("%d", route.ListenPort)
}

type cachePolicy struct {
	allowCache        bool
	allowStore        bool
	requireRevalidate bool
}

func determineCachePolicyWithHook(route *server.HubRoute, locator cache.Locator, method string, def hooks.Hooks, enabled bool, ctx *hooks.RequestContext) cachePolicy {
	base := determineCachePolicy(route, locator, method)
	if !enabled || def.CachePolicy == nil {
		return base
	}
	updated := def.CachePolicy(ctx, locator.Path, hooks.CachePolicy{
		AllowCache:        base.allowCache,
		AllowStore:        base.allowStore,
		RequireRevalidate: base.requireRevalidate,
	})
	base.allowCache = updated.AllowCache
	base.allowStore = updated.AllowStore
	base.requireRevalidate = updated.RequireRevalidate
	return base
}

func determineCachePolicy(route *server.HubRoute, locator cache.Locator, method string) cachePolicy {
	if method != http.MethodGet {
		return cachePolicy{}
	}
	return cachePolicy{allowCache: true, allowStore: true}
}

func isCacheableStatus(status int) bool {
	return status == http.StatusOK
}

func (h *Handler) isCacheFresh(
	c fiber.Ctx,
	route *server.HubRoute,
	locator cache.Locator,
	entry cache.Entry,
	hook *hookState,
) (bool, error) {
	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	upstreamURL := resolveUpstreamURL(route, route.UpstreamURL, c, hook)
	resp, err := h.revalidateRequest(c, route, upstreamURL, locator, "")
	if err != nil {
		return false, err
	}

	if shouldRetryAuth(route, resp.StatusCode) {
		challenge, ok := parseBearerChallenge(resp.Header.Values("Www-Authenticate"))
		resp.Body.Close()

		authHeader := ""
		if ok {
			token, err := h.fetchBearerToken(ctx, challenge, route)
			if err != nil {
				return false, err
			}
			authHeader = "Bearer " + token
		}

		resp, err = h.revalidateRequest(c, route, upstreamURL, locator, authHeader)
		if err != nil {
			return false, err
		}
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return true, nil
	case http.StatusOK:
		h.rememberETag(route, locator, resp)
		remote := extractModTime(resp.Header)
		if !remote.After(entry.ModTime.Add(time.Second)) {
			return true, nil
		}
		return false, nil
	case http.StatusNotFound:
		if h.store != nil {
			_ = h.store.Remove(ctx, locator)
		}
		h.forgetETag(route, locator)
		return false, nil
	default:
		return false, nil
	}
}

func (h *Handler) revalidateRequest(
	c fiber.Ctx,
	route *server.HubRoute,
	upstreamURL *url.URL,
	locator cache.Locator,
	overrideAuth string,
) (*http.Response, error) {
	req, err := h.buildUpstreamRequest(c, upstreamURL, route, http.MethodHead, http.NoBody, overrideAuth)
	if err != nil {
		return nil, err
	}
	if etag := h.cachedETag(route, locator); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	return h.doRequest(req, route)
}

func extractModTime(header http.Header) time.Time {
	if last := header.Get("Last-Modified"); last != "" {
		if parsed, err := http.ParseTime(last); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

type bearerChallenge struct {
	Realm   string
	Service string
	Scope   string
}

func parseBearerChallenge(values []string) (bearerChallenge, bool) {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			continue
		}
		params := parseAuthParams(raw[len("Bearer "):])
		challenge := bearerChallenge{
			Realm:   params["realm"],
			Service: params["service"],
			Scope:   params["scope"],
		}
		if challenge.Realm == "" {
			continue
		}
		return challenge, true
	}
	return bearerChallenge{}, false
}

func parseAuthParams(input string) map[string]string {
	params := make(map[string]string)
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		params[key] = value
	}
	return params
}

func (h *Handler) fetchBearerToken(
	ctx context.Context,
	challenge bearerChallenge,
	route *server.HubRoute,
) (string, error) {
	if challenge.Realm == "" {
		return "", errors.New("bearer realm missing")
	}
	tokenURL, err := url.Parse(challenge.Realm)
	if err != nil {
		return "", fmt.Errorf("invalid bearer realm: %w", err)
	}
	query := tokenURL.Query()
	if challenge.Service != "" {
		query.Set("service", challenge.Service)
	}
	if challenge.Scope != "" {
		query.Set("scope", challenge.Scope)
	}
	tokenURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}
	if route.Config.Username != "" && route.Config.Password != "" {
		req.SetBasicAuth(route.Config.Username, route.Config.Password)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf(
			"token request failed: status=%d body=%s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var tokenResp struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}
	if token == "" {
		return "", errors.New("token response missing token value")
	}
	return token, nil
}

func buildCredentialHeader(username, password string) string {
	if username == "" || password == "" {
		return ""
	}
	token := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(token))
}

func shouldRetryAuth(route *server.HubRoute, status int) bool {
	return route != nil && route.Config.HasCredentials() && isAuthFailure(status)
}

func isAuthFailure(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusTooManyRequests
}

func (h *Handler) logAuthRetry(route *server.HubRoute, upstream string, requestID string, status int) {
	fields := logging.RequestFields(
		route.Config.Name,
		route.Config.Domain,
		route.Config.Type,
		route.Config.AuthMode(),
		route.ModuleKey,
		string(route.RolloutFlag),
		false,
		route.ModuleKey == hubmodule.DefaultModuleKey(),
	)
	fields["action"] = "proxy_retry"
	fields["upstream"] = upstream
	fields["upstream_status"] = status
	fields["reason"] = "auth_retry"
	if requestID != "" {
		fields["request_id"] = requestID
	}
	h.logger.WithFields(fields).Warn("proxy_auth_retry")
}

func (h *Handler) logAuthFailure(route *server.HubRoute, upstream string, requestID string, status int) {
	fields := logging.RequestFields(
		route.Config.Name,
		route.Config.Domain,
		route.Config.Type,
		route.Config.AuthMode(),
		route.ModuleKey,
		string(route.RolloutFlag),
		false,
		route.ModuleKey == hubmodule.DefaultModuleKey(),
	)
	fields["action"] = "proxy"
	fields["upstream"] = upstream
	fields["upstream_status"] = status
	fields["error"] = "upstream_auth_failed"
	if requestID != "" {
		fields["request_id"] = requestID
	}
	h.logger.WithFields(fields).Error("proxy_auth_failed")
}

func (h *Handler) rememberETag(route *server.HubRoute, locator cache.Locator, resp *http.Response) {
	if resp == nil {
		return
	}
	etag := resp.Header.Get("Docker-Content-Digest")
	if etag == "" {
		etag = resp.Header.Get("Etag")
	}
	etag = normalizeETag(etag)
	if etag == "" {
		return
	}
	h.etags.Store(h.locatorKey(route, locator), etag)
}

func (h *Handler) cachedETag(route *server.HubRoute, locator cache.Locator) string {
	if value, ok := h.etags.Load(h.locatorKey(route, locator)); ok {
		if etag, ok := value.(string); ok {
			return etag
		}
	}
	return ""
}

func (h *Handler) forgetETag(route *server.HubRoute, locator cache.Locator) {
	h.etags.Delete(h.locatorKey(route, locator))
}

func (h *Handler) locatorKey(route *server.HubRoute, locator cache.Locator) string {
	hub := locator.HubName
	if route != nil && route.Config.Name != "" {
		hub = route.Config.Name
	}
	return hub + "::" + locator.Path
}

func normalizeETag(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Trim(value, "\"")
}
