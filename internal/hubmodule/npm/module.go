// Package npm 描述 npm Registry 模块的默认策略与注册逻辑，方便新 Hub 直接启用。
package npm

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const npmDefaultTTL = 30 * time.Minute

// npm 模块描述 NPM Registry 的默认缓存策略，并允许通过 [[Hub]] 覆盖 TTL/Validation。
func init() {
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "npm",
		Description:    "NPM proxy module with cache strategy overrides for metadata/tarballs",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"npm",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                npmDefaultTTL,
			ValidationMode:         hubmodule.ValidationModeLastModified,
			DiskLayout:             "raw_path",
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true,
		},
		LocatorRewrite: hubmodule.DefaultLocatorRewrite("npm"),
	})
}
