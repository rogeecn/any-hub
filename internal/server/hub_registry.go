package server

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/hubmodule/legacy"
)

// HubRoute 将 Hub 配置与派生属性（如缓存 TTL、解析后的 Upstream/Proxy URL）
// 聚合在一起，供路由/代理层直接复用，避免重复解析配置。
type HubRoute struct {
	// Config 是用户在 config.toml 中声明的 Hub 字段副本，避免外部修改。
	Config config.HubConfig
	// ListenPort 记录当前 CLI 监听端口，方便日志/转发头输出。
	ListenPort int
	// CacheTTL 是对当前 Hub 生效的 TTL，若 Hub 未覆盖则等于全局值。
	CacheTTL time.Duration
	// UpstreamURL/ProxyURL 在构造 Registry 时提前解析完成，便于后续请求快速复用。
	UpstreamURL *url.URL
	ProxyURL    *url.URL
	// ModuleKey/Module 记录当前 hub 选用的模块及其元数据，便于日志与观测。
	ModuleKey string
	Module    hubmodule.ModuleMetadata
	// CacheStrategy 代表模块默认策略与 hub 覆盖后的最终结果。
	CacheStrategy hubmodule.CacheStrategyProfile
	// RolloutFlag 反映当前 hub 的 legacy → modular 迁移状态，供日志/诊断使用。
	RolloutFlag legacy.RolloutFlag
}

// HubRegistry 提供 Host/Host:port 到 HubRoute 的查询能力，所有 Hub 共享同一个监听端口。
type HubRegistry struct {
	routes  map[string]*HubRoute
	ordered []*HubRoute
}

// NewHubRegistry 根据配置构建 Host/端口映射。调用方应在启动阶段创建一次并复用。
func NewHubRegistry(cfg *config.Config) (*HubRegistry, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	registry := &HubRegistry{
		routes: make(map[string]*HubRoute, len(cfg.Hubs)),
	}

	if len(cfg.Hubs) == 0 {
		return registry, nil
	}

	for _, hub := range cfg.Hubs {
		normalizedHost := normalizeDomain(hub.Domain)
		if normalizedHost == "" {
			return nil, fmt.Errorf("invalid domain for hub %s", hub.Name)
		}
		if _, exists := registry.routes[normalizedHost]; exists {
			return nil, fmt.Errorf("duplicate domain mapping detected for %s", normalizedHost)
		}

		route, err := buildHubRoute(cfg, hub)
		if err != nil {
			return nil, err
		}

		registry.routes[normalizedHost] = route
		registry.ordered = append(registry.ordered, route)
	}

	return registry, nil
}

// Lookup 根据 Host 或 Host:port 查找 HubRoute。
func (r *HubRegistry) Lookup(host string) (*HubRoute, bool) {
	if r == nil {
		return nil, false
	}

	normalizedHost, _ := normalizeHost(host)
	if normalizedHost == "" {
		return nil, false
	}

	route, ok := r.routes[normalizedHost]
	return route, ok
}

// List 返回当前注册的 HubRoute 列表（按配置定义的顺序），用于调试或 /status 输出。
func (r *HubRegistry) List() []HubRoute {
	if r == nil || len(r.ordered) == 0 {
		return nil
	}

	result := make([]HubRoute, len(r.ordered))
	for i, route := range r.ordered {
		result[i] = *route
	}
	return result
}

func buildHubRoute(cfg *config.Config, hub config.HubConfig) (*HubRoute, error) {
	flag := hub.RolloutFlagValue()
	effectiveKey := config.EffectiveModuleKey(hub.Module, flag)
	meta, err := moduleMetadataForKey(effectiveKey)
	if err != nil {
		return nil, fmt.Errorf("hub %s: %w", hub.Name, err)
	}

	upstreamURL, err := url.Parse(hub.Upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream for hub %s: %w", hub.Name, err)
	}

	var proxyURL *url.URL
	if hub.Proxy != "" {
		proxyURL, err = url.Parse(hub.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy for hub %s: %w", hub.Name, err)
		}
	}

	effectiveTTL := cfg.EffectiveCacheTTL(hub)
	runtime := config.BuildHubRuntime(hub, meta, effectiveTTL, flag)
	legacy.RecordAdapterState(hub.Name, runtime.Module.Key, flag)

	return &HubRoute{
		Config:        hub,
		ListenPort:    cfg.Global.ListenPort,
		CacheTTL:      effectiveTTL,
		UpstreamURL:   upstreamURL,
		ProxyURL:      proxyURL,
		ModuleKey:     runtime.Module.Key,
		Module:        runtime.Module,
		CacheStrategy: runtime.CacheStrategy,
		RolloutFlag:   runtime.Rollout,
	}, nil
}

func normalizeDomain(domain string) string {
	host, _ := normalizeHost(domain)
	return host
}

func normalizeHost(raw string) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}

	host := raw
	port := 0

	if strings.Contains(raw, ":") {
		if h, p, err := net.SplitHostPort(raw); err == nil {
			host = h
			if parsedPort, err := strconv.Atoi(p); err == nil {
				port = parsedPort
			}
		} else if idx := strings.LastIndex(raw, ":"); idx > -1 && strings.Count(raw[idx+1:], ":") == 0 {
			if parsedPort, err := strconv.Atoi(raw[idx+1:]); err == nil {
				host = raw[:idx]
				port = parsedPort
			}
		}
	}

	host = strings.TrimSuffix(host, ".")
	host = strings.ToLower(host)
	return host, port
}
