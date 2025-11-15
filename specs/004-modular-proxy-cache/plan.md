# Implementation Plan: Modular Proxy & Cache Segmentation

**Branch**: `004-modular-proxy-cache` | **Date**: 2025-11-14 | **Spec**: /home/rogee/Projects/any-hub/specs/004-modular-proxy-cache/spec.md
**Input**: Feature specification from `/specs/004-modular-proxy-cache/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Modularize the proxy and cache layers so every hub type (npm, Docker, PyPI, future ecosystems) implements a self-contained module that conforms to shared interfaces, is registered via config, and exposes hub-specific cache strategies while preserving legacy behavior during phased migration. The work introduces a module registry/factory, per-hub configuration for selecting modules, migration tooling, and observability tags so operators can attribute incidents to specific modules.

## Technical Context

**Language/Version**: Go 1.25+ (静态链接，单二进制交付)  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`  
**Storage**: 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`，直接复用请求路径完成磁盘定位  
**Testing**: `go test ./...`，使用 `httptest`、临时目录和自建上游伪服务验证配置/缓存/代理路径  
**Target Platform**: Linux/Unix CLI 进程，由 systemd/supervisor 管理，匿名下游客户端  
**Project Type**: 单 Go 项目（`cmd/` 入口 + `internal/*` 包）  
**Performance Goals**: 缓存命中直接返回；回源路径需流式转发，单请求常驻内存 <256MB；命中/回源日志可追踪  
**Constraints**: 禁止 Web UI 或账号体系；所有行为受单一 TOML 配置控制；每个 Hub 需独立 Domain/Port 绑定；仅匿名访问  
**Scale/Scope**: 支撑 Docker/NPM/Go/PyPI 等多仓代理，面向弱网及离线缓存复用场景  
**Module Registry Location**: `internal/hubmodule/registry.go` 暴露注册/解析 API，模块子目录位于 `internal/hubmodule/<name>/`  
**Config Binding for Modules**: `[[Hub]].Module` 字段控制模块名，默认 `legacy`，配置加载阶段校验必须命中已注册模块

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Feature 仍然是“轻量多仓 CLI 代理”，未引入 Web UI、账号体系或与代理无关的能力。
- 仅使用 Go + 宪法指定依赖；任何新第三方库都已在本计划中说明理由与审核结论。
- 行为完全由 `config.toml` 控制，新增 `[[Hub]].Module` 配置项已规划默认值、校验与迁移策略。
- 方案维持缓存优先 + 流式回源路径，并给出命中/回源/失败的日志与观测手段。
- 计划内列出了配置解析、缓存读写、Host Header 路由等强制测试与中文注释交付范围。

**Gate Status**: ✅ All pre-research gates satisfied; no violations logged in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
```text
cmd/any-hub/main.go        # CLI 入口、参数解析
internal/config/           # TOML 加载、默认值、校验
internal/server/           # Fiber 服务、路由、中间件
internal/cache/            # 磁盘/内存缓存与 .meta 管理
internal/proxy/            # 上游访问、缓存策略、流式复制
configs/                   # 示例 config.toml（如需）
tests/                     # `go test` 下的单元/集成测试，用临时目录
```

**Structure Decision**: 采用单 Go 项目结构，特性代码应放入上述现有目录；如需新增包或目录，必须解释其与 `internal/*` 的关系并给出后续维护策略。

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |

## Phase 0 – Research

### Unknowns & Tasks
- **Module registry location** → researched Go package placement that keeps modules isolated yet internal.
- **Config binding for modules** → determined safest schema extension and defaults.
- **Dependency best practices** → confirmed singletons for Fiber/Viper/Logrus and storage layout compatibility.
- **Testing harness expectations** → documented shared approach for new modules.

### Output Artifact
- `/home/rogee/Projects/any-hub/specs/004-modular-proxy-cache/research.md` summarizes each decision with rationale and alternatives.

### Impact on Plan
- Technical Context now references concrete package paths and configuration fields.
- Implementation will add `internal/hubmodule/` with registry helpers plus validation wiring in `internal/config`.

## Phase 1 – Design & Contracts

### Data Model
- `/home/rogee/Projects/any-hub/specs/004-modular-proxy-cache/data-model.md` defines HubConfigEntry, ModuleMetadata, ModuleRegistry, CacheStrategyProfile, and LegacyAdapterState including validation and state transitions.

### API Contracts
- `/home/rogee/Projects/any-hub/specs/004-modular-proxy-cache/contracts/module-registry.openapi.yaml` introduces a diagnostics API (`GET /-/modules`, `GET /-/modules/{key}`) for observability around module registrations and hub bindings.

### Quickstart Guidance
- `/home/rogee/Projects/any-hub/specs/004-modular-proxy-cache/quickstart.md` walks engineers through adding a module, wiring config, running tests, and verifying logs/storage.

### Agent Context Update
- `.specify/scripts/bash/update-agent-context.sh codex` executed to sync AGENTS.md with Go/Fiber/Viper/logging/storage context relevant to this feature.

### Post-Design Constitution Check
- New diagnostics endpoint remains internal and optional; no UI/login introduced. ✅ Principle I
- Code still single Go binary with existing dependency set. ✅ Principle II
- `Module` field documented with defaults, validation, and migration path; no extra config sources. ✅ Principle III
- Cache strategy enforces“原始路径 == 磁盘路径”的布局与流式回源，相关观测需求写入 contracts。✅ Principle IV
- Logs/quickstart/test guidance ensure observability and Chinese documentation continue. ✅ Principle V

## Phase 2 – Implementation Outlook (pre-tasks)

1. **Module Registry & Interfaces**: Create `internal/hubmodule` package, define shared interfaces, implement registry with tests, and expose diagnostics data source reused by HTTP endpoints.
2. **Config Loader & Validation**: Extend `internal/config/types.go` and `validation.go` to include `Module` with default `legacy`, plus wiring to registry resolution during startup.
3. **Legacy Adapter & Migration Switches**: Provide adapter module that wraps current shared proxy/cache, plus feature flags or config toggles to control rollout states per hub.
4. **Module Implementations**: Carve existing npm/docker/pypi logic into dedicated modules within `internal/hubmodule/`, ensuring cache writer复用原始请求路径与必要的 telemetry 标签。
5. **Observability/Diagnostics**: Implement `/−/modules` endpoint (Fiber route) and log tags showing `module_key` on cache/proxy events.
6. **Testing**: Add shared test harness for modules, update integration tests to cover mixed legacy + modular hubs, and document commands in README/quickstart.
