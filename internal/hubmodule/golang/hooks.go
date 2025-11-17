package golang

import "github.com/any-hub/any-hub/internal/proxy/hooks"

import "strings"

func init() {
	hooks.MustRegister("go", hooks.Hooks{
		CachePolicy: cachePolicy,
	})
}

func cachePolicy(_ *hooks.RequestContext, locatorPath string, current hooks.CachePolicy) hooks.CachePolicy {
	if strings.Contains(locatorPath, "/@v/") &&
		(strings.HasSuffix(locatorPath, ".zip") ||
			strings.HasSuffix(locatorPath, ".mod") ||
			strings.HasSuffix(locatorPath, ".info")) {
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = false
		return current
	}
	current.AllowCache = true
	current.AllowStore = true
	current.RequireRevalidate = true
	return current
}
