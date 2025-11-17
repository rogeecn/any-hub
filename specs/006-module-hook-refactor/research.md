# Research: Module Hook Refactor

## Decisions

- **Hook Contract Scope**: 模块 Hook 将覆盖 5 个扩展点（路径/locator 重写、上游 URL 解析、响应重写、缓存策略、内容类型推断），并提供统一注册 API。
  - *Rationale*: 这些环节是当前 `handler.go` 中所有类型分支的来源；一次性覆盖可保证 proxy handler 只保留调度/缓存写入。
  - *Alternatives*: 仅重写部分（如响应）会导致残余分支；完全改写为“每模块自己的 ProxyHandler”则重复缓存代码，风险更高。

- **Legacy Handler Role**: 保留 legacy handler 作为默认兜底（未迁移模块或外部插件），但日志/诊断会标记为 `legacy-only`。
  - *Rationale*: 确保迁移期间功能可用，同时提示 SRE 识别未迁移模块。
  - *Alternatives*: 强制所有模块一次迁移；风险大且不利于渐进上线。

- **Diagnostics Visibility**: `/ - /modules` 输出增加 Hook 状态（已注册/缺失/legacy 默认），并用于 SRE 排查。
  - *Rationale*: 迁移阶段需要监控 Hook 注册情况，避免静默回退。
  - *Alternatives*: 单靠日志搜索，排查效率低。

- **Testing Approach**: 每个模块迁移后需要端到端“Miss → Hit”回归，同时新增 Hook 单元测试覆盖缺失/panic 等异常路径。
  - *Rationale*: 确认行为等价且新错误处理生效。
  - *Alternatives*: 仅依赖现有集成测试，不足以验证 Hook 入口。

## Clarifications

- 无须额外澄清；假设各模块团队可修改其包并维护 Hook 实现。
