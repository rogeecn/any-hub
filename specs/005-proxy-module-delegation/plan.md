# Implementation Plan: Proxy Module Delegation

**Branch**: `005-proxy-module-delegation` | **Date**: 2025-11-17 | **Spec**: specs/005-proxy-module-delegation/spec.md  
**Input**: Feature specification from `/specs/005-proxy-module-delegation/spec.md`

**Note**: This file captures planning up to Phase 2 (tasks generated separately).

## Summary

目标：让通用 proxy 只做路由分发，所有仓类型的缓存/回源/重写/错误处理下沉到各自 hubmodule handler；新增仓类型仅需新增模块与 handler，不再修改通用 proxy 分支，同时保持现有仓（docker/npm/pypi/composer/go）功能与日志不回归。技术路线：调整模块注册/调度接口，模块自管理缓存策略与路径重写，通用层只封装调度、统一日志与错误包装。

## Goals & Constraints Recap

- 通用层职责最小化：仅路由映射与 handler 调度，无类型分支或缓存策略逻辑。
- 模块自洽：每个模块自带元数据、缓存策略、路径重写与 handler；新增模块不改通用代码。
- 兼容优先：现有仓库的缓存/日志/响应行为保持等价；日志字段保持 hub/domain/module_key/cache_hit/upstream_status/request_id。
- 控制面统一：所有行为仍由 `config.toml` 控制，缺失或重复模块注册需启动前失败。

## Technical Context

**Language/Version**: Go 1.25+ (静态链接，单二进制交付)  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`  
**Storage**: 本地文件系统缓存目录 `StoragePath/<Hub>/<path>`（按模块定义的布局）  
**Testing**: `go test ./...`，使用 `httptest`、临时目录和上游伪服务验证配置/缓存/代理路径  
**Target Platform**: Linux/Unix CLI 进程，由 systemd/supervisor 管理，匿名下游客户端  
**Project Type**: 单 Go 项目（`cmd/` 入口 + `internal/*` 包）  
**Performance Goals**: 缓存命中直接返回；回源路径需流式转发，单请求常驻内存 <256MB；命中/回源日志可追踪  
**Constraints**: 禁止 Web UI 或账号体系；所有行为受单一 TOML 配置控制；每个 Hub 需独立 Domain 绑定；仅匿名访问  
**Scale/Scope**: 支撑 Docker/NPM/Go/PyPI/Composer 多仓代理，面向弱网及离线缓存复用场景

## Current Dispatch & Branching (现状与移除点)

- router 层 (`internal/server/router.go`) 通过 `ensureRouterHubType` 针对 hub_type 做白名单；需移除类型判断，改为 module_key→handler 是否注册的检查。
- proxy 层 (`internal/proxy/handler.go`) 通过 `ensureProxyHubType` 再次按 hub_type 分支；迁移后应删除，转由模块 handler 自己决定支持策略。
- forwarder (`internal/proxy/forwarder.go`) 维护 module_handler map，但默认 handler 仍被类型检查阻断；目标是保留 map/lookup，去除类型特例。
- 移除点：去掉 hub_type 判定与每类型的特化分支，将错误处理集中在“handler 缺失/异常”路径；模块内保留各自策略。

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
| *(none)* | - | - |
