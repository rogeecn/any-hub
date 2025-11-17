// Package legacy 提供旧版共享代理+缓存实现的适配器，确保未迁移 Hub 可继续运行。
package legacy

import "github.com/any-hub/any-hub/internal/hubmodule"

// 模块描述：包装当前共享的代理 + 缓存实现，供未迁移的 Hub 使用，并在 diagnostics 中标记为 legacy-only。
func init() {
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            hubmodule.DefaultModuleKey(),
		Description:    "Legacy proxy + cache implementation bundled with any-hub",
		MigrationState: hubmodule.MigrationStateLegacy,
		SupportedProtocols: []string{
			"docker", "npm", "go", "pypi",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			DiskLayout:             "raw_path",
			ValidationMode:         hubmodule.ValidationModeETag,
			SupportsStreamingWrite: true,
		},
	})
}
