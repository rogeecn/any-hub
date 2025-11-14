# Implementation Plan: Hub 配置凭证字段

**Branch**: `003-hub-auth-fields` | **Date**: 2025-11-14 | **Spec**: [/home/rogee/Projects/any-hub/specs/003-hub-auth-fields/spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-hub-auth-fields/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

交付单端口 Host 路由代理的配置增强：
1. 将监听端口提升到全局 `ListenPort` 字段，所有 Hub 共享同一 Fiber 端口并严格依赖 Host 头；旧的 `[[Hub]].Port` 需在配置校验阶段报错并提供迁移指引。
2. 为 Hub 添加可选 `Username`/`Password` 字段，由 CLI 在回源时自动注入 Authorization header，同时在日志中只暴露掩码。
3. 为 Hub 添加必填 `Type` 枚举（当前支持 docker/npm/go），驱动日志字段与未来协议特定策略，并保留扩展钩子。
4. 更新示例配置、quickstart、文档与测试，确保凭证/类型/单端口行为可独立验证。

## Technical Context

**Language/Version**: Go 1.25+（静态链接单二进制）  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置加载/校验）、Logrus + Lumberjack（结构化日志 & 滚动）、标准库 `net/http`/`io`（代理回源）  
**Storage**: 本地 `StoragePath/<Hub>/<path>` + `.meta` 元数据缓存布局  
**Testing**: `GOCACHE=/tmp/go-build go test ./...`，基于 `httptest` + upstream stub + 临时目录验证配置、代理、缓存、Host 路由  
**Target Platform**: Linux/Unix CLI 进程，由 systemd/supervisor 管理，匿名下游客户端  
**Project Type**: 单 Go 模块（`cmd/any-hub` 入口 + `internal/config|server|cache|proxy`）  
**Performance Goals**: 回源路径仍需流式复制并保持单请求 <256MB；凭证注入不可降低缓存命中率；单端口模式吞吐与现状持平  
**Constraints**: 禁止新增依赖；所有行为受单一 `config.toml` 控制；凭证不得写入日志；需提供向后兼容的迁移提示  
**Scale/Scope**: 当前支持 docker/npm/go 三类仓库，未来可扩展 apt/yum/composer/go proxy 等；单端口 Host 路由需支撑数十 Hub 并保持透明体验

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- 方案继续服务“轻量 CLI 多仓代理”使命，未引入 UI/账号体系或附加系统。
- 仍使用 Go + Fiber/Viper/Logrus/Lumberjack/标准库，无新增依赖。
- 所有新增能力均由 `config.toml` 驱动，且设计了默认值/校验/迁移提示。
- 缓存优先 + 流式回源路径保持不变；凭证注入仅影响上游请求头。
- 计划列出了配置解析、代理凭证、Host 路由等必备测试，并要求中文注释与 quickstart 更新。

## Project Structure

### Documentation (this feature)

```text
specs/003-hub-auth-fields/
├── plan.md              # 当前文件
├── research.md          # Phase 0 输出
├── data-model.md        # Phase 1 输出
├── quickstart.md        # Phase 1 输出
├── contracts/           # Phase 1 输出
└── tasks.md             # Phase 2 输出（由 /speckit.tasks 生成）
```

### Source Code (repository root)
```text
cmd/any-hub/main.go        # CLI 入口、单端口监听
internal/config/           # TOML schema + 校验 + 默认值
internal/server/           # Host Registry、Fiber 启动
internal/cache/            # 缓存方案（需保持兼容）
internal/proxy/            # 凭证注入、类型策略、流式代理
configs/                   # 示例 config（docker/npm/go）
tests/                     # 单元/集成测试 (httptest、stubs)
```

**Structure Decision**: 延续既有模块划分；单端口逻辑集中在 `internal/server`，凭证/类型侧重 `internal/config` + `internal/proxy`，必要时可新增 `internal/server/types` 等子包但需保持依赖有向无环。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _None_ | — | — |
