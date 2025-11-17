# Tasks: Module Hook Refactor

**Input**: Design documents from `/specs/006-module-hook-refactor/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: 包含配置解析、缓存读写、代理命中/回源、Host Header 绑定、模块 Hook 行为（缺失/异常）的端到端与单元测试。

**Organization**: Tasks are grouped by user story so each delivers independent value.

## Phase 1: Setup

- [X] T001 阅读 spec/plan/research，整理 Hook 目标与迁移范围（specs/006-module-hook-refactor/）
- [X] T002 运行现有基线 `GOCACHE=$(pwd)/.cache/go-build /home/rogee/.local/go/bin/go test ./...`，记录失败用例（当前因 handler 尚未实现 Hook 接口导致编译失败）

---

## Phase 2: Foundational

- [X] T003a 定义 Hook/RequestContext/CachePolicy 接口骨架（internal/proxy/hooks/hooks.go）
- [X] T003b [P] 实现 HookRegistry 注册/查询/重复检测，并暴露 diagnostics 所需状态（internal/proxy/hooks/registry.go, internal/server/routes/modules.go）
- [X] T003c [P] 添加 Hook 契约单元测试（internal/proxy/hooks/hooks_test.go）
- [X] T004 更新 diagnostics 接口，显示注册状态但仍未接入 handler（internal/server/routes/modules.go）
- [X] T005 建立示例模块 Hook（internal/hubmodule/template/, internal/hubmodule/template/module_test.go）
- [X] T006 更新 quickstart/README 说明 Hook 用法（specs/006-module-hook-refactor/quickstart.md, README.md）

**Checkpoint**: Hook 契约与注册机制 ready。

---

## Phase 3: User Story 1 - 定义模块 Hook 契约 (Priority: P1) 🎯 MVP

**Goal**: proxy handler 仅调度 + 缓存读写；Hook 提供各种扩展点。

**Independent Test**: 使用示例模块覆盖路径/缓存策略 → proxy handler 中无 `hub_type` 分支也可完成请求。

- [X] T007 [US1] 重构 handler，接入 Hook 扩展点（internal/proxy/handler.go）
- [X] T008 [P] [US1] 在 forwarder 中注入 Hook/handler 错误处理（internal/proxy/forwarder.go）
- [X] T009 [US1] 编写 Hook 单元测试覆盖缺失/重复/panic 场景（internal/proxy/hooks/, internal/proxy/forwarder_test.go）
- [X] T010 [US1] 更新 diagnostics `/ - /modules` 输出 Hook 状态（internal/server/routes/modules.go, docs）

**Checkpoint**: Hook 契约落地并可验证。

---

## Phase 4: User Story 2 - 迁移现有模块 (Priority: P1)

**Goal**: Docker/NPM/PyPI/Composer/Go 等模块将特化逻辑迁移到 Hook，行为保持等价。

**Independent Test**: 对每个仓执行“第一次 miss、第二次 hit”测试，比对日志与响应头与改造前一致。

- [X] T011 [US2] 迁移 Docker Hook（路径 fallback、内容类型、缓存策略等）（internal/hubmodule/docker/）
- [X] T012 [P] [US2] 迁移 npm Hook（包 metadata、tarball 缓存）（internal/hubmodule/npm/）
- [X] T013 [P] [US2] 迁移 PyPI Hook（simple HTML/JSON 重写、files 路径）（internal/hubmodule/pypi/）
- [X] T014 [P] [US2] 迁移 Composer Hook（packages.json/p2 重写、dist URL）（internal/hubmodule/composer/）
- [X] T015 [US2] 迁移 Go Hook（模组路径、sumdb 重写）（internal/hubmodule/go/）
- [X] T016 [US2] 更新 legacy/default handler 说明及行为（internal/hubmodule/legacy/, docs）
- [X] T017 [US2] 为每个模块增加/更新 e2e 测试覆盖 miss/hit 及日志字段（tests/integration/*）

**Checkpoint**: 现有仓库 Hook 化并通过回归。

---

## Phase 5: User Story 3 - 清理 legacy 逻辑并增强观测 (Priority: P2)

**Goal**: proxy handler 统一错误路径；legacy 仅兜底并在诊断输出 legacy-only。

**Independent Test**: 模拟缺失 handler 或 Hook panic，返回 `module_handler_missing`/`module_handler_panic`，日志含 hub/domain/module_key/request_id。

- [X] T018 [US3] 实现 handler 缺失/重复时启动失败与运行期 5xx 响应（internal/proxy/forwarder.go, internal/hubmodule/registry.go）
- [X] T019 [P] [US3] 添加 Hook panic 捕获与结构化日志（internal/proxy/forwarder.go）
- [X] T020 [US3] 扩展 diagnostics 与日志写入，使 legacy-only 模块可观测（internal/server/routes/modules.go, internal/logging/fields.go）
- [X] T021 [P] [US3] 更新文档/quickstart，描述错误处理与 legacy-only 标记（specs/006-module-hook-refactor/contracts/README.md, quickstart.md）

**Checkpoint**: Hook 错误与 legacy 观测全面覆盖。

---

## Phase 6: Polish & Cross-Cutting

- [X] T022 整理 README/DEVELOPMENT 文档、样例配置，指导如何创建 Hook（README.md, configs/config.example.toml）
- [X] T023 [P] 最终 `gofmt` + `GOCACHE=$(pwd)/.cache/go-build /home/rogee/.local/go/bin/go test ./...`，确保无回归

---

## Dependencies & Order

1. Phase1 Setup → Phase2 Hook 契约 → Phase3 (US1) → Phase4 (US2) → Phase5 (US3) → Phase6 polish。
2. US2 依赖 Hook 契约完成；US3 依赖 US1/US2。

## Parallel Execution Examples

- T012/T013/T014/T015（各模块 Hook）可并行，互不干扰。
- 文档更新（T016/T021/T022）可与测试任务并行。
- Hook 契约（T003）完成后，可并行推进 diagnostics (T010) 与 forwarder 错误处理 (T008)。

## Implementation Strategy

- **MVP**：完成 US1（T007-T010）即可让 proxy handler 不再依赖类型分支。
- **迭代**：依次迁移模块 (US2) 并加强观测 (US3)；每阶段运行 `go test ./...`。
- **验证**：每个模块迁移后执行“Miss→Hit”回归 + 特殊错误场景测试。
