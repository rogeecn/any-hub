# Feature Specification: Hub 配置凭证字段

**Feature Branch**: `003-hub-auth-fields`  
**Created**: 2025-11-14  
**Status**: Draft  
**Input**: User description: "Hub 配置中添加 Username/Password/type类型 三个字段，user:password是为了保证上游需要认证时，下游无需认证即可获取，如docker需要登录后可以解除rate-limit；type用于指定代理的仓库类型，用于区分镜像仓库类型为不同类型的仓库进行定制化操作。当前分支名需要003开头。"

> 宪法对齐（v1.0.0）：
> - 保持“轻量、匿名、CLI 多仓代理”定位：不得引入 Web UI、账号体系或与代理无关的范围。
> - 方案必须基于 Go 1.25+ 单二进制，依赖仅限 Fiber、Viper、Logrus/Lumberjack 及必要标准库。
> - 所有行为由单一 `config.toml` 控制；若需新配置项，需在规范中说明字段、默认值与迁移策略。
> - 设计需维护缓存优先 + 流式传输路径，并描述命中/回源/失败时的日志与观测需求。
> - 验收必须包含配置解析、缓存读写、Host Header 绑定等测试与中文注释交付约束。

## Clarifications

### Session 2025-11-14

- Q: 当 `Username` 与 `Password` 只提供其中一个时应如何处理？ → A: 在 `config --check-config` 阶段直接判定为配置错误，要求两个字段要么同时提供要么一起缺省。

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - 配置上游凭证 (Priority: P1)

运维人员需要在 `config.toml` 中为特定 Hub 写入上游所需的 `Username`/`Password`，以便在代理启动后自动携带凭证，解除匿名拉取的速率限制或访问受保护的仓库。

**Why this priority**: 没有可配置的凭证字段就无法满足受限仓库或需要登录的上游，代理将丧失关键价值。

**Independent Test**: 仅依赖配置与 CLI 启动，即可通过加载带有不同凭证的配置文件并观察日志/校验错误来验证。

**Acceptance Scenarios**:

1. **Given** Hub 缺省 `Username`/`Password`，**When** CLI 读取配置，**Then** 字段可为空并保持当前匿名行为。
2. **Given** Hub 提供 `Username=foo`、`Password=bar`，**When** CLI 运行 `--check-config`，**Then** 配置校验通过且凭证不会在日志中明文打印。
3. **Given** Hub 配置凭证，**When** upstream 需要 Basic/Bearer 登录，**Then** 代理转发请求时自动附带凭证，终端可观察到 rate-limit 不再触发。

---

### User Story 2 - 下游透明体验 (Priority: P1)

下游开发者希望继续使用匿名方式访问代理，而代理应在后台代为认证；同一个代理可针对不同 Hub 自动切换凭证与仓库类型。

**Why this priority**: 透明体验是“单点代理”的核心卖点；若仍需下游显式登录，就无法简化 CI/CD。

**Independent Test**: 使用 curl/npm/docker 命令只指定 Host/端口即可验证，无需改动其他系统。

**Acceptance Scenarios**:

1. **Given** Hub 已配置凭证，**When** 下游客户端不带任何 Authorization header，**Then** 代理回源时自行插入 `Username`/`Password` 并返回 200 响应。
2. **Given** 多个 Hub 配置了不同仓库类型，**When** 同一 CLI 进程同时监听多个端口，**Then** 每个端口都能在日志中打印正确的 `hub`、`type`、`auth_mode` 字段。

---

### User Story 3 - 仓库类型适配 (Priority: P2)

平台需要在配置中声明 `Type`（如 `docker`、`npm`、`generic-http`），以区分仓库协议并触发未来的类型特定策略（Header 处理、元数据）。

**Why this priority**: 明确的仓库类型是后续分支（镜像仓库 vs. 包仓库）实施自定义逻辑的前提；越早保存该字段，越容易扩展。

**Independent Test**: 通过配置不同的 Type 值，启动 CLI 并验证日志/metrics 中输出的类型以及针对未知类型的回退策略。

