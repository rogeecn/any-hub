package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

// Duration 提供更灵活的反序列化能力，同时兼容纯秒整数与 Go Duration 字符串。
type Duration time.Duration

// UnmarshalText 使 Viper 可以识别诸如 "30s"、"5m" 或纯数字秒值等配置写法。
func (d *Duration) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		*d = Duration(0)
		return nil
	}

	if seconds, err := time.ParseDuration(raw); err == nil {
		*d = Duration(seconds)
		return nil
	}

	if intVal, err := parseInt(raw); err == nil {
		*d = Duration(time.Duration(intVal) * time.Second)
		return nil
	}

	return fmt.Errorf("invalid duration value: %s", raw)
}

// DurationValue 返回真实的 time.Duration，便于调用方计算。
func (d Duration) DurationValue() time.Duration {
	return time.Duration(d)
}

// parseInt 支持十进制或 0x 前缀的十六进制字符串解析。
func parseInt(value string) (int64, error) {
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		return strconv.ParseInt(value, 0, 64)
	}
	return strconv.ParseInt(value, 10, 64)
}

// GlobalConfig 描述全局运行时行为，所有 Hub 共享同一份参数。
type GlobalConfig struct {
	ListenPort      int      `mapstructure:"ListenPort"`
	LogLevel        string   `mapstructure:"LogLevel"`
	LogFilePath     string   `mapstructure:"LogFilePath"`
	LogMaxSize      int      `mapstructure:"LogMaxSize"`
	LogMaxBackups   int      `mapstructure:"LogMaxBackups"`
	LogCompress     bool     `mapstructure:"LogCompress"`
	StoragePath     string   `mapstructure:"StoragePath"`
	CacheTTL        Duration `mapstructure:"CacheTTL"`
	MaxMemoryCache  int64    `mapstructure:"MaxMemoryCacheSize"`
	MaxRetries      int      `mapstructure:"MaxRetries"`
	InitialBackoff  Duration `mapstructure:"InitialBackoff"`
	UpstreamTimeout Duration `mapstructure:"UpstreamTimeout"`
}

// HubConfig 决定单个代理实例如何与下游/上游交互。
type HubConfig struct {
	Name            string   `mapstructure:"Name"`
	Domain          string   `mapstructure:"Domain"`
	Upstream        string   `mapstructure:"Upstream"`
	Proxy           string   `mapstructure:"Proxy"`
	Type            string   `mapstructure:"Type"`
	Username        string   `mapstructure:"Username"`
	Password        string   `mapstructure:"Password"`
	CacheTTL        Duration `mapstructure:"CacheTTL"`
	ValidationMode  string   `mapstructure:"ValidationMode"`
	EnableHeadCheck bool     `mapstructure:"EnableHeadCheck"`
}

// Config 是 TOML 文件映射的整体结构。
type Config struct {
	Global GlobalConfig `mapstructure:",squash"`
	Hubs   []HubConfig  `mapstructure:"Hub"`
}

// HasCredentials 表示当前 Hub 是否配置了完整的上游凭证。
func (h HubConfig) HasCredentials() bool {
	return h.Username != "" && h.Password != ""
}

// AuthMode 输出 `credentialed` 或 `anonymous`，供日志字段使用。
func (h HubConfig) AuthMode() string {
	if h.HasCredentials() {
		return "credentialed"
	}
	return "anonymous"
}

// CredentialModes 返回所有 Hub 的鉴权模式摘要，例如 secure:credentialed。
func CredentialModes(hubs []HubConfig) []string {
	if len(hubs) == 0 {
		return nil
	}
	result := make([]string, len(hubs))
	for i, hub := range hubs {
		result[i] = fmt.Sprintf("%s:%s", hub.Name, hub.AuthMode())
	}
	return result
}

// StrategyOverrides 将 hub 层的 TTL/Validation 配置映射为模块策略覆盖项。
func (h HubConfig) StrategyOverrides(ttl time.Duration) hubmodule.StrategyOptions {
	opts := hubmodule.StrategyOptions{
		TTLOverride: ttl,
	}
	if mode := strings.TrimSpace(h.ValidationMode); mode != "" {
		opts.ValidationOverride = hubmodule.ValidationMode(mode)
	}
	return opts
}
