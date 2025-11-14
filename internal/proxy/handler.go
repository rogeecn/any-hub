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
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/logging"
	"github.com/any-hub/any-hub/internal/server"
)

// Handler 负责 orchestrate “缓存命中 → revalidate → 回源写缓存” 的全流程，
// 对外暴露 Fiber handler，内部复用共享 http.Client 与磁盘缓存。
type Handler struct {
	client *http.Client
	logger *logrus.Logger
	store  cache.Store
}

// NewHandler constructs a proxy handler with shared HTTP client/logger/store.
func NewHandler(client *http.Client, logger *logrus.Logger, store cache.Store) *Handler {
	return &Handler{
		client: client,
		logger: logger,
		store:  store,
	}
}

// Handle 执行缓存查找、条件回源和最终 streaming 逻辑，任何阶段出错都会输出结构化日志。
func (h *Handler) Handle(c fiber.Ctx, route *server.HubRoute) error {
	started := time.Now()
	requestID := server.RequestID(c)
	locator := buildLocator(route, c)
	policy := determineCachePolicy(route, locator, c.Method())

	if err := ensureProxyHubType(route); err != nil {
		h.logger.WithField("hub", route.Config.Name).WithError(err).Error("hub_type_unsupported")
		return h.writeError(c, fiber.StatusNotImplemented, "hub_type_unsupported")
	}

	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	var cached *cache.ReadResult
	if h.store != nil && policy.allowCache {
		result, err := h.store.Get(ctx, locator)
		switch {
		case err == nil:
			cached = result
		case errors.Is(err, cache.ErrNotFound):
			// miss, continue
		default:
			h.logger.WithError(err).WithField("hub", route.Config.Name).Warn("cache_get_failed")
		}
	}

	if cached != nil {
		serve := true
		if policy.requireRevalidate {
			fresh, err := h.isCacheFresh(c, route, locator, cached.Entry)
			if err != nil {
				h.logger.WithError(err).WithField("hub", route.Config.Name).Warn("cache_revalidate_failed")
				serve = false
			} else if !fresh {
				serve = false
			}
		}
		if serve {
			defer cached.Reader.Close()
			return h.serveCache(c, route, cached, requestID, started)
		}
		cached.Reader.Close()
	}

	return h.fetchAndStream(c, route, locator, policy, requestID, started, ctx)
}

