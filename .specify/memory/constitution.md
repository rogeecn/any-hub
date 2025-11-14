<!--
Sync Impact Report
Version change: template → 1.0.0
Modified principles:
- Template principle slot 1 → I. 轻量多仓代理使命
- Template principle slot 2 → II. Go 静态单体与受控依赖
- Template principle slot 3 → III. 单一 TOML 控制平面
- Template principle slot 4 → IV. 缓存优先与流式传输路径
- Template principle slot 5 → V. 可观测 + 中文文档驱动交付
Added sections:
- 系统约束与质量标准
- 开发流程与质量门禁
Removed sections:
- None
Templates requiring updates:
- ✅ .specify/templates/plan-template.md
- ✅ .specify/templates/spec-template.md
- ✅ .specify/templates/tasks-template.md
Follow-up TODOs:
- None
-->
# any-hub Constitution

## Core Principles

### I. 轻量多仓代理使命
- 所有需求必须服务“轻量、匿名、命令行部署”的定位：禁止引入 Web UI、账号体系或与代理无关的配套系统。
- 每个功能都要证明可提升多仓代理体验（更快缓存、多协议覆盖、离线可靠性），否则不得进入路线图。
- 优先简单、可维护的实现；复杂拓展（如多租户、插件）需保持可拆卸，供用户二次开发。

### II. Go 静态单体与受控依赖
- 仅允许使用 Go 1.25+ 构建单一静态可执行文件，运行方式限定为 CLI + systemd/supervisor 托管。
- 核心依赖限定为 Fiber v3（HTTP 服务）、Viper（配置）、Logrus + Lumberjack（日志）；新增第三方库需经治理层评审并写明理由。
- 代码结构固定为 `cmd/any-hub` 入口 + `internal/config|server|cache|proxy` 模块；优先标准库与简单结构体组合，避免过度接口化。

### III. 单一 TOML 控制平面
- 所有运行行为由单个 `config.toml` 驱动，可通过 `ANY_HUB_CONFIG` 指定路径，但禁止多来源配置或隐藏的环境开关。
- 全局段必须声明日志、缓存、重试、内存等策略；每个 `[[Hub]]` 必须同时提供 `Name/Domain/Port/Upstream`，并显式绑定监听端口与 Host 头。
- 新增特性若需配置项，必须在 `internal/config` 定义默认值、校验逻辑与文档；配置变更需提供迁移/兼容策略。

### IV. 缓存优先与流式传输路径
- 请求处理流程固定：命中且未过期 → 直接回缓存；命中过期 → 先做 ETag/Last-Modified 验证；未命中或验证需回源时，边写客户端边刷新缓存。
- 缓存目录必须遵循 `StoragePath/<Hub>/<path>`，并配套 `.meta` 记录校验信息与 TTL；改动需说明迁移策略及磁盘兼容性。
- 回源逻辑必须实现有限次退避重试并记录命中/回源/失败轨迹；任何实现都必须控制内存占用（默认小于 256MB），禁止整文件读入内存。

### V. 可观测 + 中文文档驱动交付
- 每个请求都要输出结构化日志字段：Hub、Domain、命中状态、上游响应码、耗时、错误原因，支持文件滚动或 stdout。
- 关键结构体、并发与缓存算法必须使用中文注释解释意图与使用方式，保持小团队可维护性。
- 测试范围必须覆盖配置解析、缓存读写、代理命中/回源流程、Host Header 绑定校验；缺失测试视为阻断项。

## 系统约束与质量标准

- **模块边界**：`cmd/any-hub` 只负责启动与参数解析；`internal/config` 负责 TOML 加载、默认值与校验；`internal/server` 构建 Fiber 服务、路由与中间件；`internal/cache` 管理磁盘/内存缓存与元数据；`internal/proxy` 负责上游请求、缓存回退与流式复制。新增目录需说明与现有模块的依赖关系。
- **配置 Schema**：全局键固定为 `LogLevel`、`LogFilePath`、`LogMaxSize`、`LogMaxBackups`、`LogCompress`、`StoragePath`、`CacheTTL`、`MaxMemoryCacheSize`、`MaxRetries`、`InitialBackoff`、`UpstreamTimeout`。`[[Hub]]` 必须声明 `Name`、`Domain`、`Port`、`Upstream`，可选 `Proxy`、`CacheTTL` 覆盖与特定能力开关。
- **缓存策略**：所有请求首先查询本地缓存；命中 + 未过期直接返回；命中过期需先 HEAD 验证；回源 200 需同步写缓存与客户端；304 时回退缓存；失败时透传状态码并记录日志。
- **重试与性能**：仅对可重试错误（5xx、网络超时）执行受配置控制的退避重试；每次请求需记录命中率与耗时，以形成容量与性能基线。
- **内存与磁盘限制**：`MaxMemoryCacheSize` 约束所有内存缓冲；大文件必须直接落盘；磁盘目录需支持清理任务与容量保护。
- **安全与运维**：进程仅以前台 CLI 运行；所有输入路径必须清洗以防目录遍历；Host 头需验证；可选 `/-/health` 检查；日志支持文件滚动与匿名访问环境。

## 开发流程与质量门禁

1. **阶段化交付**：遵循宪法中的阶段路线图——Phase 0（配置与骨架）→ Phase 1（HTTP 与单仓代理）→ Phase 2（多仓与域名绑定）→ Phase 3（可靠性）→ Phase 4（长期维护），任何新需求都需说明所处阶段和完成标准。
2. **工件要求**：`plan.md` 必须在 Phase 0 前通过“Constitution Check”，证明未引入 UI、未新增未审核依赖且遵守单一配置；`spec.md` 的用户故事需独立可测并呼应缓存/匿名代理场景；`tasks.md` 需按用户故事分组并包含配置、缓存、代理与可观测性任务。
3. **测试与文档 Gate**：在合入前需证明 `go test ./...` 覆盖配置、缓存、代理核心路径；新增模块必须附带中文注释与 README/DEVELOPMENT 补充说明；缺失日志或测试即视为质量门禁失败。
4. **运行验证**：任何新功能必须提供示例配置或说明如何通过 `config.toml` 打开；PR 中需附带日志样例或说明如何验证命中/回源路径；需要健康检查或运维动作时需同步更新 `DEVELOPMENT.md`。

## Governance

- 宪法优先级高于其他流程文档；所有 PR、spec、plan、tasks 都必须显式确认与本宪法一致才能进入评审。
- 修订流程：提出者需在 PR 描述中列出变更动机、影响范围、迁移/验证计划，并同步更新受影响的模板或运行文档；获批后方可更新版本号与 Ratified/Amended 记录。
- 版本管理：使用语义化版本——新增原则或重大治理变化 → MAJOR；新增章节或扩展指导 → MINOR；措辞澄清/错别字 → PATCH。每次修订都要在 Sync Impact Report 中记录变动。
- 合规复核：在每次发布前进行一次宪法对照检查（plan/spec/tasks +关键代码）；至少每季度对日志、配置与缓存策略进行抽检，确保与最新宪法一致。

**Version**: 1.0.0 | **Ratified**: 2025-11-13 | **Last Amended**: 2025-11-13
