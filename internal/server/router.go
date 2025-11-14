package server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ProxyHandler describes the component responsible for proxying requests to
// the upstream Hub. It allows injecting fake handlers during tests.
type ProxyHandler interface {
	Handle(fiber.Ctx, *HubRoute) error
}

// ProxyHandlerFunc adapts a function to the ProxyHandler interface.
type ProxyHandlerFunc func(fiber.Ctx, *HubRoute) error

// Handle makes ProxyHandlerFunc satisfy ProxyHandler.
func (f ProxyHandlerFunc) Handle(c fiber.Ctx, route *HubRoute) error {
	return f(c, route)
}

// AppOptions controls how the Fiber application should behave on a specific port.
type AppOptions struct {
	Logger     *logrus.Logger
	Registry   *HubRegistry
	Proxy      ProxyHandler
	ListenPort int
}

const (
	contextKeyRoute     = "_anyhub_route"
	contextKeyRequestID = "_anyhub_request_id"
)

// NewApp builds a Fiber application with Host/port routing middleware and
// structured error handling.
func NewApp(opts AppOptions) (*fiber.App, error) {
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if opts.Registry == nil {
		return nil, errors.New("hub registry is required")
	}
	if opts.Proxy == nil {
		return nil, errors.New("proxy handler is required")
	}
	if opts.ListenPort <= 0 {
		return nil, fmt.Errorf("invalid listen port: %d", opts.ListenPort)
	}

	app := fiber.New(fiber.Config{
		CaseSensitive: true,
	})

	app.Use(recover.New())
	app.Use(requestContextMiddleware(opts))

	app.All("/*", func(c fiber.Ctx) error {
		route, _ := getRouteFromContext(c)
		if route == nil {
			return renderHostUnmapped(c, opts.Logger, "", opts.ListenPort)
		}
		return opts.Proxy.Handle(c, route)
	})

	return app, nil
}

// requestContextMiddleware 负责生成请求 ID，并基于 Host/Host:port 查找 HubRoute。
func requestContextMiddleware(opts AppOptions) fiber.Handler {
	return func(c fiber.Ctx) error {
		reqID := uuid.NewString()
		c.Locals(contextKeyRequestID, reqID)
		c.Set("X-Request-ID", reqID)

		rawHost := strings.TrimSpace(getHostHeader(c))
		route, ok := opts.Registry.Lookup(rawHost)
		if !ok {
			return renderHostUnmapped(c, opts.Logger, rawHost, opts.ListenPort)
		}
		if err := ensureRouterHubType(route); err != nil {
			return renderTypeUnsupported(c, opts.Logger, route, err)
		}

		c.Locals(contextKeyRoute, route)
		return c.Next()
	}
}

func renderHostUnmapped(c fiber.Ctx, logger *logrus.Logger, host string, port int) error {
	fields := logrus.Fields{
		"action": "host_lookup",
		"host":   host,
		"port":   port,
	}
	logger.WithFields(fields).Warn("host unmapped")

	if host != "" {
		c.Set("X-Any-Hub-Host", host)
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": "host_unmapped",
	})
}

func getHostHeader(c fiber.Ctx) string {
	if raw := c.Request().Header.Peek(fiber.HeaderHost); len(raw) > 0 {
		return string(raw)
	}
	return c.Hostname()
}

func getRouteFromContext(c fiber.Ctx) (*HubRoute, bool) {
	if value := c.Locals(contextKeyRoute); value != nil {
		if route, ok := value.(*HubRoute); ok {
			return route, true
		}
	}
	return nil, false
}

// RequestID returns the request identifier stored by the router middleware.
func RequestID(c fiber.Ctx) string {
	if value := c.Locals(contextKeyRequestID); value != nil {
		if reqID, ok := value.(string); ok {
			return reqID
		}
	}
	return ""
}

func ensureRouterHubType(route *HubRoute) error {
	switch route.Config.Type {
	case "docker":
		return nil
	case "npm":
		return nil
	case "go":
		return nil
	case "pypi":
		return nil
	default:
		return fmt.Errorf("unsupported hub type: %s", route.Config.Type)
	}
}

func renderTypeUnsupported(c fiber.Ctx, logger *logrus.Logger, route *HubRoute, err error) error {
	fields := logrus.Fields{
		"action":   "hub_type_check",
		"hub":      route.Config.Name,
		"hub_type": route.Config.Type,
		"error":    "hub_type_unsupported",
	}
	logger.WithFields(fields).Error(err.Error())
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "hub_type_unsupported",
	})
}
