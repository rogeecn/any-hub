package hubmodule

import (
	"time"

	"github.com/any-hub/any-hub/internal/cache"
)

// MigrationState 描述模块上线阶段，方便观测端区分 legacy/beta/ga。
type MigrationState string

const (
	MigrationStateLegacy MigrationState = "legacy"
	MigrationStateBeta   MigrationState = "beta"
	MigrationStateGA     MigrationState = "ga"
)

// ValidationMode 描述缓存再验证的默认策略。
type ValidationMode string

const (
	ValidationModeETag         ValidationMode = "etag"
	ValidationModeLastModified ValidationMode = "last-modified"
	ValidationModeNever        ValidationMode = "never"
)

// CacheStrategyProfile 描述模块的缓存读写策略及其默认值。
type CacheStrategyProfile struct {
	TTLHint                time.Duration
	ValidationMode         ValidationMode
	DiskLayout             string
	RequiresMetadataFile   bool
	SupportsStreamingWrite bool
}

// ModuleMetadata 记录一个模块的静态信息，供配置校验和诊断端使用。
type ModuleMetadata struct {
	Key                string
	Description        string
	MigrationState     MigrationState
	SupportedProtocols []string
	CacheStrategy      CacheStrategyProfile
	LocatorRewrite     LocatorRewrite
}

// DefaultModuleKey 返回内置 legacy 模块的键值。
func DefaultModuleKey() string {
	return defaultModuleKey
}

// LocatorRewrite 允许模块根据自身协议调整缓存路径，例如将 npm metadata 写入独立文件。
type LocatorRewrite func(cache.Locator) cache.Locator
