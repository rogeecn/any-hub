# Research: Proxy Module Delegation

## Decisions

- **模块调度边界**: 通用 proxy 仅负责路由到 module handler，所有缓存/回源/重写由模块内实现。
  - **Rationale**: 减少跨类型分支，新增仓无需改通用层，风险集中在各自模块。
  - **Alternatives**: 保留通用 handler + 按类型分支（现状）；放弃模块化，统一代码路径。均会牵扯多处改动且增加回归面，故弃用。

- **模块注册契约**: hubmodule 注册需同时提供元数据和 handler 绑定接口；缺 handler 视为配置错误。
  - **Rationale**: 避免运行期静默回落，统一观测与错误路径。
  - **Alternatives**: 允许回退到 legacy handler；但会掩盖缺失，违背模块自洽目标。

- **现有仓兼容**: docker/npm/pypi/go/composer 模块保留各自策略与路径重写，迁移时不得改变对外响应/日志字段。
  - **Rationale**: 避免生产回归；满足 SC-001/SC-004。
  - **Alternatives**: 统一一套通用策略；但会改变缓存键/TTL，引入不必要风险。

## Clarifications Resolved

- None pending; spec中无 NEEDS CLARIFICATION。
