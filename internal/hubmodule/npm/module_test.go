package npm

import (
	"testing"
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
)

func TestNPMMetadataRegistration(t *testing.T) {
	meta, ok := hubmodule.Resolve("npm")
	if !ok {
		t.Fatalf("npm module not registered")
	}
	if meta.Key != "npm" {
		t.Fatalf("unexpected module key: %s", meta.Key)
	}
	if meta.MigrationState == "" {
		t.Fatalf("migration state must be set")
	}
	if len(meta.SupportedProtocols) == 0 {
		t.Fatalf("supported protocols must not be empty")
	}
	if meta.CacheStrategy.TTLHint != npmDefaultTTL {
		t.Fatalf("expected default ttl %s, got %s", npmDefaultTTL, meta.CacheStrategy.TTLHint)
	}
	if meta.CacheStrategy.ValidationMode != hubmodule.ValidationModeLastModified {
		t.Fatalf("expected validation mode last-modified, got %s", meta.CacheStrategy.ValidationMode)
	}
	if !meta.CacheStrategy.SupportsStreamingWrite {
		t.Fatalf("npm strategy should support streaming writes")
	}
}

func TestNPMStrategyOverrides(t *testing.T) {
	meta, ok := hubmodule.Resolve("npm")
	if !ok {
		t.Fatalf("npm module not registered")
	}

	overrideTTL := 10 * time.Minute
	strategy := hubmodule.ResolveStrategy(meta, hubmodule.StrategyOptions{
		TTLOverride:        overrideTTL,
		ValidationOverride: hubmodule.ValidationModeETag,
	})
	if strategy.TTLHint != overrideTTL {
		t.Fatalf("expected ttl override %s, got %s", overrideTTL, strategy.TTLHint)
	}
	if strategy.ValidationMode != hubmodule.ValidationModeETag {
		t.Fatalf("expected validation mode override to etag, got %s", strategy.ValidationMode)
	}
}
