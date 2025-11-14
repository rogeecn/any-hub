package proxy

import (
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"

	"github.com/any-hub/any-hub/internal/server"
)

// Forwarder 根据 HubRoute 的 module_key 选择对应的 ProxyHandler，默认回退到构造时注入的 handler。
type Forwarder struct {
	defaultHandler server.ProxyHandler
}

// NewForwarder 创建 Forwarder，defaultHandler 不能为空。
func NewForwarder(defaultHandler server.ProxyHandler) *Forwarder {
	return &Forwarder{defaultHandler: defaultHandler}
}

var (
	moduleHandlers sync.Map
)

// RegisterModuleHandler 将特定 module_key 映射到 ProxyHandler，重复注册会覆盖旧值。
func RegisterModuleHandler(key string, handler server.ProxyHandler) {
	normalized := normalizeModuleKey(key)
	if normalized == "" || handler == nil {
		return
	}
	moduleHandlers.Store(normalized, handler)
}

// Handle 实现 server.ProxyHandler，根据 route.ModuleKey 选择 handler。
func (f *Forwarder) Handle(c fiber.Ctx, route *server.HubRoute) error {
	handler := f.lookup(route)
	if handler == nil {
		return fiber.NewError(fiber.StatusInternalServerError, "proxy handler unavailable")
	}
	return handler.Handle(c, route)
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
