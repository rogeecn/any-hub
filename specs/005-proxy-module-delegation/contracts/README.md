# Contracts: Proxy Module Delegation

No external API surface changes are introduced by this feature. The core contract is the module handler interface used by the Proxy Dispatcher:

- **Handler Contract (conceptual)**: `Handle(ctx, route)` processes requests for a given route/module, applies module-defined caching/rewrite, and returns structured logs with `module_key`, `hub`, `domain`, `cache_hit`, `upstream_status`, `request_id`.
- **Registration Contract**: Modules must register both metadata and handler; missing handlers must fail fast at startup.

If future external endpoints are added, document them here with request/response schemas.

## Error Behaviors

- **module_handler_missing**: Forwarder无法找到给定 module_key 的 handler 时返回 `500 {"error":"module_handler_missing"}`，并记录 `hub/domain/module_key/rollout_flag` 等日志字段，便于排查配置缺失或注册遗漏。
- **module_handler_panic**: Module handler panic 被捕获后返回 `500 {"error":"module_handler_panic"}`，同时输出结构化日志 `error=module_handler_panic`，防止进程崩溃并提供观测。
