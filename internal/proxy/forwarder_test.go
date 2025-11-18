package proxy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/server"
)

const requestIDKey = "_anyhub_request_id"

func TestForwarderMissingHandler(t *testing.T) {
	app := fiber.New()
	defer app.Shutdown()

	ctx := app.AcquireCtx(new(fasthttp.RequestCtx))
	defer app.ReleaseCtx(ctx)
	ctx.Locals(requestIDKey, "missing-req")

	logger := logrus.New()
	logBuf := &bytes.Buffer{}
	logger.SetOutput(logBuf)

	forwarder := NewForwarder(nil, logger)
	route := testRouteWithModule("missing-module")

	if err := forwarder.Handle(ctx, route); err != nil {
		t.Fatalf("forwarder.Handle returned unexpected error: %v", err)
	}
	if status := ctx.Response().StatusCode(); status != fiber.StatusInternalServerError {
		t.Fatalf("expected 500 for missing handler, got %d", status)
	}
	if body := string(ctx.Response().Body()); !strings.Contains(body, "module_handler_missing") {
		t.Fatalf("expected error body to mention module_handler_missing, got %s", body)
	}
	if !strings.Contains(logBuf.String(), "module_handler_missing") {
		t.Fatalf("expected log to mention module_handler_missing, got %s", logBuf.String())
	}
	if got := string(ctx.Response().Header.Peek("X-Request-ID")); got != "missing-req" {
		t.Fatalf("expected request id header missing-req, got %s", got)
	}
	if !strings.Contains(logBuf.String(), "missing-req") {
		t.Fatalf("expected log to include request id, got %s", logBuf.String())
	}
}

func TestForwarderHandlerPanic(t *testing.T) {
	const moduleKey = "panic-module"
	moduleHandlers.Delete(normalizeModuleKey(moduleKey))
	defer moduleHandlers.Delete(normalizeModuleKey(moduleKey))

	MustRegisterModule(ModuleRegistration{
		Key: moduleKey,
		Handler: server.ProxyHandlerFunc(func(fiber.Ctx, *server.HubRoute) error {
			panic("boom")
		}),
	})

	app := fiber.New()
	defer app.Shutdown()
	ctx := app.AcquireCtx(new(fasthttp.RequestCtx))
	defer app.ReleaseCtx(ctx)
	ctx.Locals(requestIDKey, "panic-req")

	logger := logrus.New()
	logBuf := &bytes.Buffer{}
	logger.SetOutput(logBuf)

	forwarder := NewForwarder(nil, logger)
	route := testRouteWithModule(moduleKey)

	if err := forwarder.Handle(ctx, route); err != nil {
		t.Fatalf("forwarder.Handle returned unexpected error: %v", err)
	}
	if status := ctx.Response().StatusCode(); status != fiber.StatusInternalServerError {
		t.Fatalf("expected 500 for handler panic, got %d", status)
	}
	if body := string(ctx.Response().Body()); !strings.Contains(body, "module_handler_panic") {
		t.Fatalf("expected error body to mention module_handler_panic, got %s", body)
	}
	if !strings.Contains(logBuf.String(), "module_handler_panic") {
		t.Fatalf("expected log to mention module_handler_panic, got %s", logBuf.String())
	}
	if got := string(ctx.Response().Header.Peek("X-Request-ID")); got != "panic-req" {
		t.Fatalf("expected request id header panic-req, got %s", got)
	}
	if !strings.Contains(logBuf.String(), "panic-req") {
		t.Fatalf("expected log to include panic request id, got %s", logBuf.String())
	}
}

func testRouteWithModule(moduleKey string) *server.HubRoute {
	return &server.HubRoute{
		Config: config.HubConfig{
			Name:   "test",
			Domain: "test.local",
			Type:   "custom",
		},
		ModuleKey: moduleKey,
	}
}
