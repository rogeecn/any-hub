package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

var supportedHubTypes = map[string]struct{}{
	"docker":   {},
	"npm":      {},
	"go":       {},
	"pypi":     {},
	"composer": {},
	"debian":   {},
	"apk":      {},
}

const supportedHubTypeList = "docker|npm|go|pypi|composer|debian|apk"

// Validate 针对语义级别做进一步校验，防止非法配置启动服务。
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("配置为空")
	}

	g := c.Global
	if g.ListenPort <= 0 || g.ListenPort > 65535 {
		return newFieldError("Global.ListenPort", "必须在 1-65535")
	}
	if g.StoragePath == "" {
		return newFieldError("Global.StoragePath", "不能为空")
	}
	if g.CacheTTL.DurationValue() <= 0 {
		return newFieldError("Global.CacheTTL", "必须大于 0")
	}
	if g.MaxMemoryCache <= 0 {
		return newFieldError("Global.MaxMemoryCacheSize", "必须大于 0")
	}
	if g.MaxRetries < 0 {
		return newFieldError("Global.MaxRetries", "不能为负数")
	}
	if g.InitialBackoff.DurationValue() <= 0 {
		return newFieldError("Global.InitialBackoff", "必须大于 0")
	}
	if g.UpstreamTimeout.DurationValue() <= 0 {
		return newFieldError("Global.UpstreamTimeout", "必须大于 0")
	}

	if len(c.Hubs) == 0 {
		return errors.New("至少需要配置一个 Hub")
	}

	seenNames := map[string]struct{}{}
	for i := range c.Hubs {
		hub := &c.Hubs[i]
		if hub.Name == "" {
			return newFieldError("Hub[].Name", "不能为空")
		}
		if _, exists := seenNames[hub.Name]; exists {
			return newFieldError(hubField(hub.Name, "Name"), "重复")
		}
		seenNames[hub.Name] = struct{}{}

		if err := validateDomain(hub.Domain); err != nil {
			return fmt.Errorf("%s: %w", hubField(hub.Name, "Domain"), err)
		}

		normalizedType := strings.ToLower(strings.TrimSpace(hub.Type))
		if normalizedType == "" {
			return newFieldError(hubField(hub.Name, "Type"), "不能为空")
		}
		if _, ok := supportedHubTypes[normalizedType]; !ok {
			return newFieldError(hubField(hub.Name, "Type"), "仅支持 "+supportedHubTypeList)
		}
		hub.Type = normalizedType

		if _, ok := hubmodule.Resolve(normalizedType); !ok {
			return newFieldError(hubField(hub.Name, "Type"), fmt.Sprintf("未注册模块: %s", normalizedType))
		}
		if hub.ValidationMode != "" {
			mode := strings.ToLower(strings.TrimSpace(hub.ValidationMode))
			switch mode {
			case string(hubmodule.ValidationModeETag), string(hubmodule.ValidationModeLastModified), string(hubmodule.ValidationModeNever):
				hub.ValidationMode = mode
			default:
				return newFieldError(hubField(hub.Name, "ValidationMode"), "仅支持 etag/last-modified/never")
			}
		}

		if (hub.Username == "") != (hub.Password == "") {
			return newFieldError(hubField(hub.Name, "Username/Password"), "必须同时提供或同时留空")
		}
		if err := validateUpstream(hub.Upstream); err != nil {
			return fmt.Errorf("%s: %w", hubField(hub.Name, "Upstream"), err)
		}
		if hub.Proxy != "" {
			if err := validateUpstream(hub.Proxy); err != nil {
				return fmt.Errorf("%s: %w", hubField(hub.Name, "Proxy"), err)
			}
		}
	}

	return nil
}

func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("Domain 不能为空")
	}
	if strings.Contains(domain, "/") {
		return errors.New("Domain 不允许包含路径")
	}
	if strings.Contains(domain, " ") {
		return errors.New("Domain 不允许包含空格")
	}
	if strings.HasPrefix(domain, "http") {
		return errors.New("Domain 不应包含协议头")
	}
	return nil
}

func validateUpstream(raw string) error {
	if raw == "" {
		return errors.New("缺少上游地址")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https，上游: %s", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("上游缺少 Host: %s", raw)
	}
	return nil
}

// EffectiveCacheTTL 返回特定 Hub 生效的 TTL，未覆盖时回退至全局值。
func (c *Config) EffectiveCacheTTL(h HubConfig) time.Duration {
	if h.CacheTTL.DurationValue() > 0 {
		return h.CacheTTL.DurationValue()
	}
	return c.Global.CacheTTL.DurationValue()
}
