package hubmodule

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

const defaultModuleKey = "legacy"

var globalRegistry = newRegistry()

type registry struct {
	mu      sync.RWMutex
	modules map[string]ModuleMetadata
}

func newRegistry() *registry {
	return &registry{modules: make(map[string]ModuleMetadata)}
}

// Register 将模块元数据加入全局注册表，重复键会返回错误。
func Register(meta ModuleMetadata) error {
	return globalRegistry.register(meta)
}

// MustRegister 在注册失败时 panic，适合模块 init() 中调用。
func MustRegister(meta ModuleMetadata) {
	if err := Register(meta); err != nil {
		panic(err)
	}
}

// Resolve 返回指定键的模块元数据。
func Resolve(key string) (ModuleMetadata, bool) {
	return globalRegistry.resolve(key)
}

// List 返回按键排序的模块元数据列表。
func List() []ModuleMetadata {
	return globalRegistry.list()
}

// Keys 返回所有已注册模块的键值，供调试或诊断使用。
func Keys() []string {
	items := List()
	result := make([]string, len(items))
	for i, meta := range items {
		result[i] = meta.Key
	}
	return result
}

func (r *registry) normalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func (r *registry) register(meta ModuleMetadata) error {
	if meta.Key == "" {
		return fmt.Errorf("module key is required")
	}
	key := r.normalizeKey(meta.Key)
	if key == "" {
		return fmt.Errorf("module key is required")
	}
	meta.Key = key

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[key]; exists {
		return fmt.Errorf("module %s already registered", key)
	}
	r.modules[key] = meta
	return nil
}

func (r *registry) mustRegister(meta ModuleMetadata) {
	if err := r.register(meta); err != nil {
		panic(err)
	}
}

func (r *registry) resolve(key string) (ModuleMetadata, bool) {
	if key == "" {
		return ModuleMetadata{}, false
	}
	normalized := r.normalizeKey(key)

	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, ok := r.modules[normalized]
	return meta, ok
}

func (r *registry) list() []ModuleMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.modules) == 0 {
		return nil
	}

	keys := make([]string, 0, len(r.modules))
	for key := range r.modules {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]ModuleMetadata, 0, len(keys))
	for _, key := range keys {
		result = append(result, r.modules[key])
	}
	return result
}
