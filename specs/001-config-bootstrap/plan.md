# Implementation Plan: 配置与骨架

**Branch**: `001-config-bootstrap` | **Date**: 2025-11-13 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-config-bootstrap/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Phase 0 delivers a reliable bootstrap for any-hub: a validated `internal/config` loader with defaults/tests, a CLI entry (`cmd/any-hub`) that accepts `--config`, `--check-config`, `--version`, and a Logrus + Lumberjack logging stack that honors config-provided level/output. The approach is to codify the configuration schema (global + `[[Hub]]`), implement strict validation with helpful errors, ensure CLI paths separate validation vs. runtime, and guarantee consistent structured logging for both success and failure flows.

## Technical Context

**Language/Version**: Go 1.25+ (静态链接，单二进制交付)  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`  
**Storage**: 本地文件系统缓存目录 `StoragePath/<Hub>/<path>` + `.meta` 元数据  
**Testing**: `go test ./...`，使用 `httptest`、临时目录和自建上游伪服务验证配置/缓存/代理路径  
**Target Platform**: Linux/Unix CLI 进程，由 systemd/supervisor 管理，匿名下游客户端  
**Project Type**: 单 Go 项目（`cmd/` 入口 + `internal/*` 包）  
**Performance Goals**: 缓存命中直接返回；回源路径需流式转发，单请求常驻内存 <256MB；命中/回源日志可追踪  
**Constraints**: 禁止 Web UI 或账号体系；所有行为受单一 TOML 配置控制；每个 Hub 需独立 Domain/Port 绑定；仅匿名访问  
**Scale/Scope**: 支撑 Docker/NPM/Go/PyPI 等多仓代理，面向弱网及离线缓存复用场景

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Feature 聚焦“轻量、匿名、CLI 多仓代理”，未引入 Web UI、账号体系或与代理无关的能力。
- 仅使用 Go + 宪法指定依赖（Fiber/Viper/Logrus/Lumberjack + 标准库）；无新增第三方库需求。
- 行为完全由 `config.toml` 控制，计划中定义所有默认值、优先级（标志 > 环境变量 > 默认）、以及迁移策略。
- CLI 与配置实现维持缓存优先 + 流式回源的大前提，并输出命中/回源/失败日志字段，满足可观测原则。
- 计划明确覆盖配置解析、必填字段校验、Host Header 约束、中文注释与 `go test` 需求，满足测试 Gate。

**Post-design re-check (Phase 1)**: 设计阶段确认各文档与合同未偏离宪法约束，仍无新增依赖、UI 或多配置源，GATE 维持通过。

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
| _None_ | — | — |
