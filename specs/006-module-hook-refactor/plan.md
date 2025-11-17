# Implementation Plan: Module Hook Refactor

**Branch**: `006-module-hook-refactor` | **Date**: 2025-11-17 | **Spec**: specs/006-module-hook-refactor/spec.md  
**Input**: Feature specification from `/specs/006-module-hook-refactor/spec.md`

**Note**: This file captures planning up to Phase 2 (tasks generated separately).

## Summary

目标：让每个 hubmodule 完整自管缓存/代理逻辑（路径重写、缓存策略、上游解析、响应重写等），proxy handler 仅负责调度、缓存读写及统一日志/错误包装。技术路线：\n1. 定义 Hook/Handler 契约与注册机制。\n2. 将 Docker/NPM/PyPI/Composer/Go 的特化逻辑迁移到各自 Hook。\n3. legacy handler 仅做兜底；proxy handler 移除所有 `hub_type` 分支并加强错误观测。

## Technical Context

**Language/Version**: Go 1.25+ (静态链接，单二进制交付)  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`  
**Storage**: 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`（由模块 Hook 定义布局）  
**Testing**: `go test ./...`，使用 `httptest`、临时目录与模块 Hook 示例验证缓存/代理路径  
**Target Platform**: Linux/Unix CLI 进程，由 systemd/supervisor 管理，匿名下游客户端  
**Project Type**: 单 Go 项目（`cmd/` 入口 + `internal/*` 包）  
**Performance Goals**: 缓存命中直接返回；回源路径需流式转发，单请求常驻内存 <256MB；命中/回源日志可追踪  
**Constraints**: 禁止 Web UI 或账号体系；所有行为受单一 TOML 控制；每个 Hub 需独立 Domain 绑定；仅匿名访问  
**Scale/Scope**: 支撑 Docker/NPM/Go/PyPI/Composer，多仓 Hook 自治，面向弱网及离线缓存复用场景

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Feature 仍然是“轻量多仓 CLI 代理”，未引入 Web UI、账号体系或与代理无关的能力。
- 仅使用 Go + 宪法指定依赖；任何新第三方库都已在本计划中说明理由与审核结论。
- 行为完全由 `config.toml` 控制，新增配置项已规划默认值、校验与迁移策略。
- 方案维持缓存优先 + 流式回源路径，并给出命中/回源/失败的日志与观测手段。
- 计划内列出了配置解析、缓存读写、Host Header 路由等强制测试与中文注释交付范围。

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
