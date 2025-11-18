package hooks

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
	UpstreamHost string
	Method       string
}

// Hooks describes customization points for module-specific behavior.
type Hooks struct {
	NormalizePath   func(ctx *RequestContext, cleanPath string, rawQuery []byte) (string, []byte)
	ResolveUpstream func(ctx *RequestContext, baseURL string, path string, rawQuery []byte) string
	RewriteResponse func(ctx *RequestContext, status int, headers map[string]string, body []byte, path string) (int, map[string]string, []byte, error)
	CachePolicy     func(ctx *RequestContext, locatorPath string, current CachePolicy) CachePolicy
	ContentType     func(ctx *RequestContext, locatorPath string) string
}