**Acceptance Scenarios**:

1. **Given** Hub `Type=docker`、`npm` 或 `go`，**When** 代理启动，**Then** 日志/配置校验会表明使用对应策略，非法值或缺失将被拒绝并提示可选列表。
2. **Given** Hub 未按要求设置 `Type`，**When** CLI 运行或 `--check-config`，**Then** 返回校验错误并指明必须从受支持列表中选择（docker/npm/go，未来扩展）。

### Edge Cases

- 当 `Username` 或 `Password` 单独出现时，系统必须在配置校验阶段阻止并提示需同时提供。
- 当 `Type` 与实际上游协议不符（如标记为 docker 但 upstream 为 npm）时，需确保仍可 fallback 到透传并输出警告。
- 凭证旋转：在 CLI 重启前如何避免旧凭证仍被缓存？需记录文档告知必须重启或触发热加载。
- 凭证日志安全：所有日志中禁止打印纯文本密码，且结构化日志只显示是否启用凭证。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Hub 配置必须支持新增 `Username`、`Password` 字段，字段为完全可选，若未配置则保持匿名透传；若任意一项填写则必须两项同时提供（允许为空字符串），否则 `config --check-config` 需报错并拒绝启动。
- **FR-002**: CLI 在读取配置后必须将凭证存入运行期内存，不得在 `logger` 输出中展示明文（只可显示存在性或掩码）。
- **FR-003**: 代理回源请求必须根据 Hub 配置决定是否附加 Authorization header；缺省模式保持现状（匿名）。
- **FR-004**: 当 upstream 返回 401/429 且 Hub 配置了凭证，系统必须重试一次（遵循既有重试策略）并记录“凭证认证失败”的错误字段，便于排障。
- **FR-005**: Hub 配置需新增必填 `Type` 字段，短期支持值列表（`docker`, `npm`, `go`），配置校验必须拒绝缺失值或列表外值，并提示“仅支持 docker/npm/go，未来将扩展 apt/yum/composer 等”。
- **FR-006**: CLI/日志/metrics 必须输出 `hub_type`、`auth_mode`（anonymous/credentialed）等字段，方便观察不同仓库行为，并在新增类型时无需额外字段即可扩展。
- **FR-007**: 配置校验必须阻止缺失 `Type` 的 Hub，通过显式错误消息引导用户选择受支持值；内部设计需保留枚举扩展点，便于未来加入 apt/yum/composer/go proxy 等类型。
- **FR-008**: 文档与示例配置需覆盖凭证字段的写法、敏感信息处理方式及最佳实践（例如不要提交到 Git），并说明 type 字段的当前支持列表及扩展策略。
- **FR-009**: 测试覆盖需包括：配置解析（含非法组合）、凭证透传的集成测试、以及 Type 值驱动的分支逻辑。

### Key Entities *(include if feature involves data)*

- **HubConfig**: 描述单个代理 Hub 的所有属性，新增 `Username`、`Password`（可选字符串，按 Hub 粒度保存）、`Type`（枚举，默认 `generic-http`）。与 HubRegistry 绑定。
- **AuthProfile**: 运行期派生实体，包含凭证存在性、认证模式、最后一次认证状态等，并与日志/metrics 关联。
- **HubType**: 表示仓库协议类别（docker、npm、generic-http 等），驱动特定 Header、缓存策略或 future features。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% 的受保护仓库（需要 Basic/Bearer 凭证）可在下游匿名请求下成功响应，且无需再配置下游凭证。
- **SC-002**: 代理日志中 `auth_mode=credentialed` 的请求，其 401/429 错误率较匿名模式下降 ≥80%（以相同上游的历史基线为对照）。
- **SC-003**: 所有 Hub 配置在 `go test ./internal/config` 覆盖下均能校验通过，新增字段的解析、默认值和错误路径达到 100% 单元测试覆盖。
- **SC-004**: 文档与 quickstart 示例更新完成后，内部演练中 3 名运维工程师可在 15 分钟内配置并验证 Docker、NPM 仓库的登录代理。