func (h *Handler) serveCache(c fiber.Ctx, route *server.HubRoute, result *cache.ReadResult, requestID string, started time.Time) error {
	if seeker, ok := result.Reader.(io.Seeker); ok {
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	method := c.Method()

	contentType := inferCachedContentType(route, result.Entry.Locator)
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

func (h *Handler) fetchAndStream(c fiber.Ctx, route *server.HubRoute, locator cache.Locator, policy cachePolicy, requestID string, started time.Time, ctx context.Context) error {
	resp, upstreamURL, err := h.executeRequest(c, route)
	if err != nil {
		h.logResult(route, upstreamURL.String(), requestID, 0, false, started, err)
		return h.writeError(c, fiber.StatusBadGateway, "upstream_failed")
	}

	resp, upstreamURL, err = h.retryOnAuthFailure(c, route, requestID, started, resp, upstreamURL)
	if err != nil {
		h.logResult(route, upstreamURL.String(), requestID, 0, false, started, err)
		return h.writeError(c, fiber.StatusBadGateway, "upstream_failed")
	}
	defer resp.Body.Close()

	shouldStore := policy.allowStore && h.store != nil && isCacheableStatus(resp.StatusCode) && c.Method() == http.MethodGet
	return h.consumeUpstream(c, route, locator, resp, shouldStore, requestID, started, ctx)
}

func (h *Handler) consumeUpstream(c fiber.Ctx, route *server.HubRoute, locator cache.Locator, resp *http.Response, shouldStore bool, requestID string, started time.Time, ctx context.Context) error {
	upstreamURL := resp.Request.URL.String()
	method := c.Method()
	authFailure := isAuthFailure(resp.StatusCode) && route.Config.HasCredentials()

	if shouldStore {
		return h.cacheAndStream(c, route, locator, resp, requestID, started, ctx, upstreamURL)
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

func (h *Handler) cacheAndStream(c fiber.Ctx, route *server.HubRoute, locator cache.Locator, resp *http.Response, requestID string, started time.Time, ctx context.Context, upstreamURL string) error {
	copyResponseHeaders(c, resp.Header)
	c.Set("X-Any-Hub-Upstream", upstreamURL)
	c.Set("X-Any-Hub-Cache-Hit", "false")
	if requestID != "" {
		c.Set("X-Request-ID", requestID)
	}
	c.Status(resp.StatusCode)

	reader := io.TeeReader(resp.Body, c.Response().BodyWriter())

	opts := cache.PutOptions{ModTime: extractModTime(resp.Header)}
	entry, err := h.store.Put(ctx, locator, reader, opts)
	h.logResult(route, upstreamURL, requestID, resp.StatusCode, false, started, err)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, fmt.Sprintf("cache_write_failed: %v", err))
	}
	_ = entry
	return nil
}

func (h *Handler) retryOnAuthFailure(c fiber.Ctx, route *server.HubRoute, requestID string, started time.Time, resp *http.Response, upstreamURL *url.URL) (*http.Response, *url.URL, error) {
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
		retryResp, retryURL, err := h.executeRequestWithAuth(c, route, authHeader)
		if err != nil {
			return nil, upstreamURL, err
		}
		return retryResp, retryURL, nil
	}

	retryResp, retryURL, err := h.executeRequest(c, route)
	if err != nil {
		return nil, upstreamURL, err
	}
	return retryResp, retryURL, nil
}

func (h *Handler) executeRequest(c fiber.Ctx, route *server.HubRoute) (*http.Response, *url.URL, error) {
	return h.executeRequestWithAuth(c, route, "")
}

func (h *Handler) executeRequestWithAuth(c fiber.Ctx, route *server.HubRoute, authHeader string) (*http.Response, *url.URL, error) {
	upstreamURL := resolveUpstreamURL(route, route.UpstreamURL, c)
	body := bytesReader(c.Body())
	req, err := h.buildUpstreamRequest(c, upstreamURL, route, c.Method(), body, authHeader)
	if err != nil {
		return nil, upstreamURL, err
	}

	resp, err := h.doRequest(req, route)
	return resp, upstreamURL, err
}

func (h *Handler) buildUpstreamRequest(c fiber.Ctx, upstream *url.URL, route *server.HubRoute, method string, body io.Reader, overrideAuth string) (*http.Request, error) {
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

func (h *Handler) logResult(route *server.HubRoute, upstream string, requestID string, status int, cacheHit bool, started time.Time, err error) {
	fields := logging.RequestFields(route.Config.Name, route.Config.Domain, route.Config.Type, route.Config.AuthMode(), cacheHit)
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
	case strings.HasSuffix(clean, ".mod"):
		return "text/plain"
	case strings.HasSuffix(clean, ".info"):
		return "application/json"
	case strings.HasSuffix(clean, ".tgz"):
		return "application/octet-stream"
	case strings.HasSuffix(clean, "/@v/list"):
		return "text/plain"
	}

	if route != nil {
		switch route.Config.Type {
		case "docker":
			if strings.Contains(clean, "/manifests/") {
				return "application/vnd.docker.distribution.manifest.v2+json"
			}
			if strings.Contains(clean, "/tags/list") {
				return "application/json"
			}
			if strings.Contains(clean, "/blobs/") {
				return "application/octet-stream"
			}
		case "npm":
			if strings.HasSuffix(clean, ".json") {
				return "application/json"
			}
		}
	}

	return ""
}

func buildLocator(route *server.HubRoute, c fiber.Ctx) cache.Locator {
	uri := c.Request().URI()
	pathVal := string(uri.Path())
	if pathVal == "" {
		pathVal = "/"
	}
	clean := path.Clean("/" + pathVal)
	if newPath, ok := applyDockerHubNamespaceFallback(route, clean); ok {
		clean = newPath
	}
	query := uri.QueryString()
	if len(query) > 0 {
		sum := sha1.Sum(query)
		clean = fmt.Sprintf("%s/__qs/%s", clean, hex.EncodeToString(sum[:]))
	}
	return cache.Locator{
		HubName: route.Config.Name,
		Path:    clean,
	}
}

func stripQueryMarker(p string) string {
	if idx := strings.Index(p, "/__qs/"); idx >= 0 {
		return p[:idx]
	}
	return p
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

func bytesReader(b []byte) io.Reader {
	if len(b) == 0 {
		return http.NoBody
	}
	return bytes.NewReader(b)
}

func resolveUpstreamURL(route *server.HubRoute, base *url.URL, c fiber.Ctx) *url.URL {
	uri := c.Request().URI()
	pathVal := string(uri.Path())
	relative := &url.URL{Path: pathVal, RawPath: pathVal}
	if newPath, ok := applyDockerHubNamespaceFallback(route, relative.Path); ok {
		relative.Path = newPath
		relative.RawPath = newPath
	}
	if query := string(uri.QueryString()); query != "" {
		relative.RawQuery = query
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
	allowCache       bool
	allowStore       bool
	requireRevalidate bool
}

func determineCachePolicy(route *server.HubRoute, locator cache.Locator, method string) cachePolicy {
	if route == nil || method != http.MethodGet {
		return cachePolicy{}
	}
	policy := cachePolicy{allowCache: true, allowStore: true}
	path := stripQueryMarker(locator.Path)
	switch route.Config.Type {
	case "docker":
		if path == "/v2" || path == "v2" || path == "/v2/" {
			return cachePolicy{}
		}
		if strings.Contains(path, "/_catalog") {
			return cachePolicy{}
		}
		if isDockerImmutablePath(path) {
			return policy
		}
		policy.requireRevalidate = true
		return policy
	case "go":
		if strings.Contains(path, "/@v/") && (strings.HasSuffix(path, ".zip") || strings.HasSuffix(path, ".mod") || strings.HasSuffix(path, ".info")) {
			return policy
		}
		policy.requireRevalidate = true
		return policy
	case "npm":
		if strings.Contains(path, "/-/") && strings.HasSuffix(path, ".tgz") {
			return policy
		}
		policy.requireRevalidate = true
		return policy
	default:
		return policy
	}
}

func isDockerImmutablePath(path string) bool {
	if strings.Contains(path, "/blobs/sha256:") {
		return true
	}
	if strings.Contains(path, "/manifests/sha256:") {
		return true
	}
	return false
}

func isCacheableStatus(status int) bool {
	return status == http.StatusOK
}

func (h *Handler) isCacheFresh(c fiber.Ctx, route *server.HubRoute, locator cache.Locator, entry cache.Entry) (bool, error) {
	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	upstreamURL := resolveUpstreamURL(route, route.UpstreamURL, c)
	req, err := h.buildUpstreamRequest(c, upstreamURL, route, http.MethodHead, http.NoBody, "")
	if err != nil {
		return false, err
	}

	resp, err := h.doRequest(req, route)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		remote := extractModTime(resp.Header)
		if !remote.After(entry.ModTime.Add(time.Second)) {
			return true, nil
		}
		return false, nil
	case http.StatusNotFound:
		if h.store != nil {
			_ = h.store.Remove(ctx, locator)
		}
		return false, nil
	default:
		return false, nil
	}
}

func extractModTime(header http.Header) time.Time {
	if last := header.Get("Last-Modified"); last != "" {
		if parsed, err := http.ParseTime(last); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func applyDockerHubNamespaceFallback(route *server.HubRoute, path string) (string, bool) {
	if !isDockerHubRoute(route) {
		return path, false
	}
	repo, rest, ok := splitDockerRepoPath(path)
	if !ok || repo == "" {
		return path, false
	}
	if repo == "library" || strings.Contains(repo, "/") {
		return path, false
	}
	normalized := "/v2/library/" + repo + rest
	return normalized, true
}

func isDockerHubRoute(route *server.HubRoute) bool {
	if route == nil || route.Config.Type != "docker" || route.UpstreamURL == nil {
		return false
	}
	host := strings.ToLower(route.UpstreamURL.Hostname())
	switch host {
	case "registry-1.docker.io", "docker.io", "index.docker.io":
		return true
	default:
		return false
	}
}

func splitDockerRepoPath(path string) (string, string, bool) {
	if !strings.HasPrefix(path, "/v2/") {
		return "", "", false
	}
	suffix := strings.TrimPrefix(path, "/v2/")
	if suffix == "" || suffix == "/" {
		return "", "", false
	}
	segments := strings.Split(suffix, "/")
	var repoSegments []string
	for i, seg := range segments {
		if seg == "" {
			return "", "", false
		}
		switch seg {
		case "manifests", "blobs", "tags", "referrers":
			if len(repoSegments) == 0 {
				return "", "", false
			}
			rest := "/" + strings.Join(segments[i:], "/")
			return strings.Join(repoSegments, "/"), rest, true
		case "_catalog":
			return "", "", false
		}
		repoSegments = append(repoSegments, seg)
	}
	return "", "", false
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

func (h *Handler) fetchBearerToken(ctx context.Context, challenge bearerChallenge, route *server.HubRoute) (string, error) {
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
		return "", fmt.Errorf("token request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
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
	fields := logging.RequestFields(route.Config.Name, route.Config.Domain, route.Config.Type, route.Config.AuthMode(), false)
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
	fields := logging.RequestFields(route.Config.Name, route.Config.Domain, route.Config.Type, route.Config.AuthMode(), false)
	fields["action"] = "proxy"
	fields["upstream"] = upstream
	fields["upstream_status"] = status
	fields["error"] = "upstream_auth_failed"
	if requestID != "" {
		fields["request_id"] = requestID
	}
	h.logger.WithFields(fields).Error("proxy_auth_failed")
}

func ensureProxyHubType(route *server.HubRoute) error {
	switch route.Config.Type {
	case "docker":
		return nil
	case "npm":
		return nil
	case "go":
		return nil
	default:
		return fmt.Errorf("unsupported hub type: %s", route.Config.Type)
	}
}
