package composer

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

var composerDistRegistry sync.Map

func init() {
	hooks.MustRegister("composer", hooks.Hooks{
		NormalizePath:   normalizePath,
		ResolveUpstream: resolveDistUpstream,
		RewriteResponse: rewriteResponse,
		CachePolicy:     cachePolicy,
		ContentType:     contentType,
	})
}

func normalizePath(_ *hooks.RequestContext, clean string, rawQuery []byte) (string, []byte) {
	if trimmed := trimComposerNamespace(clean); trimmed != clean {
		clean = trimmed
	}
	if isComposerDistPath(clean) {
		return clean, nil
	}
	return clean, rawQuery
}

func resolveDistUpstream(ctx *hooks.RequestContext, _ string, clean string, rawQuery []byte) string {
	domain := ""
	if ctx != nil {
		domain = ctx.Domain
	}
	if target := resolveComposerMirrorDist(domain, clean); target != "" {
		return target
	}
	if !isComposerDistPath(clean) {
		return ""
	}
	target, ok := parseComposerDistURL(clean, string(rawQuery))
	if !ok {
		return ""
	}
	return target.String()
}

func rewriteResponse(
	ctx *hooks.RequestContext,
	status int,
	headers map[string]string,
	body []byte,
	path string,
) (int, map[string]string, []byte, error) {
	cleanPath := trimComposerNamespace(path)
	switch {
	case cleanPath == "/packages.json":
		data, changed, err := rewriteComposerRootBody(body, ctx.Domain)
		if err != nil {
			return status, headers, body, err
		}
		if !changed {
			return status, headers, body, nil
		}
		outHeaders := ensureJSONHeaders(headers)
		return status, outHeaders, data, nil
	case isComposerMetadataPath(cleanPath):
		data, changed, err := rewriteComposerMetadata(body, ctx.Domain)
		if err != nil {
			return status, headers, body, err
		}
		if !changed {
			return status, headers, body, nil
		}
		outHeaders := ensureJSONHeaders(headers)
		return status, outHeaders, data, nil
	default:
		return status, headers, body, nil
	}
}

func ensureJSONHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Content-Type"] = "application/json"
	delete(headers, "Content-Encoding")
	delete(headers, "Etag")
	return headers
}

func cachePolicy(_ *hooks.RequestContext, locatorPath string, current hooks.CachePolicy) hooks.CachePolicy {
	switch {
	case isComposerDistPath(locatorPath):
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = false
	case isComposerMetadataPath(locatorPath):
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = true
	default:
		current.AllowCache = false
		current.AllowStore = false
		current.RequireRevalidate = false
	}
	return current
}

func contentType(_ *hooks.RequestContext, locatorPath string) string {
	if isComposerMetadataPath(locatorPath) {
		return "application/json"
	}
	return ""
}

func rewriteComposerRootBody(body []byte, domain string) ([]byte, bool, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, err
	}

	changed := false
	if rewriteComposerRootURLField(root, "metadata-url", domain) {
		changed = true
	}
	if rewriteComposerRootURLField(root, "providers-url", domain) {
		changed = true
	}
	if ensureComposerMirrors(root, domain) {
		changed = true
	}

	if !changed {
		return body, false, nil
	}
	data, err := json.Marshal(root)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func rewriteComposerRootURLField(root map[string]any, key string, domain string) bool {
	value, ok := root[key].(string)
	if !ok || value == "" {
		return false
	}
	proxied := buildComposerProxyURL(value, domain)
	if proxied == value {
		return false
	}
	root[key] = proxied
	return true
}

func ensureComposerMirrors(root map[string]any, domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	target := "https://" + domain + "/dists/%package%/%reference%.%type%"
	if existing, ok := root["mirrors"].([]any); ok && len(existing) == 1 {
		if entry, ok := existing[0].(map[string]any); ok {
			distURL, _ := entry["dist-url"].(string)
			preferred, _ := entry["preferred"].(bool)
			if distURL == target && preferred {
				return false
			}
		}
	}
	root["mirrors"] = []map[string]any{
		{
			"dist-url":  target,
			"preferred": true,
		},
	}
	return true
}

