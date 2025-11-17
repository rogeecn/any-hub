# Feature Specification: Module Hook Refactor

**Feature Branch**: `006-module-hook-refactor`  
**Created**: 2025-11-17  
**Status**: Draft  
**Input**: User description: "模块内部自管理缓存/代理逻辑、proxy handler 仅负责调度"

> 宪法对齐（v1.0.0）：
> - 保持 CLI 多仓代理定位，不引入 UI 或账号体系。
> - 仅依赖 Go 1.25+ 单二进制及 Fiber/Viper/Logrus/Lumberjack/标准库，不新增无关依赖。
> - 全局 `config.toml` 控制所有行为；若新增配置项需描述字段、默认值、验证与迁移。
> - 缓存优先 + 流式回源是基础能力；必须定义命中/回源/失败时的结构化日志与观测策略。
> - 验收需覆盖配置解析、缓存读写、Host Header 绑定及中文注释要求。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 定义模块 Hook 契约 (Priority: P1)

作为平台架构师，我可以定义一套模块 Hook/Handler 契约，使缓存键、路径重写、上游解析、响应重写等逻辑都由模块实现，proxy handler 只负责调度与共享的缓存写入。

**Why this priority**: 没有统一 Hook，就无法让模块自控逻辑，后续迁移无法落地。

**Independent Test**: 提供一个示例模块实现 Hook，注册后能独立覆盖缓存策略/路径重写，proxy handler 不含类型分支仍可处理请求。

**Acceptance Scenarios**:

1. **Given** 定义好 Hook 接口与注册机制，**When** 模块注册自定义 Hook，**Then** proxy handler 在日志/缓存流程中调用模块 Hook，代码中不再出现 `hub_type` 分支。
2. **Given** 模块缺少 Hook，**When** 注册时，**Then** 启动/测试阶段给出明确错误提示，防止运行期回退到旧逻辑。

---

### User Story 2 - 迁移现有模块 (Priority: P1)

作为平台工程师，我希望 Docker/NPM/PyPI/Composer/Go 等模块全部迁移到 Hook 模式，自行管理缓存与代理逻辑，保证行为与改造前一致。

**Why this priority**: 生产仓库必须可用，迁移后必须验证命中/回源与日志字段无差异。

**Independent Test**: 对每个仓库执行“首次 miss、二次 hit”流程并观察日志字段，与改造前对比一致。

**Acceptance Scenarios**:

1. **Given** Docker 模块已实现 Hook，**When** 请求 manifest/层文件，**Then** 路径重写、缓存键、内容类型判断都由模块完成，proxy handler 不含 Docker 特化。
2. **Given** PyPI/Composer 模块已迁移，**When** 请求 simple index、packages.json、dist 文件，**Then** 响应重写与内容类型准确，日志字段/缓存命中行为与现状一致。

---

### User Story 3 - 清理 legacy 逻辑并增强观测 (Priority: P2)

作为 SRE，我希望 proxy handler 只提供通用调度和错误包装；模块未注册或 Hook panic 时能输出统一 5xx 与结构化日志；legacy 模块成为纯兜底实现。

**Why this priority**: 防止运行期静默回退，简化排查路径。

**Independent Test**: 刻意注入未注册模块或 Hook panic，观察返回 `module_handler_missing`/`module_handler_panic` 错误及日志字段完整。

**Acceptance Scenarios**:

1. **Given** 模块未注册 Hook，**When** 发起请求，**Then** proxy handler 返回 5xx JSON 并记录 hub/module_key/request_id 等字段。
2. **Given** Hook panic，**When** 请求执行，**Then** panic 被捕获，返回统一错误并在日志中包含 panic 信息。

---

### Edge Cases

- 模块未注册或重复注册时需在启动阶段失败，避免运行期 fallback。
- Hook 返回非法路径/URL 时需防止逃逸缓存目录或访问非预期上游。
- 不同模块并行迁移时，需保证 legacy 模块仍可作为默认 handler。
- Hook 不得影响 diagnostics (`/-/modules`) 或健康检查路径。
- 模块 Hook 需兼容 HEAD/GET；不支持方法应返回合规状态码。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 定义模块 Hook/Handler 契约，覆盖路径重写、缓存策略、上游解析、响应重写、内容类型推断等扩展点，proxy handler 不再含 `hub_type` 分支。
- **FR-002**: 提供 Hook 注册/验证机制，要求模块在注册时同时提供元数据与 Hook；缺失或重复时启动失败并输出明确日志。
- **FR-003**: 将 Docker/NPM/PyPI/Composer/Go 模块现有特化逻辑全部迁移到各自 Hook，实现缓存键/TTL/验证、路径 fallback、响应重写的等价行为。
- **FR-004**: legacy/default 模块作为兜底 handler，确保未迁移模块仍可运行，但会记录“legacy-only”状态，便于观测。
- **FR-005**: proxy handler 仅负责调度、缓存读写和日志包装；模块 Hook panic 或缺失时返回统一 5xx JSON（`module_handler_panic`/`module_handler_missing`），日志包含 hub/domain/module_key/request_id。
- **FR-006**: Diagnostics (`/-/modules`) 需展示模块 Hook 注册状态（正常/缺失），并保持现有输出字段。
- **FR-007**: 文档/quickstart 更新，说明如何实现 Hook、注册模块，以及如何验证新模块缓存/日志。

### Key Entities

- **Module Hook**: 一组可选函数（normalize path、resolve upstream、rewrite response、cache policy、content type）。
- **Module Registration**: 绑定 module_key、元数据、Hook handler 的机构，负责唯一性与完整性校验。
- **Proxy Dispatcher**: 使用 module_key→Hook/handler map 调度请求，输出统一日志与错误。

### Assumptions

- 现有模块的缓存策略及接口稳定，可迁移到 Hook 而无需额外外部依赖。
- 模块团队可接受在各自包内实现 Hook；无需跨团队共享逻辑。

## Success Criteria *(mandatory)*

- **SC-001**: proxy handler 代码中不包含任何 `hub_type`/类型特化分支；静态分析或代码审查确认类型判断被完全移除。
- **SC-002**: 对 docker/npm/pypi/composer/go 每个仓执行“首次 miss、二次 hit”测试，首/次响应头与日志字段与改造前一致，功能回归通过。
- **SC-003**: 新增模块仅需在 hubmodule 中实现 Hook 并注册，无需修改 proxy handler；示例模块演示该流程并通过集成测试。
- **SC-004**: 缺失 handler 或 Hook panic 时返回统一 5xx JSON，日志包含 hub/domain/module_key/request_id，错误率控制在 0（测试场景）。
- **SC-005**: `/ - /modules` 诊断接口展示所有模块 Hook 状态，SRE 可识别缺失或 legacy-only 模块；与文档描述一致。
