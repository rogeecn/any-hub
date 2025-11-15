# Tasks: Modular Proxy & Cache Segmentation

**Input**: Design documents from `/specs/004-modular-proxy-cache/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: 必须覆盖配置解析 (`internal/config`)、缓存读写 (`internal/cache` + 模块)、代理命中/回源 (`internal/proxy`)、Host Header 绑定与日志 (`internal/server`).

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Scaffold `internal/hubmodule/` package with `doc.go` + `README.md` describing module contracts
- [X] T002 [P] Add `modules-test` target to `Makefile` running `go test ./internal/hubmodule/...` for future CI hooks

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T003 Create shared module interfaces + registry in `internal/hubmodule/interfaces.go` and `internal/hubmodule/registry.go`
- [X] T004 Extend config schema with `[[Hub]].Module` defaults/validation plus sample configs in `internal/config/{types.go,validation.go,loader.go}` and `configs/*.toml`
- [X] T005 [P] Wire server bootstrap to resolve modules once and inject into proxy/cache layers (`internal/server/bootstrap.go`, `internal/proxy/handler.go`)

**Checkpoint**: Registry + config plumbing complete; user story work may begin.

---

## Phase 3: User Story 1 - Add A New Hub Type Without Regressions (Priority: P1) 🎯 MVP

**Goal**: Allow engineers to add a dedicated proxy+cache module without modifying existing hubs.
**Independent Test**: Register a `testhub` module, enable it via config, and run integration tests proving other hubs remain unaffected.

### Tests

- [X] T006 [P] [US1] Add registry unit tests covering register/resolve/list/dedup in `internal/hubmodule/registry_test.go`
- [X] T007 [P] [US1] Add integration test proving new module routing isolation in `tests/integration/module_routing_test.go`

### Implementation

- [X] T008 [US1] Implement `legacy` adapter module that wraps current shared proxy/cache in `internal/hubmodule/legacy/legacy_module.go`
- [X] T009 [US1] Refactor server/proxy wiring to resolve modules per hub (`internal/server/router.go`, `internal/proxy/forwarder.go`)
- [X] T010 [P] [US1] Create reusable module template with Chinese comments under `internal/hubmodule/template/module.go`
- [X] T011 [US1] Update quickstart + README to document module creation and config binding (`specs/004-modular-proxy-cache/quickstart.md`, `README.md`)

---

## Phase 4: User Story 2 - Tailor Cache Behavior Per Hub (Priority: P2)

**Goal**: Enable per-hub cache strategies/TTL overrides while keeping modules isolated.
**Independent Test**: Swap a hub to a cache strategy module, adjust TTL overrides, and confirm telemetry/logs reflect the new policy without affecting other hubs.

### Tests

- [X] T012 [P] [US2] Add cache strategy override integration test validating TTL + revalidation paths in `tests/integration/cache_strategy_override_test.go`
- [X] T013 [P] [US2] Add module-level cache strategy unit tests in `internal/hubmodule/npm/module_test.go`

### Implementation

- [X] T014 [US2] Implement `CacheStrategyProfile` helpers and injection plumbing (`internal/hubmodule/strategy.go`, `internal/cache/writer.go`)
- [X] T015 [US2] Bind hub-level overrides to strategy metadata via config/runtime structures (`internal/config/types.go`, `internal/config/runtime.go`)
- [X] T016 [US2] Update existing modules (npm/docker/pypi) to declare strategies + honor overrides (`internal/hubmodule/{npm,docker,pypi}/module.go`)

---

## Phase 5: User Story 3 - Operate Mixed Generations During Migration (Priority: P3)

**Goal**: Support dual-path deployments with diagnostics/logging to track legacy vs. modular hubs.
**Independent Test**: Run mixed legacy/modular hubs, flip rollout flags, and confirm logs + diagnostics show module ownership and allow rollback.

### Tests

- [X] T017 [P] [US3] Add dual-mode integration test covering rollout toggle + rollback in `tests/integration/legacy_adapter_toggle_test.go`
- [X] T018 [P] [US3] Add diagnostics endpoint contract test for `/−/modules` in `tests/integration/module_diagnostics_test.go`

### Implementation

- [X] T019 [US3] Implement `LegacyAdapterState` tracker + rollout flag parsing (`internal/hubmodule/legacy/state.go`, `internal/config/runtime_flags.go`)
- [X] T020 [US3] Implement Fiber handler + routing for `/−/modules` diagnostics (`internal/server/routes/modules.go`, `internal/server/router.go`)
- [X] T021 [US3] Add structured log fields (`module_key`, `rollout_flag`) across logging middleware (`internal/server/middleware/logging.go`, `internal/proxy/logging.go`)
- [X] T022 [US3] Document operational playbook for phased migration (`docs/operations/migration.md`)

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T023 [P] Add Chinese comments + GoDoc for new interfaces/modules (`internal/hubmodule/**/*.go`)
- [X] T024 Validate quickstart by running module creation flow end-to-end and capture sample logs (`specs/004-modular-proxy-cache/quickstart.md`, `logs/`)

---

## Dependencies & Execution Order

1. **Phase 1 → Phase 2**: Setup must finish before registry/config work begins.
2. **Phase 2 → User Stories**: Module registry + config binding are prerequisites for all stories.
3. **User Stories Priority**: US1 (P1) delivers MVP and unblocks US2/US3; US2 & US3 can run in parallel after US1 if separate modules/files.
4. **Tests before Code**: For each story, write failing tests (T006/T007, T012/T013, T017/T018) before implementation tasks in that story.
5. **Polish**: Execute after all targeted user stories complete.

## Parallel Execution Examples

- **Setup**: T001 (docs) and T002 (Makefile) can run concurrently.
- **US1**: T006 registry tests and T007 routing tests can run in parallel while separate engineers tackle T008/T010.
- **US2**: T012 integration test and T013 unit test proceed concurrently; T014/T015 can run in parallel once T012/T013 drafted.
- **US3**: T017 rollout test and T018 diagnostics test work independently before T019–T021 wiring.

## Implementation Strategy

1. Deliver MVP by completing Phases 1–3 (US1) and verifying new module onboarding works end-to-end.
2. Iterate with US2 for cache flexibility, ensuring overrides are testable independently.
3. Layer US3 for migration observability and rollback safety.
4. Finish with Polish tasks to document and validate the workflow.
