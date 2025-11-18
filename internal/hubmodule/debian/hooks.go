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
	switch {
	case isAptIndexPath(clean):
		// 索引类（Release/Packages）需要 If-None-Match/If-Modified-Since 再验证。
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = true
	case isAptImmutablePath(clean):
		// pool/*.deb 与 by-hash 路径视为不可变，直接缓存后续不再 HEAD。
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = false
	default:
		current.AllowCache = false
		current.AllowStore = false
		current.RequireRevalidate = false
	}
	return current
}

func contentType(_ *hooks.RequestContext, locatorPath string) string {
	switch {
	case strings.HasSuffix(locatorPath, ".gz"):
		return "application/gzip"
	case strings.HasSuffix(locatorPath, ".xz"):
		return "application/x-xz"
	case strings.HasSuffix(locatorPath, "Release.gpg"):
		return "application/pgp-signature"
	case isAptIndexPath(locatorPath):
		return "text/plain"
	default:
		return ""
	}
}

func isAptIndexPath(p string) bool {
	clean := canonicalPath(p)
	if isByHashPath(clean) {
		return false
	}
	if strings.HasPrefix(clean, "/dists/") {
		if strings.HasSuffix(clean, "/release") || strings.HasSuffix(clean, "/inrelease") || strings.HasSuffix(clean, "/release.gpg") {
			return true
		}
		if strings.Contains(clean, "/packages") {
			return true
		}
	}
	return false
}

func isAptImmutablePath(p string) bool {
	clean := canonicalPath(p)
	if isByHashPath(clean) {
		return true
	}
	if strings.HasPrefix(clean, "/pool/") {
		return true
	}
	return false
}

func isByHashPath(p string) bool {
	clean := canonicalPath(p)
	if !strings.HasPrefix(clean, "/dists/") {
		return false
	}
	return strings.Contains(clean, "/by-hash/")
}

func canonicalPath(p string) string {
	if p == "" {
		return "/"
	}
	return strings.ToLower(path.Clean("/" + strings.TrimSpace(p)))
}
