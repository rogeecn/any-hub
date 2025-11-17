package composer

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

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
	if isComposerDistPath(clean) {
		return clean, nil
	}
	return clean, rawQuery
}

func resolveDistUpstream(_ *hooks.RequestContext, _ string, clean string, rawQuery []byte) string {
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
	switch {
	case path == "/packages.json":
		data, changed, err := rewriteComposerRootBody(body, ctx.Domain)
		if err != nil {
			return status, headers, body, err
		}
		if !changed {
			return status, headers, body, nil
		}
		outHeaders := ensureJSONHeaders(headers)
		return status, outHeaders, data, nil
	case isComposerMetadataPath(path):
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
	type root struct {
		Packages map[string]string `json:"packages"`
	}
	var payload root
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}
	if len(payload.Packages) == 0 {
		return body, false, nil
	}
	changed := false
	for key, value := range payload.Packages {
		rewritten := rewriteComposerAbsolute(domain, value)
		if rewritten != value {
			payload.Packages[key] = rewritten
			changed = true
		}
	}
	if !changed {
		return body, false, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
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

func rewriteComposerPackagesPayload(raw json.RawMessage, domain string, packageName string) (json.RawMessage, bool, error) {
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
	rewritten := rewriteComposerDistURL(domain, urlValue)
	if rewritten == urlValue {
		return changed
	}
	distVal["url"] = rewritten
	return true
}

func rewriteComposerDistURL(domain, original string) string {
	parsed, err := url.Parse(original)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return original
	}
	prefix := "/dist/" + parsed.Scheme + "/" + parsed.Host
	newURL := url.URL{
		Scheme:   "https",
		Host:     domain,
		Path:     prefix + parsed.Path,
		RawQuery: parsed.RawQuery,
		Fragment: parsed.Fragment,
	}
	if raw := parsed.RawPath; raw != "" {
		newURL.RawPath = prefix + raw
	}
	return newURL.String()
}

func rewriteComposerAbsolute(domain, raw string) string {
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "//") {
		return "https://" + domain + strings.TrimPrefix(raw, "//")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		parsed.Host = domain
		parsed.Scheme = "https"
		return parsed.String()
	}
	pathVal := raw
	if !strings.HasPrefix(pathVal, "/") {
		pathVal = "/" + pathVal
	}
	return "https://" + domain + pathVal
}

func isComposerMetadataPath(path string) bool {
	switch {
	case path == "/packages.json":
		return true
	case strings.HasPrefix(path, "/p2/"):
		return true
	case strings.HasPrefix(path, "/p/"):
		return true
	case strings.HasPrefix(path, "/provider-"):
		return true
	case strings.HasPrefix(path, "/providers/"):
		return true
	default:
		return false
	}
}

func isComposerDistPath(path string) bool {
	return strings.HasPrefix(path, "/dist/")
}

func parseComposerDistURL(path string, rawQuery string) (*url.URL, bool) {
	if !strings.HasPrefix(path, "/dist/") {
		return nil, false
	}
	trimmed := strings.TrimPrefix(path, "/dist/")
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
