package legacy

import (
	"sort"
	"strings"
	"sync"
)

// RolloutFlag 描述 legacy 模块迁移阶段。
type RolloutFlag string

const (
	RolloutLegacyOnly RolloutFlag = "legacy-only"
	RolloutDual       RolloutFlag = "dual"
	RolloutModular    RolloutFlag = "modular"
)

// AdapterState 记录特定 Hub 在 legacy 适配器中的运行状态。
type AdapterState struct {
	HubName   string
	ModuleKey string
	Rollout   RolloutFlag
}

var (
	stateMu sync.RWMutex
	state   = make(map[string]AdapterState)
)

// RecordAdapterState 更新指定 Hub 的 rollout 状态，供诊断端和日志使用。
func RecordAdapterState(hubName, moduleKey string, flag RolloutFlag) {
	if hubName == "" {
		return
	}
	key := strings.ToLower(hubName)
	stateMu.Lock()
	state[key] = AdapterState{
		HubName:   hubName,
		ModuleKey: moduleKey,
		Rollout:   flag,
	}
	stateMu.Unlock()
}

// SnapshotAdapterStates 返回所有 Hub 的 rollout 状态，按名称排序。
func SnapshotAdapterStates() []AdapterState {
	stateMu.RLock()
	defer stateMu.RUnlock()

	if len(state) == 0 {
		return nil
	}

	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]AdapterState, 0, len(keys))
	for _, key := range keys {
		result = append(result, state[key])
	}
	return result
}
