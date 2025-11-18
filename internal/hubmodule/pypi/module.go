// Package pypi 聚焦 PyPI simple index 模块，提供 TTL/验证策略的注册样例。
package pypi

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const pypiDefaultTTL = 15 * time.Minute

// pypi 模块负责 simple index + 分发包的策略声明，默认使用 Last-Modified 校验。
func init() {
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "pypi",
		Description:    "PyPI simple index module with per-hub cache overrides",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"pypi",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                0, // simple index 每次再验证
			ValidationMode:         hubmodule.ValidationModeLastModified,
			DiskLayout:             "raw_path",
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true,
		},
	})
}
