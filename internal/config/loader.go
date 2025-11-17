package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

// Load 读取并解析 TOML 配置文件，同时注入默认值与校验逻辑。
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.toml"
	}

	v := viper.New()
	v.SetConfigFile(path)
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	if err := rejectHubLevelPorts(v); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(durationDecodeHook())); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	applyGlobalDefaults(&cfg.Global)
	for i := range cfg.Hubs {
		applyHubDefaults(&cfg.Hubs[i])
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	absStorage, err := filepath.Abs(cfg.Global.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("无法解析缓存目录: %w", err)
	}
	cfg.Global.StoragePath = absStorage

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("ListenPort", 5000)
	v.SetDefault("LogLevel", "info")
	v.SetDefault("LogFilePath", "")
	v.SetDefault("LogMaxSize", 100)
	v.SetDefault("LogMaxBackups", 10)
	v.SetDefault("LogCompress", true)
	v.SetDefault("StoragePath", "./storage")
	v.SetDefault("CacheTTL", 86400)
	v.SetDefault("MaxMemoryCacheSize", 256*1024*1024)
	v.SetDefault("MaxRetries", 3)
	v.SetDefault("InitialBackoff", "1s")
	v.SetDefault("UpstreamTimeout", "30s")
}

func applyGlobalDefaults(g *GlobalConfig) {
	if g.ListenPort == 0 {
		g.ListenPort = 5000
	}
	if g.CacheTTL.DurationValue() == 0 {
		g.CacheTTL = Duration(24 * time.Hour)
	}
	if g.InitialBackoff.DurationValue() == 0 {
		g.InitialBackoff = Duration(time.Second)
	}
	if g.UpstreamTimeout.DurationValue() == 0 {
		g.UpstreamTimeout = Duration(30 * time.Second)
	}
}

func applyHubDefaults(h *HubConfig) {
	if h.CacheTTL.DurationValue() < 0 {
		h.CacheTTL = Duration(0)
	}
	if trimmed := strings.TrimSpace(h.Module); trimmed == "" {
		typeKey := strings.ToLower(strings.TrimSpace(h.Type))
		if meta, ok := hubmodule.Resolve(typeKey); ok {
			h.Module = meta.Key
		} else {
			h.Module = hubmodule.DefaultModuleKey()
		}
	} else {
		h.Module = strings.ToLower(trimmed)
	}
	if rollout := strings.TrimSpace(h.Rollout); rollout != "" {
		h.Rollout = strings.ToLower(rollout)
	}
	if h.ValidationMode == "" {
		h.ValidationMode = string(hubmodule.ValidationModeETag)
	}
}

func durationDecodeHook() mapstructure.DecodeHookFunc {
	targetType := reflect.TypeOf(Duration(0))

	return func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if to != targetType {
			return data, nil
		}

		switch v := data.(type) {
		case string:
			if v == "" {
				return Duration(0), nil
			}
			if parsed, err := time.ParseDuration(v); err == nil {
				return Duration(parsed), nil
			}
			if seconds, err := strconv.ParseFloat(v, 64); err == nil {
				return Duration(time.Duration(seconds * float64(time.Second))), nil
			}
			return nil, fmt.Errorf("无法解析 Duration 字段: %s", v)
		case int:
			return Duration(time.Duration(v) * time.Second), nil
		case int64:
			return Duration(time.Duration(v) * time.Second), nil
		case float64:
			return Duration(time.Duration(v * float64(time.Second))), nil
		case time.Duration:
			return Duration(v), nil
		case Duration:
			return v, nil
		default:
			return nil, fmt.Errorf("不支持的 Duration 类型: %T", v)
		}
	}
}

func rejectHubLevelPorts(v *viper.Viper) error {
	raw := v.Get("Hub")
	hubs, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	for idx, entry := range hubs {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if _, exists := m["Port"]; exists {
			name := fmt.Sprintf("#%d", idx)
			if rawName, ok := m["Name"].(string); ok && rawName != "" {
				name = rawName
			}
			return newFieldError(hubField(name, "Port"), "字段已弃用，请移除并使用全局 ListenPort")
		}
	}

	return nil
}
