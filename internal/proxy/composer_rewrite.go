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
		updated, rewritten, err := rewriteComposerPackagesPayload(raw, domain)
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

func rewriteComposerPackagesPayload(raw json.RawMessage, domain string) (json.RawMessage, bool, error) {
	var asArray []map[string]any
	if err := json.Unmarshal(raw, &asArray); err == nil {
		rewrote := rewriteComposerVersionSlice(asArray, domain)
		if !rewrote {
			return raw, false, nil
		}
		data, err := json.Marshal(asArray)
		return data, true, err
	}

	var asMap map[string]map[string]any
	if err := json.Unmarshal(raw, &asMap); err == nil {
		rewrote := rewriteComposerVersionMap(asMap, domain)
		if !rewrote {
			return raw, false, nil
		}
		data, err := json.Marshal(asMap)
		return data, true, err
	}

	return raw, false, nil
}

func rewriteComposerVersionSlice(items []map[string]any, domain string) bool {
	changed := false
	for _, entry := range items {
		if rewriteComposerVersion(entry, domain) {
			changed = true
		}
	}
	return changed
}

func rewriteComposerVersionMap(items map[string]map[string]any, domain string) bool {
	changed := false
	for _, entry := range items {
		if rewriteComposerVersion(entry, domain) {
			changed = true
		}
	}
	return changed
}

func rewriteComposerVersion(entry map[string]any, domain string) bool {
	if entry == nil {
		return false
	}
	distVal, ok := entry["dist"].(map[string]any)
	if !ok {
		return false
	}
	urlValue, ok := distVal["url"].(string)
	if !ok || urlValue == "" {
		return false
	}
	rewritten := rewriteComposerDistURL(domain, urlValue)
	if rewritten == urlValue {
		return false
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
