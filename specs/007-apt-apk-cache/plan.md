# Implementation Plan: APT/APK 包缓存模块

**Branch**: `007-apt-apk-cache` | **Date**: 2025-11-17 | **Spec**: `/home/rogee/Projects/any-hub/specs/007-apt-apk-cache/spec.md`
**Input**: Feature specification from `/specs/007-apt-apk-cache/spec.md`

## Summary

为 any-hub 增加 APT（Debian/Ubuntu）与 Alpine APK 的缓存代理模块：索引（Release/InRelease/Packages*/APKINDEX）必须再验证，包体（pool/*.deb、packages/*）视作不可变直接缓存，支持 Acquire-By-Hash/签名透传，不影响现有各模块。

## Technical Context

**Language/Version**: Go 1.25+ (静态链接，单二进制)  
**Primary Dependencies**: Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack、标准库 `net/http`/`io`  
**Storage**: 本地 `StoragePath/<Hub>/<path>` + `.meta`；索引需带 Last-Modified/ETag 元信息；包体按原路径落盘  
**Protocols**: APT (`/dists/<suite>/<component>/binary-<arch>/Packages*`, `Release`, `InRelease`, `pool/*`), Acquire-By-Hash；Alpine (`APKINDEX.tar.gz`, `packages/<arch>/…`)  
**Caching Rules**: 索引 RequireRevalidate=true；包体 AllowStore/AllowCache=true、RequireRevalidate=false；签名/校验文件原样透传  
**Testing**: `go test ./...`，构造 httptest 上游返回 Release/Packages/APKINDEX 与包体，验证首次回源+再验证+命中；使用临时目录校验缓存落盘  
**Target Platform**: Linux/Unix CLI，systemd/supervisor 托管，匿名客户端  
**Constraints**: 单一 config.toml 控制；禁止新增第三方依赖；保持现有模块行为不变  
**Risk/Unknowns**: Acquire-By-Hash 路径映射是否需额外校验逻辑（选择原样透传，路径即校验）

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Feature 仍然是“轻量多仓 CLI 代理”，未引入 Web UI、账号体系或与代理无关的能力。
- 仅使用 Go + 宪法指定依赖；任何新第三方库都已在本计划中说明理由与审核结论。
- 行为完全由 `config.toml` 控制，新增配置项已规划默认值、校验与迁移策略。
- 方案维持缓存优先 + 流式回源路径，并给出命中/回源/失败的日志与观测手段。
- 计划内列出了配置解析、缓存读写、Host Header 路由等强制测试与中文注释交付范围。

**Status**: PASS（未引入新依赖/界面，新增配置仅增加模块枚举与示例）

## Project Structure

### Documentation (this feature)

```text
specs/007-apt-apk-cache/
├── plan.md              # 本文件
├── research.md          # Phase 0 输出
├── data-model.md        # Phase 1 输出
├── quickstart.md        # Phase 1 输出
├── contracts/           # Phase 1 输出
└── tasks.md             # Phase 2 (/speckit.tasks 生成)
```

### Source Code (repository root)
```text
cmd/any-hub/main.go        # CLI 入口、参数解析
internal/config/           # TOML 加载、默认值、校验
internal/server/           # Fiber 服务、路由、中间件
internal/cache/            # 磁盘/内存缓存与 .meta 管理
internal/proxy/            # 上游访问、缓存策略、流式复制
internal/hubmodule/*       # 各模块 Hooks/元数据（新增 debian/apk 模块）
configs/                   # 示例 config.toml（如需）
tests/                     # go test 下的单元/集成测试
```

**Structure Decision**: 新增模块位于 `internal/hubmodule/debian` 或类似命名；复用现有 hook 注册与缓存策略接口；配置扩展在 `internal/config` 追加枚举与默认值。

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | — | — |

## Phase 0: Outline & Research

### Unknowns / Research Tasks
- Acquire-By-Hash 处理策略：是否需要额外校验逻辑 → 选择透传路径并依赖上游校验。
- Alpine APKINDEX 签名校验需求 → 方案为透传签名文件，保持客户端校验。

### Research Output
- `/home/rogee/Projects/any-hub/specs/007-apt-apk-cache/research.md`

## Phase 1: Design & Contracts

### Artifacts
- `/home/rogee/Projects/any-hub/specs/007-apt-apk-cache/data-model.md`
- `/home/rogee/Projects/any-hub/specs/007-apt-apk-cache/contracts/proxy-paths.md`
- `/home/rogee/Projects/any-hub/specs/007-apt-apk-cache/quickstart.md`
- Agent context updated via `.specify/scripts/bash/update-agent-context.sh codex`

### Design Focus
- 定义 APT/APK 路径匹配与缓存策略（索引再验证、包体直接缓存）。
- Acquire-By-Hash/签名文件透传，避免破坏校验。
- 示例配置与测试入口（httptest 上游 + 临时缓存目录）。

## Phase 2: Task Breakdown (deferred to /speckit.tasks)

- 基于用户故事和设计生成 tasks.md（后续命令）
