# Data Model: Proxy Module Delegation

## Entities

- **Hub Module**
  - Attributes: `module_key`, `description`, `supported_protocols`, `cache_strategy` (ttl_hint, validation_mode, disk_layout, supports_streaming_write), `locator_rewrite`
  - Behavior: Registers into global registry; binds to a module handler.
  - Constraints: module_key unique; handler must be registered.

- **Module Handler**
  - Attributes: `module_key`, `handle(request, route)` entrypoint; owns cache policy/rewrites; error mapping.
  - Relationships: One-to-one with Hub Module; invoked by Proxy Dispatcher.
  - Constraints: Must produce structured logs (hub, domain, module_key, cache_hit, upstream_status, request_id).

- **Proxy Dispatcher**
  - Attributes: handler map (module_key → handler), default handler fallback.
  - Behavior: Lookup by route.ModuleKey and invoke handler; wraps errors/logging.
  - Constraints: If handler missing, return 5xx with observable logging.

- **Cache Policy**
  - Attributes: `ttl_hint`, `validation_mode`, `disk_layout`, `requires_metadata_file`, `supports_streaming_write`, module-specific locator rewrite.
  - Behavior: Used by module handlers to govern caching and revalidation.

## Relationships

- Hub Module 1:1 Module Handler (per module_key).
- Proxy Dispatcher 1:N Module Handler (registered handlers).
- Module Handler uses Cache Policy defined by its Hub Module.

## Identity & Uniqueness

- `module_key` is the unique identifier for module registration and dispatch.
- Handler map keys must not collide; duplicates cause startup failure.

## State & Lifecycle

- Registration (init/startup) → Handler binding → Runtime dispatch.
- Missing handler or duplicate registration causes startup rejection.

## Volume/Scale Assumptions

- Number of modules small (handful of warehouse types). Handler map fits in memory.
- Cache size/TTL managed per module; relies on existing filesystem cache layout.
