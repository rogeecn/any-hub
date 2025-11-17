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
		data, changed, err := rewriteComposerRootBody(body)
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

func rewriteComposerRootBody(body []byte) ([]byte, bool, error) {
	// packages.json from Packagist may contain "packages" as array or object; we only care about URL-like fields.
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, err
	}

	changed := false
	for key, val := range root {
		str, ok := val.(string)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		// case "metadata-url", "providers-url", "providers-lazy-url", "notify", "notify-batch", "search":
		case "metadata-url":
			str = strings.ReplaceAll(str, "https://repo.packagist.org", "")
			root[key] = str
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
	rewritten := rewriteComposerDistURL(urlValue)
	if rewritten == urlValue {
		return changed
	}
	distVal["url"] = rewritten
	return true
}

func rewriteComposerDistURL(original string) string {
	parsed, err := url.Parse(original)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return original
	}
	if isPackagistHost(parsed.Host) {
		pathVal := parsed.Path
		if raw := parsed.RawPath; raw != "" {
			pathVal = raw
		}
		if !strings.HasPrefix(pathVal, "/") {
			pathVal = "/" + pathVal
		}
		if parsed.RawQuery != "" {
			return pathVal + "?" + parsed.RawQuery
		}
		return pathVal
	}
	return original
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

func stripPackagistHost(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "https://repo.packagist.org", "")
	raw = strings.ReplaceAll(raw, "http://repo.packagist.org", "")
	return raw
}

func isPackagistHost(host string) bool {
	return strings.EqualFold(host, "repo.packagist.org")
}
