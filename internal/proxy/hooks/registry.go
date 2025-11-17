package hooks

import (
	"errors"
	"strings"
	"sync"
)

var registry sync.Map

// ErrDuplicateHook indicates a module key already has hooks registered.
var ErrDuplicateHook = errors.New("hook already registered")

// Register stores hooks for the given module key.
func Register(moduleKey string, hooks Hooks) error {
	key := normalizeKey(moduleKey)
	if key == "" {
		return errors.New("module key required")
	}
	if _, loaded := registry.LoadOrStore(key, hooks); loaded {
		return ErrDuplicateHook
	}
	return nil
}

// MustRegister panics on registration failure.
func MustRegister(moduleKey string, hooks Hooks) {
	if err := Register(moduleKey, hooks); err != nil {
		panic(err)
	}
}

// Fetch retrieves hooks associated with a module key.
func Fetch(moduleKey string) (Hooks, bool) {
	key := normalizeKey(moduleKey)
	if key == "" {
		return Hooks{}, false
	}
	if value, ok := registry.Load(key); ok {
		if hooks, ok := value.(Hooks); ok {
			return hooks, true
		}
	}
	return Hooks{}, false
}

// Status returns hook registration status for a module key.
func Status(moduleKey string) string {
	if _, ok := Fetch(moduleKey); ok {
		return "registered"
	}
	return "missing"
}

// Snapshot returns status for a list of module keys.
func Snapshot(keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if normalized := normalizeKey(key); normalized != "" {
			out[normalized] = Status(normalized)
		}
	}
	return out
}

func normalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
