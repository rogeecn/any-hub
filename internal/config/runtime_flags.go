package config

import (
	"fmt"
	"strings"

	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/hubmodule/legacy"
)

// parseRolloutFlag 将配置中的 rollout 字段标准化，并结合模块类型输出最终状态。
func parseRolloutFlag(raw string, moduleKey string) (legacy.RolloutFlag, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return defaultRolloutFlag(moduleKey), nil
	}

	switch normalized {
	case string(legacy.RolloutLegacyOnly):
		return legacy.RolloutLegacyOnly, nil
	case string(legacy.RolloutDual):
		if moduleKey == hubmodule.DefaultModuleKey() {
			return legacy.RolloutLegacyOnly, nil
		}
		return legacy.RolloutDual, nil
	case string(legacy.RolloutModular):
		if moduleKey == hubmodule.DefaultModuleKey() {
			return legacy.RolloutLegacyOnly, nil
		}
		return legacy.RolloutModular, nil
	default:
		return "", fmt.Errorf("不支持的 rollout 值: %s", raw)
	}
}

func defaultRolloutFlag(moduleKey string) legacy.RolloutFlag {
	if strings.TrimSpace(moduleKey) == "" || moduleKey == hubmodule.DefaultModuleKey() {
		return legacy.RolloutLegacyOnly
	}
	return legacy.RolloutModular
}

// EffectiveModuleKey 根据 rollout 状态计算真实运行的模块。
func EffectiveModuleKey(moduleKey string, flag legacy.RolloutFlag) string {
	if flag == legacy.RolloutLegacyOnly {
		return hubmodule.DefaultModuleKey()
	}
	normalized := strings.ToLower(strings.TrimSpace(moduleKey))
	if normalized == "" {
		return hubmodule.DefaultModuleKey()
	}
	return normalized
}

// RolloutFlagValue 返回当前 Hub 的 rollout flag（假定 Validate 已经通过）。
func (h HubConfig) RolloutFlagValue() legacy.RolloutFlag {
	flag := legacy.RolloutFlag(strings.ToLower(strings.TrimSpace(h.Rollout)))
	if flag == "" {
		return defaultRolloutFlag(h.Module)
	}
	return flag
}
