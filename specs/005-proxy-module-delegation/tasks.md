# Tasks: Proxy Module Delegation

**Input**: Design documents from `/specs/005-proxy-module-delegation/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: 验收需覆盖配置校验、缓存读写、代理命中/回源、Host Header 绑定与结构化日志字段；针对各 user story 的验证可用现有 `go test` + integration stubs 完成。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 确认范围与基线，确保现有测试可运行

- [X] T001 复核规范/计划/研究，记录约束与目标到 specs/005-proxy-module-delegation/plan.md
- [X] T002 在仓库根目录执行基线 `go test ./...` 并记录当前失败用例（如有）

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 梳理现有调度与接口约束，定义模块化契约

- [X] T003 分析并记录现有 hub_type 分支与 dispatch 流程（internal/proxy/handler.go, internal/server/router.go），明确移除点
- [X] T004 [P] 定义模块 handler 契约与注册接口（internal/proxy/forwarder.go 或新建 internal/proxy/module.go）供后续模块复用

**Checkpoint**: Foundational ready - user story implementation can now begin

---

## Phase 3: User Story 1 - 按模块扩展新仓类型 (Priority: P1) 🎯 MVP

**Goal**: 通用 proxy 只做分发，新增模块无需改通用分支即可处理缓存/回源

**Independent Test**: 引入一个新模块（示例 stub），注册 handler 后单独请求能回源并缓存，日志含 module_key，通用层无类型分支

### Implementation for User Story 1

- [X] T005 [US1] 重构 dispatch 层仅依赖 module handler map，移除 hub_type 判断（internal/proxy/forwarder.go, internal/server/router.go）
- [X] T006 [P] [US1] 增加模块注册助手，要求元数据+handler 同时注册并校验唯一性（internal/proxy/module_contract.go 等）
- [X] T007 [P] [US1] 调整主入口绑定默认/legacy handler，并为新模块预留挂载点（cmd/any-hub/main.go, internal/proxy/forwarder.go）
- [X] T008 [US1] 更新 quickstart 说明新增模块的步骤与示例配置（specs/005-proxy-module-delegation/quickstart.md）

**Checkpoint**: User Story 1 independently testable

---

## Phase 4: User Story 2 - 现有仓类型平滑迁移 (Priority: P1)

**Goal**: Docker/NPM/PyPI/Composer/Go 行为与日志字段保持不变，缓存命中逻辑等价

**Independent Test**: 每个仓同一路径请求两次：首次 miss 写缓存，二次 hit，日志含 hub/domain/module_key/cache_hit/upstream_status/request_id

### Implementation for User Story 2

- [X] T009 [US2] 迁移 Docker 模块到新 handler/注册模式，保持路径重写与缓存策略不变（internal/hubmodule/docker/, internal/proxy/* 如需）
- [X] T010 [P] [US2] 迁移 npm/pypi/go/composer/legacy 模块到新模式并保持兼容（internal/hubmodule/{npm,pypi,go,composer,legacy}/）
- [X] T011 [US2] 更新配置加载/校验默认值及示例，缺失 handler/重复模块时报错（internal/config/loader.go, internal/config/validation.go, config.example.toml）
- [X] T012 [P] [US2] 更新/新增集成场景确保日志与缓存行为等价（tests/integration/*, internal/proxy/forwarder.go 观测字段）

**Checkpoint**: User Stories 1 AND 2 independently functional

---

## Phase 5: User Story 3 - 统一观测与错误处理 (Priority: P2)

**Goal**: 缺失 handler 或模块内部错误时输出一致的 5xx 与结构化日志

**Independent Test**: 触发未注册 handler、模块 panic/超时，统一返回 5xx JSON，日志包含 error/module_key/hub/domain/request_id

### Implementation for User Story 3

- [X] T013 [US3] 实现缺失或重复 handler 的快速失败响应与日志（internal/proxy/forwarder.go, internal/server/router.go）
- [X] T014 [P] [US3] 为模块 handler 调用增加 panic/错误捕获与统一错误映射（internal/proxy/forwarder.go, internal/proxy/handler.go）
- [X] T015 [US3] 补充观测性字段与文档说明，确保错误/命中日志一致（internal/logging/, docs/operations 或 specs/005-proxy-module-delegation/contracts/README.md）

**Checkpoint**: All user stories independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 文档、格式化与回归收尾

- [ ] T016 更新 feature 文档/README，说明新模块化调度与使用方式（specs/005-proxy-module-delegation/contracts/README.md, README.md）
- [ ] T017 [P] 运行 gofmt/go test 并记录结果（仓库根目录）

---

## Dependencies & Order

- Phase 1 → Phase 2 → Phase 3 (US1) → Phase 4 (US2) → Phase 5 (US3) → Phase 6
- User stories: US1 完成后再进行 US2；US3 依赖 US1/US2 的调度能力和兼容性验证

## Parallel Execution Examples

- T004 与 T003 可并行（定义接口与调研现状分离）。  
- 在 US2 中，各模块迁移（T009/T010）可分拆并行。  
- 日志/错误强化（T014）可与文档更新（T015/T016）并行。

## Implementation Strategy

- **MVP**: 完成 US1（T005-T008）即可验证模块化分发与新增模块示例。  
- **Incremental**: 先迁移现有仓（US2），再补齐统一错误/观测（US3），最后收尾文档与回归。  
- 持续运行 `go test ./...` 于每阶段完成后，确保回归可靠。
