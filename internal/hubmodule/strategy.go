package hubmodule

import "time"

// StrategyOptions 描述来自 Hub Config 的 override。
type StrategyOptions struct {
	TTLOverride        time.Duration
	ValidationOverride ValidationMode
}

// ResolveStrategy 将模块的默认策略与 hub 级覆盖合并。
func ResolveStrategy(meta ModuleMetadata, opts StrategyOptions) CacheStrategyProfile {
	strategy := meta.CacheStrategy
	if opts.TTLOverride > 0 {
		strategy.TTLHint = opts.TTLOverride
	}
	if opts.ValidationOverride != "" {
		strategy.ValidationMode = opts.ValidationOverride
	}
	return normalizeStrategy(strategy)
}

func normalizeStrategy(profile CacheStrategyProfile) CacheStrategyProfile {
	if profile.TTLHint < 0 {
		profile.TTLHint = 0
	}
	if profile.ValidationMode == "" {
		profile.ValidationMode = ValidationModeETag
	}
	if profile.DiskLayout == "" {
		profile.DiskLayout = "raw_path"
	}
	return profile
}
