# Feature Specification: HTTP 服务与单仓代理

**Feature Branch**: `002-fiber-single-proxy`  
**Created**: 2025-11-13  
**Status**: Draft  
**Input**: User description: "HTTP 服务与单仓代理 - 使用 Fiber 搭建 HTTP 服务，支持基于 Host 的路由到单一 Hub。 - 实现文件缓存模块（读写、TTL 检查），完成命中/回源流程。 - 提供 Docker Hub/NPM 任一仓库的最小可用代理，并通过集成测试验证。"

> 宪法对齐（v1.0.0）：
> - 保持“轻量、匿名、CLI 多仓代理”定位：不得引入 Web UI、账号体系或与代理无关的范围。
> - 方案必须基于 Go 1.25+ 单二进制，依赖仅限 Fiber、Viper、Logrus/Lumberjack 及必要标准库。
> - 所有行为由单一 `config.toml` 控制；若需新配置项，需在规范中说明字段、默认值与迁移策略。
> - 设计需维护缓存优先 + 流式传输路径，并描述命中/回源/失败时的日志与观测需求。
> - 验收必须包含配置解析、缓存读写、Host Header 绑定等测试与中文注释交付约束。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Host 路由下的单仓访问 (Priority: P1)

企业内开发者希望通过 `docker.hub.local` 或 `npm.hub.local` 这样的 Host 头访问本地代理，系统需要根据 Host 定位唯一 Hub，并把请求透明转发至上游。

**Why this priority**: 没有稳定的 HTTP 入口和 Host 路由，就无法承载任何代理能力，是 Phase 1 的核心目标。

**Independent Test**: 启动 any-hub，准备含单一 Hub 的配置，使用 `curl -H "Host: docker.hub.local" http://127.0.0.1:5000/v2/_catalog`，验证请求进入正确的 Handler 并记录结构化日志。

**Acceptance Scenarios**:

1. **Given** 配置声明 Hub `docker` 监听端口 5000，**When** 客户端携带 `Host: docker.hub.local` 访问，**Then** Fiber 将请求路由到 docker Hub，并构造正确的上游 URL。
2. **Given** 未声明的 Host，**When** 客户端请求，**Then** 立即返回 404 并在日志中标记 `host_unmapped`。

---

### User Story 2 - 磁盘缓存与回源流程 (Priority: P1)

CI/CD 任务需要重复下载相同镜像或包，期望 any-hub 能在本地缓存结果：命中时直接返回，过期或未命中时回源并刷新缓存，同时保持流式传输。

**Why this priority**: 缓存是代理节省带宽与加速的唯一方式；缺失会让 Phase 1 成为普通转发层。

**Independent Test**: 使用集成测试模拟上游服务器，第一次请求写入缓存，第二次命中缓存并快速返回；设置 TTL 过期后触发 revalidate。

**Acceptance Scenarios**:

1. **Given** 缓存文件存在且未过期，**When** 再次请求相同路径，**Then** 直接从磁盘流式返回，并记录 `cache_hit=true`。
2. **Given** 缓存过期，**When** 新请求到达，**Then** 先向上游发送带条件的请求；若上游 304，则回退本地缓存，若 200 则边回源边写磁盘与客户端。
3. **Given** 回源失败或磁盘写入错误，**Then** 系统返回合理的 5xx 并记录 `cache_hit=false` 与错误原因。

---

### User Story 3 - 最小 Docker/NPM 代理样例 (Priority: P2)

平台运维需要一份可运行的示例，让团队快速验证 Docker 或 NPM 仓库能通过 any-hub 获取常见资源，并在 CI 中运行端到端测试确保回源逻辑可靠。

**Why this priority**: 实际仓库样例可以验证配置、日志、缓存整体流程，也为后续多仓扩展提供可复制模板。

**Independent Test**: 提供 `configs/docker.sample.toml` 或 `configs/npm.sample.toml`，在集成测试中启动临时上游（可模拟 docker registry 或 npm registry），通过 HTTP 调用完成一次拉取并校验缓存目录生成。

**Acceptance Scenarios**:

