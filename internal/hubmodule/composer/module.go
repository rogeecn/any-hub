// Package composer declares metadata for Composer (PHP) package proxying.
package composer

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

const composerDefaultTTL = 6 * time.Hour

func init() {
	hubmodule.MustRegister(hubmodule.ModuleMetadata{
		Key:            "composer",
		Description:    "Composer packages proxy with metadata+dist caching",
		MigrationState: hubmodule.MigrationStateBeta,
		SupportedProtocols: []string{
			"composer",
		},
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:                composerDefaultTTL,
			ValidationMode:         hubmodule.ValidationModeETag,
			DiskLayout:             "raw_path",
			RequiresMetadataFile:   false,
			SupportsStreamingWrite: true,
		},
	})
}
