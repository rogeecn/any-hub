---

description: "Tasks for Hub 配置凭证字段"
---

# Tasks: Hub 配置凭证字段

**Input**: Design documents from `/specs/003-hub-auth-fields/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: 宪法 v1.0.0 要求覆盖配置解析、缓存读写、代理命中/回源与 Host Header 绑定，此清单在相关阶段加入对应测试任务。

**Organization**: Tasks are grouped by user story以支持独立实施与验证。

## Format: `[ID] [P?] [Story] Description`

- **[P]**: 可并行执行（触达不同文件/无直接依赖）
- **[Story]**: 指明所属用户故事（Setup/Foundational/Polish 阶段不写）
- 描述中必须包含精确文件路径

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 记录迁移策略、同步文档，让团队在单端口与凭证范围上达成一致。

- [X] T001 将全局 `ListenPort`/Hub 凭证迁移指南写入 `DEVELOPMENT.md` 与 `README.md`，提醒去除 `[[Hub]].Port`
- [X] T002 补充 `CHANGELOG.md` 与 `specs/003-hub-auth-fields/quickstart.md` 的前置说明，列出新的配置字段及基本验证命令

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 改造配置与 server 启动流程，确保所有 Hub 共用单端口并具备新字段；完成前禁止进入任何用户故事。

- [X] T003 更新 `internal/config/types.go` / `internal/config/loader.go`，为 `GlobalConfig` 添加 `ListenPort`，为 `HubConfig` 添加 `Username`/`Password`/`Type`，并删除 Hub 级 `Port`
- [X] T004 扩充 `internal/config/validation.go` 与 `internal/config/config_test.go`，覆盖 `ListenPort` 范围校验、Type 枚举校验及缺失 Type 报错
- [X] T005 重构 `cmd/any-hub/main.go` 与 `internal/server/router.go`，仅根据全局 `ListenPort` 启动 Fiber，并在加载阶段检测到 Hub 级 `Port` 时报告迁移错误
- [X] T006 [P] 更新 `internal/server/hub_registry.go` 及 `internal/server/hub_registry_test.go`，让 Host Registry 仅依赖 Host/Host:port 组合，去除对 per-Hub 端口的引用
- [X] T007 [P] 调整 `configs/config.example.toml`、`configs/docker.sample.toml`、`configs/npm.sample.toml`，移除 Hub 端口字段并添加 `ListenPort`/`Type` 示例

**Checkpoint**: 单端口 + 新字段的配置/路由路径已可运行，各用户故事可并行推进。

---

## Phase 3: User Story 1 - 配置上游凭证 (Priority: P1) 🎯 MVP

**Goal**: Hub 可配置上游凭证，CLI 回源时自动附带 Authorization，日志只输出掩码。

**Independent Test**: 使用带凭证的 `config.toml` 运行 `any-hub --check-config` 与 `go test ./tests/integration -run CredentialProxy`，无须下游凭证即可解除 rate-limit。

### Tests for User Story 1

- [X] T008 [P] [US1] 在 `tests/integration/credential_proxy_test.go` 构建带 Basic Auth 的 upstream stub，验证“无凭证失败→配置凭证成功”的流程

### Implementation for User Story 1

- [X] T009 [US1] 在 `internal/config/types.go` 与 `cmd/any-hub/main.go` 中保存可选凭证，并在所有 `logrus` 输出中仅显示掩码/存在性
- [X] T010 [US1] 修改 `internal/proxy/handler.go`，依据 Hub 凭证自动附加 Authorization header，并在 401/429 时执行一次受控重试与错误字段记录
- [X] T011 [US1] 更新 `README.md` 与 `quickstart.md`，新增凭证写法、敏感信息注意事项以及 `any-hub --check-config` 示例

**Checkpoint**: 代理可凭借配置凭证访问受限仓库，下游仍保持匿名体验。

---

## Phase 4: User Story 2 - 下游透明体验 (Priority: P1)

**Goal**: 确保下游无需配置凭证即可访问，日志/观测字段体现 `auth_mode`、`hub_type` 与上游结果。

**Independent Test**: 运行 `tests/integration/credential_proxy_test.go` 的匿名客户端用例，并以 `npm --registry http://127.0.0.1:<ListenPort>` 手动验证日志输出。

