# Contracts: Module Hook Refactor

## `/ - /modules` Diagnostics

- **Purpose**: 列出所有模块的 metadata 与 Hook 注册状态，SRE 可检查模块是否迁移到 Hook 模式。
- **Response Additions**:
  - `hook_status`: `registered | legacy-only | missing`
  - `handler_status`: `ok | missing | panic`
- **Usage**: SRE 通过 `curl http://host:port/-/modules` 观察所有模块状态；缺失 Hook 或 handler 时需在日志与响应中同步体现。

## Error Responses

- `module_handler_missing`: 500 JSON `{ "error": "module_handler_missing" }`
- `module_handler_panic`: 500 JSON `{ "error": "module_handler_panic" }`

这些错误需出现在日志中并附带 `hub/domain/module_key/request_id`。
