# Contract: Host 路由 + 缓存代理

## Request Flow
1. Client sends HTTP request to `http://<listen-host>:<port>/<path>` with `Host: <hub-domain>`.
2. Fiber middleware resolves HubRoute; if missing → 404 JSON `{ "error": "host_unmapped" }`.
3. Proxy handler checks disk cache key = `<hub-name>/<path>`.
4. Cache hit (Valid): stream file to client with stored headers.
5. Cache stale/miss: build upstream request `Upstream + path`, attach `If-None-Match`/`If-Modified-Since` when available, stream response back; on 200 store body to cache.

## Required Headers
- Host (required) → determines Hub.
- `X-Any-Hub-Upstream` (response) – actual upstream URL (debugging).
- `X-Any-Hub-Cache-Hit` (response) – `true/false`.
- `X-Request-ID` (response) – correlation id for logs.

## Error Codes
| Scenario | Status | Body |
|----------|--------|------|
| Host 未配置 | 404 | `{ "error": "host_unmapped" }` |
| 上游 4xx/5xx | mirror | 上游 body 原样透传 |
| 缓存写入失败 | 502 | `{ "error": "cache_write_failed" }` |

## Logging Fields
```
{
  "action": "proxy",
  "hub": "docker",
  "domain": "docker.hub.local",
  "cache_hit": true,
  "upstream_status": 200,
  "elapsed_ms": 123,
  "error": ""
}
```
