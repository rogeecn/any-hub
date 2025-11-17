package hooks

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// CachePolicy mirrors the proxy cache policy structure.
type CachePolicy struct {
	AllowCache        bool
	AllowStore        bool
	RequireRevalidate bool
}

// RequestContext exposes route/request details without importing server internals.
type RequestContext struct {
	HubName      string
	Domain       string
	HubType      string
	ModuleKey    string
	RolloutFlag  string
	UpstreamHost string
	Method       string
}

// Hooks describes customization points for module-specific behavior.
type Hooks struct {
	NormalizePath   func(ctx *RequestContext, cleanPath string) string
	ResolveUpstream func(ctx *RequestContext, base *url.URL, cleanPath string, rawQuery []byte) *url.URL
	RewriteResponse func(ctx *RequestContext, resp *http.Response, cleanPath string) (*http.Response, error)
	CachePolicy     func(ctx *RequestContext, locatorPath string, current CachePolicy) CachePolicy
	ContentType     func(ctx *RequestContext, locatorPath string) string
}

var registry sync.Map

// Register stores hooks for the given module key.
func Register(moduleKey string, hooks Hooks) {
	key := strings.ToLower(strings.TrimSpace(moduleKey))
	if key == "" {
		return
	}
	registry.Store(key, hooks)
}

// For retrieves hooks associated with a module key.
func For(moduleKey string) (Hooks, bool) {
	key := strings.ToLower(strings.TrimSpace(moduleKey))
	if key == "" {
		return Hooks{}, false
	}
	if value, ok := registry.Load(key); ok {
		if hooks, ok := value.(Hooks); ok {
			return hooks, true
		}
	}
	return Hooks{}, false
}
