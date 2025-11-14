---

description: "Task list for feature 001-config-bootstrap"
---

# Tasks: 配置与骨架

**Input**: Design documents from `/specs/001-config-bootstrap/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: 宪法 v1.0.0 要求覆盖配置解析、CLI 流程与日志可观测性，本任务列表默认包含相应单元/集成测试。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Ensure repository has the baseline artifacts and documentation required for Phase 0 delivery.

- [X] T001 Create canonical sample config包含全局段与单个 Hub 的默认值在 `configs/config.example.toml`
- [X] T002 Document Phase 0 bootstrap prerequisites（Go 版本、依赖、命令）于 `DEVELOPMENT.md`
- [X] T003 [P] Link quickstart入口与 CLI 使用章节到 `README.md`，方便新成员找到 `--check-config`/`--version`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Provide shared config/CLI scaffolding and fixtures required by all user stories.

- [X] T004 定义 `Config`/`GlobalConfig`/`HubConfig` 结构与默认常量于 `internal/config/types.go`
- [X] T005 实现基础加载器（读取 TOML、合并默认值）骨架于 `internal/config/loader.go`
- [X] T006 [P] 抽象校验错误类型与帮助函数（含字段路径）于 `internal/config/errors.go`
- [X] T007 建立 `internal/config/testdata/{valid,missing}.toml` 及通用测试 helper 于 `internal/config/test_helpers_test.go`
- [X] T008 为 CLI 提供统一 flag 解析入口（占位逻辑）于 `cmd/any-hub/main.go`

**Checkpoint**: Config 模型、加载骨架与 CLI flag 入口完成后，用户故事可并行推进。

---

## Phase 3: User Story 1 - 运维人员校验配置 (Priority: P1) 🎯 MVP

**Goal**: 任何非法 `config.toml` 都能在 `--check-config` 阶段被阻断，同时输出明确字段与修复建议。

**Independent Test**: 使用 `go test ./cmd/any-hub -run TestCheckConfig*` 和 `go test ./internal/config`，验证缺失字段/类型错误会失败且成功配置通过。

### Tests for User Story 1

- [X] T009 [P] [US1] 编写缺失字段/类型错误用例，确保 `LoadConfig` 返回结构化错误（`internal/config/loader_test.go`）
- [X] T010 [P] [US1] 编写 CLI 集成测试：执行 `--check-config` 并断言退出码/日志（`cmd/any-hub/main_test.go`）

### Implementation for User Story 1

- [X] T011 [US1] 完成默认值合并与字段级校验逻辑（路径/数值/唯一性）于 `internal/config/validation.go`
- [X] T012 [US1] 将 `--check-config` flag 与新校验逻辑接线，并返回对应退出码于 `cmd/any-hub/main.go`
- [X] T013 [US1] 定义用户可读的错误消息与中文注释，更新 `internal/config/errors.go`
- [X] T014 [US1] 更新 `quickstart.md` 与 `README.md`，列出 `--check-config` 使用示例与常见错误修复

**Checkpoint**: `any-hub --check-config` 可以独立验证配置且输出标准日志。

---

## Phase 4: User Story 2 - CLI 操作者加载配置并启动 (Priority: P1)

**Goal**: CLI 能根据 flag/环境/默认顺序加载配置，启动流程打印版本与配置来源，并提供 `--version` 快速查询。

**Independent Test**: 通过 `cmd/any-hub/main_test.go` 的 flag 优先级测试以及 `cmd/any-hub/version_test.go` 的版本输出测试验证。

### Tests for User Story 2

- [X] T015 [P] [US2] 编写 flag vs. 环境变量 vs. 默认路径的优先级测试（`cmd/any-hub/main_test.go`）
- [X] T016 [P] [US2] 为 `--version` 输出添加测试，断言语义化字符串与退出行为（`cmd/any-hub/version_test.go`）

### Implementation for User Story 2

- [X] T017 [US2] 实现配置路径解析顺序（flag > `ANY_HUB_CONFIG` > 默认），并在日志中记录来源（`cmd/any-hub/main.go`）
- [X] T018 [US2] 实装 `--version` 逻辑（含构建信息注入）于 `cmd/any-hub/version.go`
- [X] T019 [US2] 在正常启动路径中输出版本号、监听端口与配置路径（结构化日志）于 `cmd/any-hub/main.go`
- [X] T020 [US2] 更新 `DEVELOPMENT.md` 与 `README.md` 的 CLI 章节，描述 flag 组合与退出码

**Checkpoint**: `any-hub` 可直接启动/加载配置，`--version` 即时返回信息。

---

## Phase 5: User Story 3 - 观察日志确保运行健康 (Priority: P2)

**Goal**: 启动与校验流程都能输出结构化日志，支持 stdout/文件滚动，并在写文件失败时自动回退。

**Independent Test**: 通过 `internal/logging/logger_test.go`（模拟文件权限/滚动策略）与 `cmd/any-hub/logging_integration_test.go`（验证 stdout 回退）。

### Tests for User Story 3

- [X] T021 [P] [US3] 创建 logger 单元测试：验证级别/输出配置与字段注入（`internal/logging/logger_test.go`）
- [X] T022 [P] [US3] 编写集成测试覆盖文件不可写时的 stdout 回退（`cmd/any-hub/logging_integration_test.go`）

### Implementation for User Story 3

- [X] T023 [US3] 新增 `internal/logging/logger.go`，根据配置初始化 Logrus + Lumberjack，并暴露 `InitLogger`
- [X] T024 [US3] 在 `cmd/any-hub/main.go` 的校验与启动路径调用 `InitLogger`，并注入 `action/configPath/result` 字段

- [X] T025 [US3] 添加日志字段构建与公共 helper（`internal/logging/fields.go`），确保包含 hub/domain/命中状态
- [X] T026 [US3] 更新 `quickstart.md`/`DEVELOPMENT.md`，记录日志配置字段与排障步骤

**Checkpoint**: 日志可根据配置切换输出，并提供足够字段支持排障。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finishing touches that ensure maintainability and documentation alignment.

- [X] T027 运行 `gofmt`/`go test ./...` 并将结果记录到 `DEVELOPMENT.md` 的验证段
- [X] T028 为关键结构与算法补充中文注释，覆盖 `internal/config`、`cmd/any-hub`、`internal/logging`
- [X] T029 [P] 更新 `CHANGELOG.md`（若存在）与 `README.md`，概述 Phase 0 能力及后续路线

---

## Dependencies & Execution Order

### Phase Dependencies
- Phase 1 → Phase 2 → User Stories (US1/US2/US3) → Phase 6

### User Story Dependencies
- US1 和 US2 都依赖 Phase 2；US3 依赖 US1/US2 的 CLI/logging 接口完成后再启
- US1 是 MVP（配置校验），需先完成以解锁后续部署

### Parallel Execution Examples
- **US1**: T009 与 T010 可并行编写测试；实现任务 T011/T012 需等测试框架 ready
- **US2**: T015 与 T016 可同测；T017/T018 可并行后共同驱动 T019
- **US3**: T021 与 T022 可并行；T023/T025 可并行后统一在 T024 接线

---

## Implementation Strategy

### MVP First (User Story 1 Only)
1. 完成 Phase 1-2
2. 交付 US1（配置校验 + CLI check），并通过 quickstart 测试
3. 可在此阶段发布 CLI 校验版本，供 CI 使用

### Incremental Delivery
1. MVP（US1）完成后，向 CLI 启动/版本（US2）演进
2. 最后实现日志观测（US3），再进入 Polish

### Parallel Team Strategy
- 团队 A：聚焦 `internal/config`（US1）
- 团队 B：并行处理 CLI flag/版本（US2）
- 团队 C：在 CLI 接口稳定后实现日志（US3）
