// Package docker 定义 Docker Hub 代理模块的元数据与缓存策略描述，供 registry 查表时使用。
package docker

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const dockerDefaultTTL = 12 * time.Hour

// docker 模块继承 legacy 行为，但声明明确的缓存策略默认值，便于 hub 覆盖。
func init() {
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "docker",
		Description:    "Docker registry module with manifest/blob cache policies",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"docker",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                dockerDefaultTTL,
			ValidationMode:         hubmodule.ValidationModeETag,
			DiskLayout:             "raw_path",
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true,
		},
	})
}
