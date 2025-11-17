package proxy

import (
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/proxy/hooks"
	"github.com/any-hub/any-hub/internal/server"
)

func TestResolveUpstreamPrefersHook(t *testing.T) {
	app := fiber.New()
	defer app.Shutdown()

	ctx := app.AcquireCtx(new(fasthttp.RequestCtx))
	defer app.ReleaseCtx(ctx)
	ctx.Request().SetRequestURI("/original/path?from=req")

	base, _ := url.Parse("https://up.example")
	route := &server.HubRoute{
		Config: config.HubConfig{
			Name: "demo",
			Type: "custom",
		},
		UpstreamURL: base,
	}
	hook := &hookState{
		ctx: &hooks.RequestContext{},
		def: hooks.Hooks{
			NormalizePath: func(_ *hooks.RequestContext, clean string, rawQuery []byte) (string, []byte) {
				return clean, rawQuery
			},
			ResolveUpstream: func(_ *hooks.RequestContext, upstream string, clean string, rawQuery []byte) string {
				return upstream + "/hooked"
			},
		},
		hasHooks: true,
		clean:    "/ignored",
		rawQuery: []byte("ignored=1"),
	}

	target := resolveUpstreamURL(route, base, ctx, hook)
	if target.String() != "https://up.example/hooked" {
		t.Fatalf("expected hook override, got %s", target.String())
	}
}

func TestCachePolicyHookOverrides(t *testing.T) {
	route := &server.HubRoute{
		Config: config.HubConfig{
			Name: "demo",
			Type: "npm",
		},
	}
	locator := cacheLocatorForTest("demo", "/a.tgz")
	hook := hooks.Hooks{
		CachePolicy: func(_ *hooks.RequestContext, _ string, current hooks.CachePolicy) hooks.CachePolicy {
			current.AllowCache = false
			current.RequireRevalidate = false
			return current
		},
	}
	ctx := &hooks.RequestContext{Method: fiber.MethodGet}
	policy := determineCachePolicyWithHook(route, locator, fiber.MethodGet, hook, true, ctx)
	if policy.allowCache {
		t.Fatalf("expected hook to disable cache")
	}
	if policy.requireRevalidate {
		t.Fatalf("expected hook to disable revalidate")
	}
}

func cacheLocatorForTest(hub, path string) cache.Locator {
	return cache.Locator{HubName: hub, Path: path}
}
