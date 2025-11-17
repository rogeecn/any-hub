# Feature Specification: Proxy Module Delegation

**Feature Branch**: `005-proxy-module-delegation`  
**Created**: 2025-11-17  
**Status**: Draft  
**Input**: User description: "通用的代理模块 internal/proxy 目录中存在大量的if判定对不同代理类型进行特殊化处理，我想要的是在 hubmodule 中各自类型的代理完成自己缓存和代理功能的控制内部管理，外部的proxy只负责把请求分发给不同类型的代理模块，代理调度（ internal/proxy/handler.go ）不存在对任何类型代理的兼容和特殊性处理。请优化这块代码布局和逻辑，分支请使用005开头。"

> 宪法对齐（v1.0.0）：
> - 保持“轻量、匿名、CLI 多仓代理”定位：不得引入 Web UI、账号体系或与代理无关的范围。
> - 方案必须基于 Go 1.25+ 单二进制，依赖仅限 Fiber、Viper、Logrus/Lumberjack 及必要标准库。
> - 所有行为由单一 `config.toml` 控制；若需新配置项，需在规范中说明字段、默认值与迁移策略。
> - 设计需维护缓存优先 + 流式传输路径，并描述命中/回源/失败时的日志与观测需求。
> - 验收必须包含配置解析、缓存读写、Host Header 绑定等测试与中文注释交付约束。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 按模块扩展新仓类型 (Priority: P1)

作为平台工程师，我可以通过新增一个 hubmodule 模块（含元数据与代理实现）注册到系统，而不用改动通用 proxy，即可支持新的仓库类型并完成缓存与回源逻辑。

**Why this priority**: 模块化是本次改造的核心目标，新仓类型落地必须依赖这一能力。

**Independent Test**: 仅新增一个示例模块并注册后，发起请求可完成回源与缓存，且无需修改通用 proxy 代码路径。

**Acceptance Scenarios**:

1. **Given** 新模块已在 hubmodule 注册，且 proxy 注册了对应 handler，**When** 针对该模块域名发起 GET 请求，**Then** 请求被模块逻辑处理并写入缓存，日志包含 module_key。
2. **Given** 新模块没有覆写通用逻辑之外的分支，**When** 第二次请求同一资源，**Then** 返回缓存命中并不触发通用 proxy 中的类型分支。

---

### User Story 2 - 现有仓类型平滑迁移 (Priority: P1)

作为运维人员，我希望 Docker/NPM/PyPI/Composer 等已有仓在改造后行为不变，仍能缓存命中、回源、日志字段齐全。

**Why this priority**: 需要保证功能零回归，维护现网可用性。

**Independent Test**: 对每种仓库重复请求同一资源，两次请求分别观察首次 miss、二次 hit 的头与日志字段。

**Acceptance Scenarios**:

1. **Given** 缓存为空，**When** 首次请求 Docker manifest，**Then** 回源成功、写入缓存，日志包含 cache_hit=false、module_key=docker。
2. **Given** 同一资源已缓存，**When** 第二次请求，**Then** 返回缓存，cache_hit=true 且未触发任何 hub_type 判定错误。

---

### User Story 3 - 统一观测与错误处理 (Priority: P2)

作为 SRE，我希望通用 proxy 仅负责调度，但仍能输出一致的请求日志与错误响应，即使模块缺失或模块内部出错。

**Why this priority**: 观测和故障定位需要统一格式，不因模块化而分裂。

**Independent Test**: 刻意注册缺失 handler 或让模块返回错误，观察通用层日志与响应代码符合约定。

**Acceptance Scenarios**:

1. **Given** 请求路由到未注册 handler 的模块，**When** 发起请求，**Then** 返回 5xx 并记录 module_key、hub、domain、error。
2. **Given** 模块内部返回错误，**When** 请求失败，**Then** 通用层仍输出结构化日志，错误码与 message 对齐错误策略。

---

### Edge Cases

- 如果 hub 的 module_key 未注册或 handler 未绑定，调度层应快速返回可观测的 5xx，并记录 module_key、hub、domain、request_id。
- 模块 handler panic 或超时，通用 proxy 应捕获并返回统一错误响应，避免进程崩溃。
- 配置缺失或模块注册失败时，启动阶段需拒绝加载并给出清晰的报错，而非运行时才暴露。
- 请求方法不被模块支持（如 HEAD/PUT），模块应返回合理状态码，通用层不做类型判断。
- 缓存键或路径重写由模块负责，通用层不应叠加额外重写，避免路径错乱。

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: 通用 proxy 层只负责路由匹配与将请求分发给对应 module handler，不再包含按 hub_type/module 的逻辑分支。
- **FR-002**: hubmodule 为每个仓类型提供独立的 handler/策略入口，包含缓存命中、回源、重写等行为，通用层无需了解细节。
- **FR-003**: Module 注册应同时提供元数据（module_key、支持协议、默认策略）与运行时 handler 绑定；缺失 handler 时请求需返回可观测错误。
- **FR-004**: 现有仓类型（docker、npm、pypi、composer、go）改造后行为与现状等价：首次请求 miss 返回 200+cache miss，二次请求命中缓存且日志字段不变（hub、domain、module_key、cache_hit、upstream_status、elapsed_ms、request_id）。
- **FR-005**: 统一错误与超时处理：模块内部错误或 panic 被捕获并转化为 5xx JSON 响应，日志含 error、module_key、hub、domain。
- **FR-006**: 配置校验/启动流程应在模块注册缺失或重复时直接失败，并输出明确错误信息。
- **FR-007**: 支持新增模块的可插拔模式：新增模块仅需新增 hubmodule 包和 handler 注册，不需要改动通用 proxy 调度代码。
- **FR-008**: 保持现有缓存读写与流式返回路径：缓存命中直接返回，未命中流式回源并写缓存；各模块可自定义缓存键/TTL/验证策略。

### Key Entities

- **Hub Module**: 描述仓类型的元数据和代理行为（缓存策略、路径重写、handler 入口）。
- **Module Handler**: 由模块实现的请求处理器，负责缓存、回源、重写、错误处理。
- **Proxy Dispatcher**: 通用层组件，仅根据路由/module_key 寻址 handler 并执行；提供统一日志、错误包装。
- **Cache Policy**: 由模块定义的 TTL、验证策略和路径布局，用于缓存命中/重写决策。

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 对 docker/npm/pypi/composer/go 各执行“首次 miss、二次 hit”请求链路，返回的 `cache_hit`、`module_key`、`hub`、`domain` 日志字段保持现有值，命中率达到 100% 在重复请求场景。
- **SC-002**: 新增一个示例模块时，无需修改通用 proxy 文件即可完成注册与处理请求，验证请求往返成功且日志包含新 module_key。
- **SC-003**: 缺失 handler 或模块初始化失败时，请求返回 5xx 且日志/响应中的错误码一致，未出现 panic 或进程退出。
- **SC-004**: 改造后对现有仓库类型的端到端请求耗时及成功率与改造前相比吞吐不下降超过 5%，功能回归测试通过。