### Tests for User Story 2

- [X] T012 [P] [US2] 扩展 `tests/integration/credential_proxy_test.go`，加入“不带 Authorization 仍命中凭证”与日志字段断言

### Implementation for User Story 2

- [X] T013 [US2] 在 `internal/logging/fields.go` 与 `internal/proxy/handler.go` 中输出 `hub_type`、`auth_mode`、`upstream_status`，并确保缓存命中/回源路径均记录
- [X] T014 [US2] 在 `tests/integration/upstream_stub_test.go` 及 `quickstart.md` 中补充匿名客户端示例命令，指导如何验证透明代理

**Checkpoint**: 日志、quickstart 与匿名客户端流程可完整验证下游体验。

---

## Phase 5: User Story 3 - 仓库类型适配 (Priority: P2)

**Goal**: 强制声明 Hub `Type`（docker/npm/go），日志与运行期可识别类型，为未来扩展留接口。

**Independent Test**: 使用覆盖三种 Type 的配置运行 `any-hub --check-config` 与 `go test ./internal/config`，非法/缺失 Type 会被拒绝，日志中能看到正确的 `hub_type`。

### Tests for User Story 3

- [X] T015 [P] [US3] 在 `internal/config/config_test.go` 新增表驱动测试，覆盖合法 Type、非法 Type、缺失 Type 的报错/提示

### Implementation for User Story 3

- [X] T016 [US3] 在 `internal/proxy/handler.go` 与 `internal/server/router.go` 加入 `switch Type`，目前仅设置日志/标签，并对未支持类型抛出明确错误
- [X] T017 [US3] 更新 `quickstart.md` 及 `configs/*.sample.toml`，列出 Type 可选值与未来扩展策略

**Checkpoint**: Hub 类型被强制校验，日志和示例配置均反映正确值。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 统一测试、文档及残留引用，确保产物满足宪法门槛。

- [X] T018 [P] 运行 `gofmt ./cmd ./internal ./tests` 与 `GOCACHE=/tmp/go-build go test ./...`，并把命令写入 `DEVELOPMENT.md`
- [X] T019 清理 `DEVELOPMENT.md`、`README.md`、`specs/003-hub-auth-fields/plan.md` 中残留的 Hub 端口描述，确保只推荐全局 `ListenPort`
- [X] T020 在 `CHANGELOG.md` 与 `quickstart.md` 中记录演练结果（docker + npm），并附一次手动验证日志

---

## Dependencies & Execution Order

### Phase Dependencies

1. Setup → Foundational → 所有用户故事 → Polish
2. Foundational 完成前，任何用户故事不得启动。
3. US2 依赖 US1 产出的凭证注入，但可以在实现层并行。
4. US3 仅依赖 Foundational，可与 US1/US2 并行实施。

### User Story Dependencies

- US1 完成后，即可单独交付 MVP。
- US2 依赖 US1 的日志/凭证输出，但测试可提前编写。
- US3 与其他故事仅在配置层相互作用，无硬依赖。

### Parallel Opportunities

- T006 与 T007 可由不同成员分别处理（registry vs. 示例配置）。
- T008/T012/T015 测试任务都可在实现前准备并行执行。
- 不同用户故事的实现（T009- T017）可由独立小组并行推进。
- Polish 阶段 T018 与 T019/T020 可交由不同成员并行完成。

---

## Implementation Strategy

### MVP First (User Story 1)
1. 完成 Setup + Foundational，确认单端口与配置路径无误。
2. 实施 US1（凭证字段 + 代理注入）并通过集成测试。
3. 以此为最小可交付版本，供受限仓库使用。

### Incremental Delivery
1. Increment 1: Setup + Foundational + US1 → 解锁凭证代理。
2. Increment 2: US2 → 强化透明体验与观测性。
3. Increment 3: US3 → 引入 Type 校验及扩展思路。
4. Polish: 全量测试、文档与 quickstart 验证。

### Parallel Team Strategy
- Team A：负责 Foundational + US1。
- Team B：在 Foundational 完成后并行 US2（日志 + quickstart）。
- Team C：并行 US3（Type 校验与示例）。
- Polish 阶段由值班成员统一收尾。
