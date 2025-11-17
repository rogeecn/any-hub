package pypi

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func init() {
	hooks.MustRegister("pypi", hooks.Hooks{
		NormalizePath:   normalizePath,
		ResolveUpstream: resolveFilesUpstream,
		RewriteResponse: rewriteResponse,
		CachePolicy:     cachePolicy,
		ContentType:     contentType,
	})
}

func normalizePath(_ *hooks.RequestContext, clean string, rawQuery []byte) (string, []byte) {
	if strings.HasPrefix(clean, "/files/") || strings.HasPrefix(clean, "/simple/") {
		return ensureSimpleTrailingSlash(clean), rawQuery
	}
	if isDistributionAsset(clean) {
		return clean, rawQuery
	}
	trimmed := strings.Trim(clean, "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "_") {
		return clean, rawQuery
	}
	if !strings.HasSuffix(trimmed, "/") {
		trimmed += "/"
	}
	return "/simple/" + trimmed, rawQuery
}

func ensureSimpleTrailingSlash(path string) string {
	if !strings.HasPrefix(path, "/simple/") {
		return path
	}
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}

func resolveFilesUpstream(_ *hooks.RequestContext, baseURL string, clean string, rawQuery []byte) string {
	if !strings.HasPrefix(clean, "/files/") {
		return ""
	}
	trimmed := strings.TrimPrefix(clean, "/files/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	scheme := parts[0]
	host := parts[1]
	rest := parts[2]
	if scheme == "" || host == "" {
		return ""
	}
	target := url.URL{Scheme: scheme, Host: host, Path: "/" + strings.TrimPrefix(rest, "/")}
	if len(rawQuery) > 0 {
		target.RawQuery = string(rawQuery)
	}
	return target.String()
}

func cachePolicy(_ *hooks.RequestContext, locatorPath string, current hooks.CachePolicy) hooks.CachePolicy {
	if isDistributionAsset(locatorPath) {
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = false
		return current
	}
	current.RequireRevalidate = true
	return current
}

func contentType(_ *hooks.RequestContext, locatorPath string) string {
	if strings.Contains(locatorPath, "/simple/") {
		return "text/html"
	}
	return ""
}

func rewriteResponse(
	ctx *hooks.RequestContext,
	status int,
	headers map[string]string,
	body []byte,
	path string,
) (int, map[string]string, []byte, error) {
	if !strings.HasPrefix(path, "/simple") && path != "/" {
		return status, headers, body, nil
	}
	domain := ctx.Domain
	rewritten, contentType, err := rewritePyPIBody(body, headers["Content-Type"], domain)
	if err != nil {
		return status, headers, body, err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	delete(headers, "Content-Encoding")
	return status, headers, rewritten, nil
}

func rewritePyPIBody(body []byte, contentType string, domain string) ([]byte, string, error) {
	lowerCT := strings.ToLower(contentType)
	if strings.Contains(lowerCT, "application/vnd.pypi.simple.v1+json") || strings.HasPrefix(strings.TrimSpace(string(body)), "{") {
		data := map[string]interface{}{}
		if err := json.Unmarshal(body, &data); err != nil {
			return body, contentType, err
		}
		if files, ok := data["files"].([]interface{}); ok {
			for _, entry := range files {
				if fileMap, ok := entry.(map[string]interface{}); ok {
					if urlValue, ok := fileMap["url"].(string); ok {
						fileMap["url"] = rewritePyPIFileURL(domain, urlValue)
					}
				}
			}
		}
		rewriteBytes, err := json.Marshal(data)
		if err != nil {
			return body, contentType, err
		}
		return rewriteBytes, "application/vnd.pypi.simple.v1+json", nil
	}

	rewrittenHTML, err := rewritePyPIHTML(body, domain)
	if err != nil {
		return body, contentType, err
	}
	return rewrittenHTML, "text/html; charset=utf-8", nil
}

func rewritePyPIHTML(body []byte, domain string) ([]byte, error) {
	node, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	rewriteHTMLNode(node, domain)
	var buf bytes.Buffer
	if err := html.Render(&buf, node); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rewriteHTMLNode(n *html.Node, domain string) {
	if n.Type == html.ElementNode {
		rewriteHTMLAttributes(n, domain)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		rewriteHTMLNode(child, domain)
	}
}

func rewriteHTMLAttributes(n *html.Node, domain string) {
	for i, attr := range n.Attr {
		switch attr.Key {
		case "href", "data-dist-info-metadata", "data-core-metadata":
			if strings.HasPrefix(attr.Val, "http://") || strings.HasPrefix(attr.Val, "https://") {
				n.Attr[i].Val = rewritePyPIFileURL(domain, attr.Val)
			}
		}
	}
}

func rewritePyPIFileURL(domain, original string) string {
	parsed, err := url.Parse(original)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return original
	}
	prefix := "/files/" + parsed.Scheme + "/" + parsed.Host
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

func isDistributionAsset(path string) bool {
	switch {
	case strings.HasSuffix(path, ".whl"):
		return true
	case strings.HasSuffix(path, ".tar.gz"):
		return true
	case strings.HasSuffix(path, ".tar.bz2"):
		return true
	case strings.HasSuffix(path, ".tgz"):
		return true
	case strings.HasSuffix(path, ".zip"):
		return true
	case strings.HasSuffix(path, ".egg"):
		return true
	default:
		return false
	}
}
