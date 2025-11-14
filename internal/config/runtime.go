package config

import (
	"github.com/any-hub/any-hub/internal/hubmodule"
)

// HubRuntime 将 Hub 配置与模块元数据合并，方便运行时快速取用策略。
type HubRuntime struct {
	Config        HubConfig
	Module        hubmodule.ModuleMetadata
	CacheStrategy hubmodule.CacheStrategyProfile
}

// BuildHubRuntime 根据 Hub 配置和模块元数据创建运行时描述。
func BuildHubRuntime(cfg HubConfig, meta hubmodule.ModuleMetadata) HubRuntime {
	strategy := hubmodule.ResolveStrategy(meta, hubmodule.StrategyOptions{
		TTLOverride:        cfg.CacheTTL.DurationValue(),
		ValidationOverride: hubmodule.ValidationMode(cfg.ValidationMode),
	})
	return HubRuntime{
		Config:        cfg,
		Module:        meta,
		CacheStrategy: strategy,
	}
}
