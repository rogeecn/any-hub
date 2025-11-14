# Feature Specification: Modular Proxy & Cache Segmentation

**Feature Branch**: `004-modular-proxy-cache`  
**Created**: 2025-11-14  
**Status**: Draft  
**Input**: User description: "当前项目使用一个共用的 proxy、Cache 层处理代理逻辑， 这样导致在新增或变更接入时需要考虑已有类型的兼容，造成了后续可维护性变弱。把每种代理、缓存层使用类型进行分模块（目录）组织编写，抽象统一的interface用于 功能约束，这样虽然不同类型模块会有部分代码重复，但是可维护性会大大增强。"

> 宪法对齐（v1.0.0）：
> - 保持“轻量、匿名、CLI 多仓代理”定位：不得引入 Web UI、账号体系或与代理无关的范围。
> - 方案必须基于 Go 1.25+ 单二进制，依赖仅限 Fiber、Viper、Logrus/Lumberjack 及必要标准库。
> - 所有行为由单一 `config.toml` 控制；若需新配置项，需在规范中说明字段、默认值与迁移策略。
> - 设计需维护缓存优先 + 流式传输路径，并描述命中/回源/失败时的日志与观测需求。
> - 验收必须包含配置解析、缓存读写、Host Header 绑定等测试与中文注释交付约束。

## Clarifications

### Session 2025-11-14

- Q: Should each hub select proxy and cache modules separately or through a single combined module? → A: Single combined module per hub encapsulating proxy + cache behaviors.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Add A New Hub Type Without Regressions (Priority: P1)

As a platform maintainer, I can scaffold a dedicated proxy + cache module for a new hub type without touching existing hub implementations so I avoid regressions and lengthy reviews.

**Why this priority**: Unlocks safe onboarding of new ecosystems (npm, Docker, PyPI, etc.) which is the primary growth lever.

**Independent Test**: Provision a sample "testhub" type, wire it through config, and run integration tests showing legacy hubs still route correctly.

**Acceptance Scenarios**:

1. **Given** an empty module directory following the prescribed skeleton, **When** the maintainer registers the module via the unified interface, **Then** the hub becomes routable via config with no code changes in other hub modules.
2. **Given** existing hubs running in production, **When** the new hub type is added, **Then** regression tests confirm traffic for other hubs is unchanged and logs correctly identify hub-specific modules.

---

### User Story 2 - Tailor Cache Behavior Per Hub (Priority: P2)

As an SRE, I can choose a cache strategy module that matches a hub’s upstream semantics (e.g., npm tarballs vs. metadata) and tune TTL/validation knobs without rewriting shared logic.

**Why this priority**: Cache efficiency and disk safety differ by artifact type; misconfiguration previously caused incidents like "not a directory" errors.

**Independent Test**: Swap cache strategies for one hub in staging and verify cache hit/miss, revalidation, and eviction behavior follow the new module’s contract while others remain untouched.

**Acceptance Scenarios**:

1. **Given** a hub referencing cache strategy `npm-tarball`, **When** TTL overrides are defined in config, **Then** only that hub’s cache files adopt the overrides and telemetry reports the chosen strategy.
2. **Given** a hub using a streaming proxy that forbids disk writes, **When** the hub switches to a cache-enabled module, **Then** the interface enforces required callbacks (write, validate, purge) before deployment passes.

---

### User Story 3 - Operate Mixed Generations During Migration (Priority: P3)

As a release manager, I can keep legacy shared modules alive while migrating hubs incrementally, with clear observability that highlights which hubs still depend on the old stack.

**Why this priority**: Avoids risky flag days and allows gradual cutovers aligned with hub traffic peaks.

**Independent Test**: Run a deployment where half the hubs use the modular stack and half remain on the legacy stack, verifying routing table, logging, and alerts distinguish both paths.

**Acceptance Scenarios**:

1. **Given** hubs split between legacy and new modules, **When** traffic flows through both, **Then** logs, metrics, and config dumps tag each request path with its module name for debugging.
2. **Given** a hub scheduled for migration, **When** the rollout flag switches it to the modular implementation, **Then** rollback toggles exist to return to legacy routing within one command.

---

### Edge Cases

- What happens when config references a hub type whose proxy/cache module has not been registered? System must fail fast during config validation with actionable errors.
- How does the system handle partial migrations where legacy cache files conflict with new module layouts? Must auto-migrate or isolate on first access to prevent `ENOTDIR`.
- How is observability handled when a module panics or returns invalid data? The interface must standardize error propagation so circuit breakers/logging stay consistent.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide explicit proxy and cache interfaces describing the operations (request admission, upstream fetch, cache read/write/invalidation, observability hooks) that every hub-specific module must implement.
- **FR-002**: Restructure the codebase so each hub type registers a single module directory that owns both proxy and cache behaviors (optional internal subpackages allowed) while sharing only the common interfaces; no hub-specific logic may leak into the shared adapters.
- **FR-003**: Implement a registry or factory that maps the `config.toml` hub definition to the corresponding proxy/cache module and fails validation if no module is found.
- **FR-004**: Allow hub-level overrides for cache behaviors (TTL, validation strategy, disk layout) that modules can opt in to, with documented defaults and validation of allowed ranges.
- **FR-005**: Maintain backward compatibility by providing a legacy adapter that wraps the existing shared proxy/cache until all hubs migrate, including feature flags to switch per hub.
- **FR-006**: Ensure runtime telemetry (logs, metrics, tracing spans) include the module identifier so operators can attribute failures or latency to a specific hub module.
- **FR-007**: Deliver migration guidance and developer documentation outlining how to add a new module, required tests, and expected directory structure.
- **FR-008**: Update automated tests (unit + integration) so each module can be exercised independently and regression suites cover mixed legacy/new deployments.

### Key Entities *(include if feature involves data)*

- **Hub Module**: Represents a cohesive proxy+cache implementation for a specific ecosystem; attributes include supported protocols, cache strategy hooks, telemetry tags, and configuration constraints.
- **Module Registry**: Describes the mapping between hub names/types in config and their module implementations; stores module metadata (version, status, migration flag) for validation and observability.
- **Cache Strategy Profile**: Captures the policy knobs a module exposes (TTL, validation method, disk layout, eviction rules) and the allowed override values defined per hub.

### Assumptions

- Existing hubs (npm, Docker, PyPI) will be migrated sequentially; legacy adapters remain available until the last hub switches.
- Engineers adding a new hub type can modify configuration schemas and documentation but not core runtime dependencies.
- Telemetry stack (logs/metrics) already exists and only requires additional tags; no new observability backend is needed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new hub type can be added by touching only its module directory plus configuration (≤2 additional files) and passes the module’s test suite within one working day.
- **SC-002**: Regression test suites show zero failing cases for unchanged hubs after enabling the modular architecture (baseline established before rollout).
- **SC-003**: Configuration validation rejects 100% of hubs that reference unregistered modules, preventing runtime panics in staging or production.
- **SC-004**: Operational logs for proxy and cache events include the module identifier in 100% of entries, enabling SREs to scope incidents in under 5 minutes.
