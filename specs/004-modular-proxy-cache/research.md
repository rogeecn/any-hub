# Research Log: Modular Proxy & Cache Segmentation

## Decision 1: Module Registry Location
- **Decision**: Introduce `internal/hubmodule/` as the root for module implementations plus a `registry.go` that exposes `Register(name ModuleFactory)` and `Resolve(hubType string)` helpers.
- **Rationale**: Keeps new hub-specific code outside `internal/proxy`/`internal/cache` core while still within internal tree; mirrors existing package layout expectations and eases discovery.
- **Alternatives considered**:
  - Embed modules under `internal/proxy/<hub>`: rejected because cache + proxy concerns would blend with shared proxy infra, blurring ownership lines.
  - Place modules under `pkg/`: rejected since repo avoids exported libraries and wants all runtime code under `internal`.

## Decision 2: Config Binding Field
- **Decision**: Add optional `Module` string field to each `[[Hub]]` block in `config.toml`, defaulting to `"legacy"` to preserve current behavior. Validation ensures the value matches a registered module key.
- **Rationale**: Minimal change to config schema, symmetric across hubs, and allows gradual opt-in by flipping a single field.
- **Alternatives considered**:
  - Auto-detect module from `hub.Name`: rejected because naming conventions differ across users and would impede third-party forks.
  - Separate `ProxyModule`/`CacheModule` fields: rejected per clarification outcome that modules encapsulate both behaviors.

## Decision 3: Fiber/Viper/Logrus Best Practices for Modular Architecture
- **Decision**: Continue to initialize Fiber/Viper/Logrus exactly once at process start; modules receive interfaces (logger, config handles) instead of initializing their own instances.
- **Rationale**: Prevents duplicate global state and adheres to constitution (single binary, centralized config/logging).
- **Alternatives considered**: Allow modules to spin up custom Fiber groups or loggers—rejected because it complicates shutdown hooks and breaks structured logging consistency.

## Decision 4: Storage Layout Compatibility
- **Decision**: Keep current `StoragePath/<Hub>/<path>.body` layout; modules may add subdirectories below `<path>` only when necessary but must expose migration hooks via registry metadata.
- **Rationale**: Recent cache fix established `.body` suffix to avoid file/dir conflicts; modules should reuse it to maintain operational tooling compatibility.
- **Alternatives considered**: Give each module a distinct root folder—rejected because it would fragment cleanup tooling and require per-module disk quotas.

## Decision 5: Testing Strategy
- **Decision**: For each module, enforce a shared test harness that spins a fake upstream using `httptest.Server`, writes to `t.TempDir()` storage, and asserts registry wiring end-to-end via integration tests.
- **Rationale**: Aligns with Technical Context testing guidance while avoiding bespoke harnesses per hub type.
- **Alternatives considered**: Rely solely on unit tests per module—rejected since regressions often arise from wiring/registry mistakes.
