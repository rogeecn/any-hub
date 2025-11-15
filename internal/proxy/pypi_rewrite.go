package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/any-hub/any-hub/internal/server"
)

func (h *Handler) rewritePyPIResponse(route *server.HubRoute, resp *http.Response, path string) (*http.Response, error) {
	if resp == nil {
		return resp, nil
	}
	if !strings.HasPrefix(path, "/simple") && path != "/" {
		return resp, nil
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	resp.Body.Close()

	rewritten, contentType, err := rewritePyPIBody(bodyBytes, resp.Header.Get("Content-Type"), route.Config.Domain)
	if err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return resp, err
	}

	resp.Body = io.NopCloser(bytes.NewReader(rewritten))
	resp.ContentLength = int64(len(rewritten))
	resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
	if contentType != "" {
		resp.Header.Set("Content-Type", contentType)
	}
	resp.Header.Del("Content-Encoding")
	return resp, nil
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
