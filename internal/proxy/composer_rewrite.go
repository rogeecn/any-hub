package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/any-hub/any-hub/internal/server"
)

func (h *Handler) rewriteComposerResponse(route *server.HubRoute, resp *http.Response, path string) (*http.Response, error) {
	if resp == nil || route == nil || route.Config.Type != "composer" {
		return resp, nil
	}
	if path == "/packages.json" {
		return rewriteComposerRoot(resp, route.Config.Domain)
	}
	if !isComposerMetadataPath(path) {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	resp.Body.Close()

	rewritten, changed, err := rewriteComposerMetadata(body, route.Config.Domain)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, err
	}
	if !changed {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(rewritten))
	resp.ContentLength = int64(len(rewritten))
	resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Etag")
	return resp, nil
}

func rewriteComposerRoot(resp *http.Response, domain string) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	resp.Body.Close()

	data, changed, err := rewriteComposerRootBody(body, domain)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, err
	}
	if !changed {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	resp.Header.Set("Content-Length", strconv.Itoa(len(data)))
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Etag")
	return resp, nil
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
	prefix := fmt.Sprintf("/dist/%s/%s", parsed.Scheme, parsed.Host)
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
	return fmt.Sprintf("https://%s%s", domain, pathVal)
}

func rewriteComposerRootBody(body []byte, domain string) ([]byte, bool, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, err
	}

	changed := false
	for _, key := range []string{"metadata-url", "providers-api", "providers-url", "notify-batch"} {
		if raw, ok := root[key].(string); ok && raw != "" {
			newVal := rewriteComposerAbsolute(domain, raw)
			if newVal != raw {
				root[key] = newVal
				changed = true
			}
		}
	}

	if includes, ok := root["provider-includes"].(map[string]any); ok {
		for file, hashVal := range includes {
			pathVal := file
			if rawPath, ok := hashVal.(map[string]any); ok {
				if urlValue, ok := rawPath["url"].(string); ok {
					pathVal = urlValue
				}
			}
			newPath := rewriteComposerAbsolute(domain, pathVal)
			if newPath != pathVal {
				changed = true
			}
			if rawPath, ok := hashVal.(map[string]any); ok {
				rawPath["url"] = newPath
				includes[file] = rawPath
			} else {
				includes[file] = newPath
			}
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
