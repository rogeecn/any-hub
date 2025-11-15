package config

import (
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/hubmodule/legacy"
)

// HubRuntime 将 Hub 配置与模块元数据合并，方便运行时快速取用策略。
type HubRuntime struct {
	Config        HubConfig
	Module        hubmodule.ModuleMetadata
	CacheStrategy hubmodule.CacheStrategyProfile
	Rollout       legacy.RolloutFlag
}

// BuildHubRuntime 根据 Hub 配置和模块元数据创建运行时描述，应用最终 TTL 覆盖。
func BuildHubRuntime(cfg HubConfig, meta hubmodule.ModuleMetadata, ttl time.Duration, flag legacy.RolloutFlag) HubRuntime {
	strategy := hubmodule.ResolveStrategy(meta, cfg.StrategyOverrides(ttl))
	return HubRuntime{
		Config:        cfg,
		Module:        meta,
		CacheStrategy: strategy,
		Rollout:       flag,
	}
}
