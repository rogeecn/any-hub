# Data Model: HTTP 服务与单仓代理

## Entities

### HubRoute
- **Description**: Host/端口到上游仓库的映射，供 Fiber 路由和 Proxy handler 使用。
- **Fields**: `Name` (string, unique), `Domain` (string, FQDN), `Port` (int, 1-65535), `Upstream` (URL), `Proxy` (URL, optional), `CacheTTL` (duration override), `EnableHeadCheck` (bool).
- **Validation**: Name 唯一；Domain 不含协议/路径；Upstream 必须 http/https。
- **Relationships**: 由 config 加载到 `HubRegistry`；与 CacheEntry、ProxyRequest 通过 `Name` 关联。

### HubRegistry
- **Description**: 运行期内存结构，按 (Port, Host) 查找 HubRoute。
- **Fields**: `routes map[key]HubRoute`, key = `port:host`；支持默认端口匹配。
- **Operations**: `Lookup(host, port)`, `List()`；返回结果提供给 Fiber 中间件。

### CacheEntry
- **Description**: 表示磁盘缓存的一个对象（正文 + 元数据）。
- **Fields**: `HubName`, `Path`, `FilePath`, `MetaPath`, `ETag`, `LastModified`, `StoredAt`, `TTL`, `Size`, `Checksum`。
- **State**:
  1. `Empty`: 未缓存
  2. `Valid`: TTL 内、可直接返回
  3. `Stale`: TTL 过期，需 revalidate
  4. `Invalid`: 写入失败或上游错误，需删除

### ProxyRequest
- **Description**: 一次请求生命周期的可观测性数据。
- **Fields**: `ID`, `HubName`, `Host`, `Path`, `Method`, `CacheHit` (bool), `UpstreamStatus`, `LatencyMs`, `Error`。
- **Usage**: 记录日志/metrics，未来可扩展 tracing。

### SampleConfig
- **Description**: 示例配置集（Docker/NPM）。
- **Fields**: `Type` (docker|npm), `Domain`, `Port`, `Upstream`, `Proxy`, `Notes`。
- **Purpose**: quickstart & integration tests复用，证明真实配置长什么样。
