---

description: "Task list for HTTP 服务与单仓代理"
---

# Tasks: HTTP 服务与单仓代理

**Input**: Design documents from `/specs/002-fiber-single-proxy/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: 宪法 v1.0.0 要求覆盖配置解析、Host Header 路由、缓存读写、条件回源与示例集成测试，本任务清单默认包含相应测试项。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 准备文档与目录结构，确保团队对 Phase 1 范围有统一认知。

- [X] T001 更新 `DEVELOPMENT.md` 的 Phase 1 章节，描述 HTTP 服务/缓存迭代目标
- [X] T002 在 `README.md` 添加 “单仓代理 (Phase 1)” 小节并链接到 spec/plan
- [X] T003 [P] 创建 `internal/server/` 与 `internal/cache/` 目录说明文件（`doc.go`）概述职责

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 构建 Host→Hub 注册表、HTTP 客户端和缓存存储基础，所有用户故事依赖这些能力。

- [X] T004 实现 `internal/server/hub_registry.go`，从配置构建 Host/端口→Hub 映射并提供查询 API
- [X] T005 [P] 在 `internal/server/http_client.go` 创建共享上游 HTTP 客户端（含超时、Proxy、Header 透传）
- [X] T006 设计 `internal/cache/store.go` 接口 + 文件布局（`StoragePath/<hub>/<path>` 与 `.meta`）
- [X] T007 [P] 在 `tests/integration/upstream_stub_test.go` 搭建可复用的模拟上游服务器（Docker/NPM 两种路径）

**Checkpoint**: Registry、HTTP 客户端、缓存存储与测试桩可用，方可进入用户故事实现。

---

## Phase 3: User Story 1 - Host 路由下的单仓访问 (Priority: P1) 🎯 MVP

**Goal**: Fiber 服务可根据 Host/端口匹配到唯一 Hub，并将请求透明转发。

**Independent Test**: 使用 `curl -H "Host: docker.hub.local"` 对本地服务发起请求，结合 httptest 断言正确 Hub、上游 URL 和日志字段。

### Tests for User Story 1

- [X] T008 [P] [US1] 编写路由单元测试：`internal/server/router_test.go` 覆盖 Host 命中/未配置/默认逻辑
- [X] T009 [P] [US1] 添加集成测试：`tests/integration/host_routing_test.go` 验证端口+Host 组合处理

### Implementation for User Story 1

- [X] T010 [US1] 构建 Fiber App & 中间件（请求日志、错误捕获）于 `internal/server/router.go`
- [X] T011 [US1] 在 `cmd/any-hub/main.go` 接线 server 启动逻辑，传入 registry + HTTP 客户端
- [X] T012 [US1] 为 Host 未命中添加 404 响应与日志字段（`internal/server/router.go`）
- [X] T013 [US1] 更新 `quickstart.md`，记录如何使用 Host 头访问单仓代理

**Checkpoint**: CLI 可启动 HTTP 服务并完成 Host→Hub 路由，日志含 action/host 字段。

---

## Phase 4: User Story 2 - 磁盘缓存与回源流程 (Priority: P1)

**Goal**: 在磁盘上缓存上游响应，支持 TTL、条件请求与流式读写。

**Independent Test**: 使用模拟上游写入缓存后再次请求，观察命中与 304 流程；覆盖写入失败、上游错误等场景。

### Tests for User Story 2

- [X] T014 [P] [US2] 为 `internal/cache/store_test.go` 添加命中/未命中/TTL 过期测试
- [X] T015 [P] [US2] 在 `tests/integration/cache_flow_test.go` 编写端到端测试（首次写入、304 回退、上游失败）

### Implementation for User Story 2

- [X] T016 [US2] 在 `internal/cache/store.go` 实现读/写/元数据 API，并处理并发写入/临时文件
- [X] T017 [US2] 在 `internal/proxy/handler.go` 中实现缓存流程（命中→读磁盘，未命中→回源→写缓存→流式返回）
- [X] T018 [US2] 支持条件请求 Header（`If-None-Match`/`If-Modified-Since`）并处理 304（`internal/proxy/upstream.go`）
- [X] T019 [US2] 扩展日志字段，记录 `cache_hit`、`upstream_status`、`elapsed_ms`（`internal/logging/fields.go` + `internal/proxy/handler.go`）
- [X] T020 [US2] 在 `config.example.toml` 增加缓存路径/TTL 示例，并更新 `DEVELOPMENT.md` 的缓存调优段落

**Checkpoint**: 缓存命中率可在日志中观察；回源路径流式返回并具备条件请求能力。

---

## Phase 5: User Story 3 - 最小 Docker/NPM 代理样例 (Priority: P2)

**Goal**: 提供可复制的示例配置与 quickstart 脚本，验证 Docker 或 NPM 仓库的代理行为。

**Independent Test**: 运行文档中的 quickstart，对真实或模拟上游完成一次包/镜像拉取，并验证缓存目录写入。

### Tests for User Story 3

- [X] T021 [P] [US3] 编写示例集成测试 `tests/integration/docker_sample_test.go`（或 `npm_sample_test.go`）验证端到端流程

### Implementation for User Story 3

- [X] T022 [US3] 添加 `configs/docker.sample.toml` 与 `configs/npm.sample.toml`，注释必要字段
- [X] T023 [US3] 在 `quickstart.md` 新增示例步骤（Docker/NPM）、常见问题与日志示例
- [X] T024 [US3] 准备脚本或 Make 目标 `scripts/demo-proxy.sh` 运行示例配置
- [X] T025 [US3] 补充 README “示例代理” 章节链接 demo 脚本与 quickstart

**Checkpoint**: 示例配置 & 文档可指导用户完成最小代理体验，并由自动化测试验证。

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: 收尾文档、CI 与质量保证，确保 Phase 1 可长期维护。

- [X] T026 在 `cmd/any-hub/main.go` 与 `internal/*` 新增中文注释，解释 server/cache 关键流程
- [X] T027 [P] 增加 `tests/integration/interrupt_test.go`，验证下载中断后的缓存清理
- [X] T028 更新 `CHANGELOG.md` 与 `README.md`，记录 Phase 1 完成情况
- [X] T029 运行 `gofmt`、`go test ./...`，并在 `DEVELOPMENT.md` 记录验证结果与命令

---

## Dependencies & Execution Order

### Phase Dependencies
- Phase 1 → Phase 2 → User Stories (US1/US2/US3) → Phase 6

### User Story Dependencies
- US1 (Host 路由) 依赖 Phase 2 完整；US2 依赖 US1 提供的 Fiber/Proxy 框架；US3 依赖 US1+US2 的代理能力

### Parallel Execution Examples
- **US1**: T008 与 T009（测试）可并行；T010/T011 实现后可由 T012/T013 跟进
- **US2**: T014/T015 可并行；缓存实现 T016 可与日志扩展 T019 同步推进
- **US3**: T022（配置）与 T024（脚本）可并行，测试 T021 需在示例完成后执行

---

## Implementation Strategy

### MVP First (User Story 1 Only)
1. 完成 Phase 1-2
2. 交付 Host 路由 + HTTP 服务（US1），即可演示基础代理能力

### Incremental Delivery
1. 在 MVP 基础上继续 US2（缓存）→ US3（示例）
2. 每个阶段完成后运行 quickstart + 集成测试，确保可独立交付

### Parallel Team Strategy
- 团队 A：负责 server/router（US1）
- 团队 B：负责 cache/proxy（US2）
- 团队 C：负责示例配置与文档（US3）
