package golang

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const goDefaultTTL = 30 * time.Minute

func init() {
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "go",
		Description:    "Go module proxy with sumdb/cache defaults",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"go",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                goDefaultTTL,
			ValidationMode:         hubmodule.ValidationModeLastModified,
			DiskLayout:             "raw_path",
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true,
		},
		LocatorRewrite: hubmodule.DefaultLocatorRewrite("go"),
	})
}
