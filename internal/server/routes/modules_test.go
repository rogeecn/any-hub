package routes

import (
	"testing"
	"time"

	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func TestEncodeModulesAddsHookStatus(t *testing.T) {
	modules := []hubmodule.ModuleMetadata{
		{
			Key: "b",
			CacheStrategy: hubmodule.CacheStrategyProfile{
				TTLHint:        time.Hour,
				ValidationMode: hubmodule.ValidationModeNever,
				DiskLayout:     "flat",
			},
		},
		{
			Key: "a",
			CacheStrategy: hubmodule.CacheStrategyProfile{
				TTLHint:        time.Minute,
				ValidationMode: hubmodule.ValidationModeNever,
				DiskLayout:     "flat",
			},
		},
	}
	status := map[string]string{"a": "registered"}

	encoded := encodeModules(modules, status)
	if len(encoded) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(encoded))
	}
	if encoded[0].Key != "a" {
		t.Fatalf("expected sorted module key a first, got %s", encoded[0].Key)
	}
	if encoded[0].HookStatus != "registered" {
		t.Fatalf("expected hook status registered for a, got %s", encoded[0].HookStatus)
	}
	if encoded[1].Key != "b" {
		t.Fatalf("expected second module key b, got %s", encoded[1].Key)
	}
	if encoded[1].HookStatus != "" {
		t.Fatalf("expected empty hook status for b, got %s", encoded[1].HookStatus)
	}
}

func TestEncodeModuleAddsStatusForDetail(t *testing.T) {
	key := "module-routes-test"
	_ = hooks.Register(key, hooks.Hooks{})

	meta := hubmodule.ModuleMetadata{
		Key: key,
		CacheStrategy: hubmodule.CacheStrategyProfile{
			TTLHint:        time.Minute,
			ValidationMode: hubmodule.ValidationModeNever,
			DiskLayout:     "flat",
		},
	}
	payload := encodeModule(meta)
	payload.HookStatus = hooks.Status(key)
	if payload.HookStatus != "registered" {
		t.Fatalf("expected hook status registered, got %s", payload.HookStatus)
	}
}
