package hooks

import (
	"sync"
	"testing"
)

func TestRegisterAndFetch(t *testing.T) {
	registry = sync.Map{}
	h := Hooks{ContentType: func(*RequestContext, string) string { return "ok" }}
	if err := Register("test", h); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if _, ok := Fetch("test"); !ok {
		t.Fatalf("expected fetch ok")
	}
	if Status("test") != "registered" {
		t.Fatalf("expected registered status")
	}
	if Status("missing") != "missing" {
		t.Fatalf("expected missing status")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	registry = sync.Map{}
	if err := Register("dup", Hooks{}); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := Register("dup", Hooks{}); err != ErrDuplicateHook {
		t.Fatalf("expected ErrDuplicateHook, got %v", err)
	}
}

func TestSnapshot(t *testing.T) {
	registry = sync.Map{}
	_ = Register("a", Hooks{})
	snap := Snapshot([]string{"a", "b"})
	if snap["a"] != "registered" {
		t.Fatalf("expected a registered, got %s", snap["a"])
	}
	if snap["b"] != "missing" {
		t.Fatalf("expected b missing, got %s", snap["b"])
	}
}
