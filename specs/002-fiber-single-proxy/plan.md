# Implementation Plan: HTTP 服务与单仓代理

**Branch**: `002-fiber-single-proxy` | **Date**: 2025-11-13 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-fiber-single-proxy/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Deliver a Phase 1-ready proxy: Fiber HTTP server routing by Host→Hub, disk cache with TTL + conditional revalidation, and runnable Docker/NPM samples backed by integration tests. Core work spans server/router scaffolding, cache/proxy pipeline, observability, and documentation to prove the feature end to end.

## Technical Context

**Language/Version**: Go 1.25+ (静态链接，单二进制交付)  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`  
**Storage**: 本地文件系统缓存目录 `StoragePath/<Hub>/<path>` + `.meta` 元数据  
**Testing**: `go test ./...`，配合 `httptest`、fake upstream server、临时目录验证路由/缓存/集成路径  
**Target Platform**: Linux/Unix CLI 进程，由 systemd/supervisor 管理，匿名下游客户端  
**Project Type**: 单 Go 项目（`cmd/` 入口 + `internal/*` 包）  
**Performance Goals**: 缓存命中路径需低延迟（相对首个请求减少 ≥70%），回源路径流式传输并限制常驻内存 <256MB，日志可追踪命中/回源/错误  
**Constraints**: 禁止 Web/UI、账号体系、额外配置源；必须通过 `config.toml` 驱动 Host、端口、缓存策略；请求匿名；不可新增未审核依赖  
**Scale/Scope**: Phase 1 仅实现单 Hub 路由 + Docker/NPM 示例；后续多 Hub、多协议将基于此扩展

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- 方案遵守“轻量 CLI 代理”定位，不引入 UI/账号/外部控制面。
- 继续使用 Go + Fiber/Viper/Logrus/Lumberjack + 标准库，Helm/DB/Web 依赖均不在 scope。
- 仅依赖 `config.toml` 说明 Host/端口/缓存字段；新增字段需写入配置 schema 与文档。
- 缓存策略保持“命中即返回、未命中流式回源”，并要求结构化日志记录 `cache_hit`、上游状态等字段。
- 计划包含配置解析、Host Header 路由、缓存逻辑、示例及测试，满足宪法强制测试/文档 Gate。

**Post-design re-check (Phase 1)**: 设计阶段将审查 server/cache/示例文档，确认未引入新依赖且 quickstart/contract 体现在 spec/plan/task 中；若出现额外库或多配置源，需重新申请豁免。

## Project Structure

### Documentation (this feature)

```text
specs/002-fiber-single-proxy/
├── plan.md              # 当前文件
├── research.md          # Phase 0 研究决策
├── data-model.md        # 实体/关系/约束
├── quickstart.md        # 演练 Docker/NPM 示例步骤
├── contracts/           # CLI/Fiber/缓存交付合同（md 或 OpenAPI）
└── tasks.md             # Phase 2 工作拆解（/speckit.tasks 输出）
```

### Source Code (repository root)
```text
cmd/any-hub/             # CLI 入口，启动 server/cache
internal/server/         # Fiber app、Host Registry、路由/中间件
internal/cache/          # 磁盘缓存/元数据/TTL
internal/proxy/          # 回源/条件请求/流式复制
internal/logging/        # 结构化日志字段
configs/                 # 示例 config (docker|npm)
tests/integration/       # upstream stub + e2e 测试
```

**Structure Decision**: 仍按单仓库单二进制；新增 `internal/server`, `internal/cache`, `tests/integration` 目录必须保持与 `internal/proxy` 清晰边界，所有引用通过包级接口注入，避免循环依赖。

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _None_ | — | — |
