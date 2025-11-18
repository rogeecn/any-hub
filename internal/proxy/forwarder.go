package proxy

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/logging"
	"github.com/any-hub/any-hub/internal/server"
)

// Forwarder 根据 HubRoute 的 module_key 选择对应的 ProxyHandler，默认回退到构造时注入的 handler。
type Forwarder struct {
	defaultHandler server.ProxyHandler
	logger         *logrus.Logger
}

// NewForwarder 创建 Forwarder，defaultHandler 不能为空。
func NewForwarder(defaultHandler server.ProxyHandler, logger *logrus.Logger) *Forwarder {
	return &Forwarder{
		defaultHandler: defaultHandler,
		logger:         logger,
	}
}

var (
	moduleHandlers sync.Map
)

// RegisterModuleHandler is kept for backward compatibility; it panics on invalid input.
func RegisterModuleHandler(key string, handler server.ProxyHandler) {
	MustRegisterModule(ModuleRegistration{Key: key, Handler: handler})
}

// Handle 实现 server.ProxyHandler，根据 route.ModuleKey 选择 handler。
func (f *Forwarder) Handle(c fiber.Ctx, route *server.HubRoute) error {
	requestID := server.RequestID(c)
	handler := f.lookup(route)
	if handler == nil {
		return f.respondMissingHandler(c, route, requestID)
	}
	return f.invokeHandler(c, route, handler, requestID)
}

func (f *Forwarder) respondMissingHandler(c fiber.Ctx, route *server.HubRoute, requestID string) error {
	f.logModuleError(route, "module_handler_missing", nil, requestID)
	setRequestIDHeader(c, requestID)
	return c.Status(fiber.StatusInternalServerError).
		JSON(fiber.Map{"error": "module_handler_missing"})
}

func (f *Forwarder) invokeHandler(c fiber.Ctx, route *server.HubRoute, handler server.ProxyHandler, requestID string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = f.respondHandlerPanic(c, route, r, requestID)
		}
	}()
	return handler.Handle(c, route)
}

func (f *Forwarder) respondHandlerPanic(c fiber.Ctx, route *server.HubRoute, recovered interface{}, requestID string) error {
	f.logModuleError(route, "module_handler_panic", fmt.Errorf("panic: %v", recovered), requestID)
	setRequestIDHeader(c, requestID)
	return c.Status(fiber.StatusInternalServerError).
		JSON(fiber.Map{"error": "module_handler_panic"})
}

func setRequestIDHeader(c fiber.Ctx, requestID string) {
	if requestID != "" {
		c.Set("X-Request-ID", requestID)
	}
}

func (f *Forwarder) logModuleError(route *server.HubRoute, code string, err error, requestID string) {
	if f.logger == nil {
		return
	}
	fields := f.routeFields(route, requestID)
	fields["action"] = "proxy"
	fields["error"] = code
	if err != nil {
		f.logger.WithFields(fields).Error(err.Error())
		return
	}
	f.logger.WithFields(fields).Error("module handler unavailable")
}

func (f *Forwarder) lookup(route *server.HubRoute) server.ProxyHandler {
	if route != nil {
		if handler := lookupModuleHandler(route.ModuleKey); handler != nil {
			return handler
		}
	}
	return f.defaultHandler
}

func lookupModuleHandler(key string) server.ProxyHandler {
	normalized := normalizeModuleKey(key)
	if normalized == "" {
		return nil
	}
	if value, ok := moduleHandlers.Load(normalized); ok {
		if handler, ok := value.(server.ProxyHandler); ok {
			return handler
		}
	}
	return nil
}

func normalizeModuleKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func (f *Forwarder) routeFields(route *server.HubRoute, requestID string) logrus.Fields {
	if route == nil {
		return logrus.Fields{
			"hub":        "",
			"domain":     "",
			"hub_type":   "",
			"auth_mode":  "",
			"cache_hit":  false,
			"module_key": "",
		}
	}

	fields := logging.RequestFields(
		route.Config.Name,
		route.Config.Domain,
		route.Config.Type,
		route.Config.AuthMode(),
		route.ModuleKey,
		false,
	)
	if requestID != "" {
		fields["request_id"] = requestID
	}
	return fields
}
