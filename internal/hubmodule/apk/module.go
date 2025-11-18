// Package apk registers metadata for Alpine APK proxying.
package apk

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const apkDefaultTTL = 6 * time.Hour

func init() {
	// 模块元数据声明，具体 hooks 见 hooks.go（已在 init 自动注册）。
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "apk",
		Description:    "Alpine APK proxy with cached indexes and packages",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"apk",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                0,                                    // APKINDEX 每次再验证，包体直接命中
			ValidationMode:         hubmodule.ValidationModeLastModified, // APKINDEX 再验证
			DiskLayout:             "raw_path",
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true, // 包体流式写
		},
	})
}
