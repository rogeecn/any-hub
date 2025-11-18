// Package debian defines hook behaviors for APT (Debian/Ubuntu) proxying.
// 索引（Release/InRelease/Packages*）需要再验证；包体（pool/ 和 by-hash）视为不可变直接缓存。
// 日志字段沿用通用 proxy（命中/上游状态），无需额外改写。
package debian

import (
	"path"
	"strings"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func init() {
	hooks.MustRegister("debian", hooks.Hooks{
		NormalizePath: normalizePath,
		CachePolicy:   cachePolicy,
		ContentType:   contentType,
	})
}

func normalizePath(_ *hooks.RequestContext, p string, rawQuery []byte) (string, []byte) {
	clean := path.Clean("/" + strings.TrimSpace(p))
	return clean, rawQuery
}

func cachePolicy(_ *hooks.RequestContext, locatorPath string, current hooks.CachePolicy) hooks.CachePolicy {
	clean := canonicalPath(locatorPath)
	if strings.Contains(clean, "/by-hash/") || strings.Contains(clean, "/pool/") {
		// pool/*.deb 与 by-hash 路径视为不可变，直接缓存后续不再 HEAD。
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = false
		return current
	}

	if strings.Contains(clean, "/dists/") {
		// 索引类（Release/Packages/Contents）需要 If-None-Match/If-Modified-Since 再验证。
		if strings.HasSuffix(clean, "/release") ||
			strings.HasSuffix(clean, "/inrelease") ||
			strings.HasSuffix(clean, "/release.gpg") {
			current.AllowCache = true
			current.AllowStore = true
			current.RequireRevalidate = true
			return current
		}
	}

	current.AllowCache = false
	current.AllowStore = false
	current.RequireRevalidate = false
	return current
}

func contentType(_ *hooks.RequestContext, locatorPath string) string {
	clean := canonicalPath(locatorPath)
	switch {
	case strings.HasSuffix(clean, ".gz"):
		return "application/gzip"
	case strings.HasSuffix(clean, ".xz"):
		return "application/x-xz"
	case strings.HasSuffix(clean, "release.gpg"):
		return "application/pgp-signature"
	case strings.Contains(clean, "/dists/") &&
		(strings.HasSuffix(clean, "/release") || strings.HasSuffix(clean, "/inrelease") || strings.HasSuffix(clean, "/release.gpg")):
		return "text/plain"
	default:
		return ""
	}
}

func canonicalPath(p string) string {
	if p == "" {
		return "/"
	}
	return strings.ToLower(path.Clean("/" + strings.TrimSpace(p)))
}
