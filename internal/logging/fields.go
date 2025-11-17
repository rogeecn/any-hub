package logging

import "github.com/sirupsen/logrus"

// BaseFields 构建 action + 配置路径等基础字段，便于不同入口复用。
func BaseFields(action, configPath string) logrus.Fields {
	return logrus.Fields{
		"action":     action,
		"configPath": configPath,
	}
}

// RequestFields 提供 hub/domain/命中状态字段，供代理请求日志复用。
func RequestFields(hub, domain, hubType, authMode, moduleKey, rolloutFlag string, cacheHit bool, legacyOnly bool) logrus.Fields {
	return logrus.Fields{
		"hub":          hub,
		"domain":       domain,
		"hub_type":     hubType,
		"auth_mode":    authMode,
		"cache_hit":    cacheHit,
		"legacy_only":  legacyOnly,
		"module_key":   moduleKey,
		"rollout_flag": rolloutFlag,
	}
}
