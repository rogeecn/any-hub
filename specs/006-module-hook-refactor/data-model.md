# Data Model: Module Hook Refactor

## Entities

- **ModuleHook**
  - Attributes: `module_key`, `normalize_path`, `resolve_upstream`, `rewrite_response`, `cache_policy`, `content_type`（函数指针/接口）。
  - Behavior: 由模块在 init() 或启动阶段注册；proxy handler 在请求生命周期调用。
  - Constraints: 所有函数可选；若未实现则 fallback 到通用逻辑；禁止造成路径逃逸或空 handler。

- **HookRegistry**
  - Attributes: `map[module_key]ModuleHook`、并发安全读写锁。
  - Behavior: 提供 `Register`, `MustRegister`, `Fetch`；在启动时验证唯一性。
  - Constraints: module_key 小写唯一；重复注册报错。

- **LegacyHandler**
  - Attributes: 使用旧行为的 handler（默认缓存策略、路径重写）。
  - Behavior: 作为默认 handler；Hook 缺失时退回，并在日志/诊断中标记。

- **ProxyDispatcher**
  - Attributes: handler map（module_key→handler），默认 handler，日志指针。
  - Behavior: lookup handler → 调用并做错误捕获；缺失时返回 `module_handler_missing`。

- **Diagnostics Snapshot**
  - Attributes: 模块元数据 + Hook 状态（`registered`/`legacy`/`missing`）。
  - Behavior: `/ - /modules` 接口读取 HookRegistry 与 HubRegistry，生成 JSON。

## Relationships

- Hub module 注册时同时在 HookRegistry 与 Forwarder handler map 建立关联。
- ProxyDispatcher 在请求进入后根据 route.Module.Key（来自 Hub Type）查询 Hook + handler。
- Diagnostics 依赖 HookRegistry 与 HubRegistry 联合输出状态。

## Lifecycle

1. 启动：加载 `config.toml` → 初始化 HookRegistry（legacy 默认） → 模块 init() 注册 Hook。
2. 运行时：请求 → Dispatcher 查找 handler + Hook → 调用 Hook 执行特定逻辑 → 通用缓存/回源流程。
3. 诊断：`/-/modules` 读取当前 Hook 状态并输出。