1. **Given** 示例配置启用 Docker Hub，**When** 运行 quickstart 脚本，**Then** 可以从真实或模拟上游下载一个 manifest 并写入缓存目录。
2. **Given** 示例配置选择 NPM，**When** 执行 `npm view foo` 指向代理，**Then** CLI 能收到正确响应，日志显示命中/回源信息。

### Edge Cases

- 配置监听端口但 Host 头缺失或大小写异常：必须直接返回 404，并在日志中记录 `host_unmapped` 字段，禁止回退默认 Hub。
- 大文件下载中途中断：需要保证写入临时文件并在失败时清理，避免污染缓存。
- TTL 设为 0（永远回源）或非常大：需要解释行为并防止整数溢出。
- 上游返回 4xx/5xx：缓存不得写入，同时应透传状态码。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 系统必须提供基于 Fiber 的 HTTP Server，监听配置声明的端口，并按照 `Host` → `Hub` 的映射路由所有请求。
- **FR-002**: 对于匹配的 Hub，请求路径、查询、方法、Headers 必须重新组装为上游 URL，并保持匿名代理（不注入用户标识）。
- **FR-003**: 缓存模块必须在磁盘上以 `StoragePath/<Hub>/<path>` 结构保存内容，并为每个条目维护元数据（TTL、ETag/Last-Modified、写入时间）。
- **FR-004**: 命中缓存时必须流式读取磁盘并返回；未命中或过期时需边回源边写入磁盘，并在完成前向客户端持续输出，避免全量加载内存。
- **FR-005**: 系统必须支持条件请求：若缓存存在 ETag/Last-Modified，回源时附带 `If-None-Match`/`If-Modified-Since`，收到 304 时回退缓存。
- **FR-006**: 任一请求都要记录结构化日志字段（hub、domain、cache_hit、upstream_status、elapsed_ms），并在错误时附带原因。
- **FR-007**: 提供至少一个 Docker 或 NPM 的示例配置与 quickstart 说明，包含端到端集成测试脚本，证明可从代理获取真实或模拟数据。
- **FR-008**: 所有配置项（端口、Host、TTL、缓存目录）必须在 `config.toml` 中声明，CLI 不引入新的隐式参数。

### Key Entities *(include if feature involves data)*

- **HubRoute**: 映射 Host/端口到具体上游信息，字段包括 `Name`, `Domain`, `Port`, `Upstream`, `Proxy`, `CacheTTL`。
- **CacheEntry**: 表示磁盘缓存文件与 `.meta` 元数据（ETag、Last-Modified、StoredAt、Size、Checksum）。
- **ProxyRequest**: 记录一次代理请求生命周期（原始 URL、Host、缓存命中状态、上游响应码、耗时）。
- **SampleConfig**: 示例配置集合，用于定义 Docker/NPM Hub 所需的字段和默认值。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 针对同一资源的第二次请求延迟较首次下降 ≥70%，且日志显示 `cache_hit=true`。
- **SC-002**: Host 路由能在 100% 测试用例中将请求映射到正确 Hub，未配置的 Host 返回 404，误路由率为 0。
- **SC-003**: 在示例配置下，端到端集成测试成功率达到 100%，并能在 2 分钟内完成一次 docker 或 npm 包的完整拉取。
- **SC-004**: 缓存目录在异常情况下不产生损坏文件，测试覆盖包括中断写入、上游错误、TTL 过期等至少 5 个边界场景。

## Assumptions

- Phase 1 仅需支持单一 Hub 路由；多 Hub 并行将在后续阶段扩展。
- 上游仓库需支持 HTTP/HTTPS GET/HEAD，暂不支持 WebSocket、Chunked 上传等复杂协议。
- 示例代理默认指向公共 Docker Hub；若网络受限，可在 quickstart 中改为模拟上游。
- 磁盘缓存容量和清理策略仍沿用全局配置，不在本次迭代扩展淘汰算法。

## Clarifications

### Session 2025-11-13

- Q: Host 头缺失或未匹配时应如何处理？ → A: 一律返回 404 并记录 `host_unmapped`，不允许回退默认 Hub。
