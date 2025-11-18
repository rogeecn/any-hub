// Package debian registers metadata for Debian/Ubuntu APT proxying.
package debian

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const debianDefaultTTL = 6 * time.Hour

func init() {
	// 仅声明模块元数据（缓存策略等）；具体 hooks 在后续实现。
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "debian",
		Description:    "APT proxy with cached indexes and packages",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"debian",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                0,                                    // 索引每次再验证，由 ETag/Last-Modified 控制
			ValidationMode:         hubmodule.ValidationModeLastModified, // 索引使用 Last-Modified/ETag 再验证
			DiskLayout:             "raw_path",                           // 复用通用原始路径布局
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true, // 包体需要流式写入，避免大文件占用内存
		},
	})
}
