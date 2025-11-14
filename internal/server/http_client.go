package server

import (
	"net"
	"net/http"
	"net/textproto"
	"time"

	"github.com/any-hub/any-hub/internal/config"
)

// Shared HTTP transport tunings，复用长连接并集中配置超时。
var defaultTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ForceAttemptHTTP2:     true,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
}

// NewUpstreamClient 返回共享 http.Client，用于所有上游请求。
func NewUpstreamClient(cfg *config.Config) *http.Client {
	timeout := 30 * time.Second
	if cfg != nil && cfg.Global.UpstreamTimeout.DurationValue() > 0 {
		timeout = cfg.Global.UpstreamTimeout.DurationValue()
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: defaultTransport.Clone(),
	}
}

// hopByHopHeaders 定义 RFC 7230 中禁止代理转发的头部。
var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Proxy-Connection":    {}, // 非标准字段，但部分代理仍使用
}

// CopyHeaders 将 src 中允许透传的头复制到 dst，自动忽略 hop-by-hop 字段。
func CopyHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	canonical := textproto.CanonicalMIMEHeaderKey(key)
	if _, ok := hopByHopHeaders[canonical]; ok {
		return true
	}

	return false
}

// IsHopByHopHeader reports whether the header should be stripped by proxies.
func IsHopByHopHeader(key string) bool {
	return isHopByHopHeader(key)
}