func rewriteComposerMetadata(body []byte, domain string) ([]byte, bool, error) {
	type packagesRoot struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	var root packagesRoot
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, err
	}
	if len(root.Packages) == 0 {
		return body, false, nil
	}

	changed := false
	for name, raw := range root.Packages {
		updated, rewritten, err := rewriteComposerPackagesPayload(raw, domain, name)
		if err != nil {
			return nil, false, err
		}
		if rewritten {
			root.Packages[name] = updated
			changed = true
		}
	}
	if !changed {
		return body, false, nil
	}
	data, err := json.Marshal(root)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func rewriteComposerPackagesPayload(
	raw json.RawMessage,
	domain string,
	packageName string,
) (json.RawMessage, bool, error) {
	var asArray []map[string]any
	if err := json.Unmarshal(raw, &asArray); err == nil {
		rewrote := rewriteComposerVersionSlice(asArray, domain, packageName)
		if !rewrote {
			return raw, false, nil
		}
		data, err := json.Marshal(asArray)
		return data, true, err
	}

	var asMap map[string]map[string]any
	if err := json.Unmarshal(raw, &asMap); err == nil {
		rewrote := rewriteComposerVersionMap(asMap, domain, packageName)
		if !rewrote {
			return raw, false, nil
		}
		data, err := json.Marshal(asMap)
		return data, true, err
	}

	return raw, false, nil
}

func rewriteComposerVersionSlice(items []map[string]any, domain string, packageName string) bool {
	changed := false
	for _, entry := range items {
		if rewriteComposerVersion(entry, domain, packageName) {
			changed = true
		}
	}
	return changed
}

func rewriteComposerVersionMap(items map[string]map[string]any, domain string, packageName string) bool {
	changed := false
	for _, entry := range items {
		if rewriteComposerVersion(entry, domain, packageName) {
			changed = true
		}
	}
	return changed
}

func rewriteComposerVersion(entry map[string]any, domain string, packageName string) bool {
	if entry == nil {
		return false
	}
	changed := false
	if packageName != "" {
		if name, _ := entry["name"].(string); strings.TrimSpace(name) == "" {
			entry["name"] = packageName
			changed = true
		}
	}
	distVal, ok := entry["dist"].(map[string]any)
	if !ok {
		return changed
	}
	urlValue, ok := distVal["url"].(string)
	if !ok || urlValue == "" {
		return changed
	}
	reference, _ := distVal["reference"].(string)
	distType, _ := distVal["type"].(string)
	if packageName != "" && domain != "" && reference != "" && distType != "" {
		registerComposerDist(domain, packageName, reference, distType, urlValue)
	}
	rewritten := rewriteComposerLegacyDistURL(urlValue, domain)
	if rewritten == urlValue {
		return changed
	}
	distVal["url"] = rewritten
	return true
}

func rewriteComposerLegacyDistURL(original string, domain string) string {
	trimmed := strings.TrimSpace(original)
	if trimmed == "" {
		return original
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return original
	}
	if domain != "" && strings.EqualFold(parsed.Host, domain) && strings.HasPrefix(parsed.Path, "/dist/") {
		// Already rewritten.
		return original
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return original
	}
	pathVal := parsed.Path
	if raw := parsed.RawPath; raw != "" {
		pathVal = raw
	}
	if !strings.HasPrefix(pathVal, "/") {
		pathVal = "/" + pathVal
	}
	var builder strings.Builder
	builder.WriteString("/dist/")
	builder.WriteString(parsed.Scheme)
	builder.WriteString("/")
	builder.WriteString(parsed.Host)
	builder.WriteString(pathVal)
	if parsed.RawQuery != "" {
		builder.WriteString("?")
		builder.WriteString(parsed.RawQuery)
	}
	proxiedPath := builder.String()
	if domain == "" {
		return proxiedPath
	}
	return buildComposerProxyURL(proxiedPath, domain)
}

func isComposerMetadataPath(path string) bool {
	clean := trimComposerNamespace(path)
	switch {
	case clean == "/packages.json":
		return true
	case strings.HasPrefix(clean, "/p2/"):
		return true
	case strings.HasPrefix(clean, "/p/"):
		return true
	case strings.HasPrefix(clean, "/provider-"):
		return true
	case strings.HasPrefix(clean, "/providers/"):
		return true
	default:
		return false
	}
}

func isComposerDistPath(path string) bool {
	clean := trimComposerNamespace(path)
	if strings.HasPrefix(clean, "/dist/") {
		return true
	}
	return strings.HasPrefix(clean, "/dists/")
}

