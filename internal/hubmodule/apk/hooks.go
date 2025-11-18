// Package apk defines hook behaviors for Alpine APK proxying.
// APKINDEX/签名需要再验证；packages/*.apk 视为不可变缓存。
package apk

import (
	"path"
	"strings"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func init() {
	hooks.MustRegister("apk", hooks.Hooks{
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
	case isAPKIndexPath(clean), isAPKSignaturePath(clean):
		// APKINDEX 及签名需要再验证，确保索引最新。
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = true
	case isAPKPackagePath(clean):
		// 包体不可变，允许直接命中缓存，无需 HEAD。
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
	clean := canonicalPath(locatorPath)
	switch {
	case strings.HasSuffix(clean, ".apk"):
		return "application/vnd.android.package-archive"
	case strings.HasSuffix(clean, ".tar.gz"):
		return "application/gzip"
	case strings.HasSuffix(clean, ".tar.gz.asc") || strings.HasSuffix(clean, ".tar.gz.sig"):
		return "application/pgp-signature"
	default:
		return ""
	}
}

func isAPKIndexPath(p string) bool {
	clean := canonicalPath(p)
	return strings.HasSuffix(clean, "/apkindex.tar.gz")
}

func isAPKSignaturePath(p string) bool {
	clean := canonicalPath(p)
	return strings.HasSuffix(clean, "/apkindex.tar.gz.asc") || strings.HasSuffix(clean, "/apkindex.tar.gz.sig")
}

func isAPKPackagePath(p string) bool {
	clean := canonicalPath(p)
	if isAPKIndexPath(clean) || isAPKSignaturePath(clean) {
		return false
	}
	return strings.HasSuffix(clean, ".apk")
}

func canonicalPath(p string) string {
	if p == "" {
		return "/"
	}
	return strings.ToLower(path.Clean("/" + strings.TrimSpace(p)))
}
