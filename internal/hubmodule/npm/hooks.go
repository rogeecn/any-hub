package npm

import (
	"strings"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func init() {
	hooks.MustRegister("npm", hooks.Hooks{
		CachePolicy: cachePolicy,
	})
}

func cachePolicy(_ *hooks.RequestContext, locatorPath string, current hooks.CachePolicy) hooks.CachePolicy {
	if strings.Contains(locatorPath, "/-/") && strings.HasSuffix(locatorPath, ".tgz") {
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