func parseComposerDistURL(path string, rawQuery string) (*url.URL, bool) {
	clean := trimComposerNamespace(path)
	if !strings.HasPrefix(clean, "/dist/") {
		return nil, false
	}
	trimmed := strings.TrimPrefix(clean, "/dist/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 3 {
		return nil, false
	}
	scheme := parts[0]
	host := parts[1]
	rest := parts[2]
	if scheme == "" || host == "" {
		return nil, false
	}
	if rest == "" {
		rest = "/"
	} else {
		rest = "/" + rest
	}
	target := &url.URL{
		Scheme:  scheme,
		Host:    host,
		Path:    rest,
		RawPath: rest,
	}
	if rawQuery != "" {
		target.RawQuery = rawQuery
	}
	return target, true
}

func stripPackagistHost(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "https://repo.packagist.org", "")
	raw = strings.ReplaceAll(raw, "http://repo.packagist.org", "")
	return raw
}

func isPackagistHost(host string) bool {
	return strings.EqualFold(host, "repo.packagist.org")
}

func buildComposerProxyURL(raw string, domain string) string {
	trimmed := stripPackagistHost(strings.TrimSpace(raw))
	if trimmed == "" {
		return trimmed
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		if domain != "" && strings.EqualFold(parsed.Host, domain) {
			return trimmed
		}
		if !isPackagistHost(parsed.Host) {
			return trimmed
		}
		if path := parsed.EscapedPath(); path != "" {
			trimmed = path
			if parsed.RawQuery != "" {
				trimmed = trimmed + "?" + parsed.RawQuery
			}
		}
	}

	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}

	if domain == "" {
		return trimmed
	}
	return "https://" + domain + trimmed
}

func resolveComposerMirrorDist(domain string, locator string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	pkg, reference, distType, ok := parseComposerMirrorDistLocator(locator)
	if !ok {
		return ""
	}
	target, ok := lookupComposerDist(domain, pkg, reference, distType)
	if !ok {
		return ""
	}
	return target
}

func parseComposerMirrorDistLocator(locator string) (string, string, string, bool) {
	clean := trimComposerNamespace(locator)
	if !strings.HasPrefix(clean, "/dists/") {
		return "", "", "", false
	}
	trimmed := strings.TrimPrefix(clean, "/dists/")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash <= 0 || lastSlash >= len(trimmed)-1 {
		return "", "", "", false
	}
	packagePart := trimmed[:lastSlash]
	file := trimmed[lastSlash+1:]
	if packagePart == "" || file == "" {
		return "", "", "", false
	}
	ext := path.Ext(file)
	if ext == "" {
		return "", "", "", false
	}
	reference := strings.TrimSuffix(file, ext)
	distType := strings.TrimPrefix(ext, ".")
	if reference == "" || distType == "" {
		return "", "", "", false
	}
	packageName := strings.ToLower(strings.Trim(packagePart, "/"))
	if packageName == "" {
		return "", "", "", false
	}
	return packageName, reference, distType, true
}

func registerComposerDist(domain string, packageName string, reference string, distType string, upstream string) {
	key := composerDistKey(domain, packageName, reference, distType)
	if key == "" || strings.TrimSpace(upstream) == "" {
		return
	}
	composerDistRegistry.Store(key, upstream)
}

func lookupComposerDist(domain string, packageName string, reference string, distType string) (string, bool) {
	key := composerDistKey(domain, packageName, reference, distType)
	if key == "" {
		return "", false
	}
	value, ok := composerDistRegistry.Load(key)
	if !ok {
		return "", false
	}
	str, _ := value.(string)
	if strings.TrimSpace(str) == "" {
		return "", false
	}
	return str, true
}

func composerDistKey(domain string, packageName string, reference string, distType string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	pkg := strings.ToLower(strings.TrimSpace(packageName))
	ref := strings.TrimSpace(reference)
	typ := strings.ToLower(strings.TrimSpace(distType))
	if domain == "" || pkg == "" || ref == "" || typ == "" {
		return ""
	}
	return domain + "|" + pkg + "|" + ref + "|" + typ
}

func trimComposerNamespace(p string) string {
	if strings.HasPrefix(p, "/composer/") {
		return strings.TrimPrefix(p, "/composer")
	}
	if p == "/composer" {
		return "/"
	}
	return p
}

func resetComposerDistRegistry() {
	composerDistRegistry.Range(func(key, _ any) bool {
		composerDistRegistry.Delete(key)
		return true
	})
}
