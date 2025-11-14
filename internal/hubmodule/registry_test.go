package hubmodule

import "testing"

func replaceRegistry(t *testing.T) func() {
	t.Helper()
	prev := globalRegistry
	globalRegistry = newRegistry()
	return func() { globalRegistry = prev }
}

func TestRegisterResolveAndList(t *testing.T) {
	cleanup := replaceRegistry(t)
	defer cleanup()

	if err := Register(ModuleMetadata{Key: "beta", MigrationState: MigrationStateBeta}); err != nil {
		t.Fatalf("register beta failed: %v", err)
	}
	if err := Register(ModuleMetadata{Key: "gamma", MigrationState: MigrationStateGA}); err != nil {
		t.Fatalf("register gamma failed: %v", err)
	}

	if _, ok := Resolve("beta"); !ok {
		t.Fatalf("expected beta to resolve")
	}
	if _, ok := Resolve("BETA"); !ok {
		t.Fatalf("resolve should be case-insensitive")
	}

	list := List()
	if len(list) != 2 {
		t.Fatalf("list length mismatch: %d", len(list))
	}
	if list[0].Key != "beta" || list[1].Key != "gamma" {
		t.Fatalf("unexpected order: %+v", list)
	}
}

func TestRegisterDuplicateFails(t *testing.T) {
	cleanup := replaceRegistry(t)
	defer cleanup()

	if err := Register(ModuleMetadata{Key: "legacy"}); err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}
	if err := Register(ModuleMetadata{Key: "legacy"}); err == nil {
		t.Fatalf("duplicate registration should fail")
	}
}
