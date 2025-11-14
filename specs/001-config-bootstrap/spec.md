# Feature Specification: 配置与骨架

**Feature Branch**: `001-config-bootstrap`  
**Created**: 2025-11-13  
**Status**: Draft  
**Input**: User description: "：配置与骨架 - 实现 internal/config，覆盖 TOML 加载、默认值、校验及测试。 - 初始化 cmd/any-hub，解析 --config、--check-config、--version。 - 建立 logrus 初始化流程，完成基础日志输出。"

> 宪法对齐（v1.0.0）：
> - 保持“轻量、匿名、CLI 多仓代理”定位：不得引入 Web UI、账号体系或与代理无关的范围。
> - 方案必须基于 Go 1.25+ 单二进制，依赖仅限 Fiber、Viper、Logrus/Lumberjack 及必要标准库。
> - 所有行为由单一 `config.toml` 控制；若需新配置项，需在规范中说明字段、默认值与迁移策略。
> - 设计需维护缓存优先 + 流式传输路径，并描述命中/回源/失败时的日志与观测需求。
> - 验收必须包含配置解析、缓存读写、Host Header 绑定等测试与中文注释交付约束。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 运维人员校验配置 (Priority: P1)

运维人员需要在启动 any-hub 之前验证 `config.toml` 是否完整、字段合法，以便在 CI/CD 和本地快速发现错误配置，避免生产中断。

**Why this priority**: 若配置错误无法在启动前阻断，将直接导致多仓代理不可用，是 Phase 0 的最小可行能力。

**Independent Test**: 提供一份带有缺失字段或非法取值的配置，通过 `--check-config` 运行，命令应返回错误并指出问题；提供一份合格配置则应返回成功状态。

**Acceptance Scenarios**:

1. **Given** 配置文件缺少必填字段，**When** 运维运行 `any-hub --check-config --config broken.toml`，**Then** 命令立即失败并展示缺失字段及修复指引。
2. **Given** 配置文件全部合法，**When** 运维运行 `any-hub --check-config --config valid.toml`，**Then** 命令成功退出并记录“配置有效”的日志。

---

### User Story 2 - CLI 操作者加载配置并启动 (Priority: P1)

CLI 操作者需要使用 `--config` 标志加载默认或指定路径的 `config.toml`，在无额外 UI 的环境中完成 any-hub 启动，并通过 `--version` 确认二进制身份。

**Why this priority**: Phase 0 要求可运行的入口；若 CLI 无法正确加载配置或识别版本，后续阶段均无法进行。

**Independent Test**: 准备默认路径 `config.toml` 与自定义路径 `custom.toml`，分别运行 `any-hub` 并确认应用读取正确文件、输出版本信息、在缺失文件时提供明确错误。

**Acceptance Scenarios**:

1. **Given** 默认路径存在配置文件，**When** 操作者直接运行 `any-hub`，**Then** 程序打印版本信息、载入默认配置并开始初始化。
2. **Given** 自定义配置路径，**When** 操作者运行 `any-hub --config /tmp/custom.toml`，**Then** 程序使用该路径并在日志中记录“正在读取 /tmp/custom.toml”。
3. **Given** 用户想快速确认版本，**When** 运行 `any-hub --version`，**Then** 输出语义化版本并立即退出。

---

### User Story 3 - 观察日志确保运行健康 (Priority: P2)

SRE 希望 any-hub 在启动和运行阶段输出结构化、可预测的日志，用以判断配置来源、监听端口和缓存策略，支持文件或 stdout 并尊重配置中的日志级别。

**Why this priority**: Phase 0 需具备排障手段；若无一致日志，无法判断配置成效或升级问题。

**Independent Test**: 通过配置项将日志写入文件与 stdout，触发一次成功的配置校验和一次失败案例，验证日志包含时间、级别、Hub 名称/域名、动作结果等字段。

**Acceptance Scenarios**:

1. **Given** 配置中启用文件日志，**When** 程序启动，**Then** 在文件中生成带时间戳、级别、消息字段的日志并滚动管理。
2. **Given** 配置校验失败，**When** 运维重试 `--check-config`，**Then** 日志包含校验错误详情和“阻止启动”提示，帮助快速定位问题。

### Edge Cases

- 配置文件缺失或路径不可读时，需要输出可操作指引（例如“请提供 --config 或在当前目录创建 config.toml”）。
- TOML 语法错误或类型不匹配时，必须指出具体行列与字段，避免笼统报错。
- 日志路径无写权限或磁盘填满时，应用需自动退回 stdout 并提示权限/容量问题。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统必须提供 `--config` 标志，默认读取工作目录下的 `config.toml`，并在日志中记录最终使用的路径。
- **FR-002**: 系统必须提供 `--check-config` 模式，在不启动服务器的情况下完成配置加载、默认值合并与字段校验，并以退出码区分成功/失败。
- **FR-003**: 系统必须提供 `--version` 标志，输出语义化版本号（含 Git 提交或构建信息）后立即退出且不执行其他逻辑。
- **FR-004**: 配置加载器必须支持全局与 `[[Hub]]` 段的必填字段校验，自动补齐默认值并返回结构化错误，且覆盖单元测试。
- **FR-005**: 日志初始化流程必须尊重配置中的级别/输出位置，支持 stdout 及文件滚动策略，且在任何模式（校验/启动）中启用结构化日志字段。
- **FR-006**: 当配置合法时，系统必须输出一条“配置验证通过”或“启动完成”的信息；当配置非法时，必须输出明确的字段及原因，方便用户修复。

### Key Entities *(include if feature involves data)*

- **Configuration Profile**: 由全局段（日志、缓存、重试、内存限制）及一个或多个 Hub 条目组成，需保证 `Name/Domain/Port/Upstream` 等必填项存在，并允许可选字段继承或覆盖默认值。
- **CLI Flag Set**: 运行入口可解析 `--config`、`--check-config`、`--version` 等标志；不同组合需决定流程（例如校验模式不启动服务器、版本模式直接退出）。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% 的非法 `config.toml` 在 `--check-config` 阶段被阻断，并输出至少一条包含字段名与错误原因的日志。
- **SC-002**: 在默认与自定义路径情况下，`any-hub` 启动时间不超过 2 秒即可完成配置加载并打印版本信息。
- **SC-003**: 至少 95% 的启动/校验日志包含时间、级别、动作、配置路径等标准字段，可直接用于排障。
- **SC-004**: CLI 操作者能够在 1 次运行内确认版本号与配置状态，相关指令的使用步骤在 README/DEVELOPMENT 中有据可查。

## Assumptions

- 默认配置文件路径为仓库根目录或部署目录下的 `config.toml`；若环境变量 `ANY_HUB_CONFIG` 已存在，则 CLI 标志优先级高于环境变量。
- 日志默认输出到 stdout，且仅在配置显式开启时写入文件，避免初期部署的磁盘依赖。
- 版本号信息可通过构建流程注入，若缺失则回退为“unknown”。
