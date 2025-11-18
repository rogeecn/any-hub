# Data Model: APT/APK 包缓存模块

## Entities

### HubConfig (APT/APK)
- Fields: `Name`, `Domain`, `Port`, `Upstream`, `Type` (`debian`/`apk`), optional `CacheTTL` override.
- Rules: `Type` 必须新增枚举；`Upstream` 必须为 HTTP/HTTPS；每个 Hub 独立缓存前缀。

### IndexFile
- Represents: APT Release/InRelease/Packages*，Alpine APKINDEX。
- Attributes: `Path`, `ETag` (if present), `Last-Modified`, `Hash` (if provided in index), `Size`, `ContentType`.
- Rules: RequireRevalidate=true；缓存命中需携带条件请求；内容不得修改。

### PackageFile
- Represents: APT `pool/*/*.deb` 包体，Alpine `packages/<arch>/*.apk`。
- Attributes: `Path`, `Size`, `Hash` (from upstream metadata if available), `StoredAt`.
- Rules: 视作不可变；AllowCache/AllowStore=true；不做本地内容改写。

### SignatureFile
- Represents: APT `Release.gpg`/`InRelease` 自带签名，Alpine APKINDEX 签名文件。
- Attributes: `Path`, `Size`, `StoredAt`.
- Rules: 原样透传；与对应 IndexFile 绑定，同步缓存与再验证。

### CacheEntry
- Represents: 本地缓存记录（索引或包体）。
- Attributes: `LocatorPath`, `ModTime`, `ETag`, `Size`, `AllowStore`, `RequireRevalidate`.
- Rules: 读取时决定是否回源；写入需同步 `.meta`；路径格式 `StoragePath/<Hub>/<Path>`.

## Relationships
- HubConfig → IndexFile/PackageFile/SignatureFile 通过 Domain/Upstream 绑定。
- IndexFile ↔ SignatureFile：同一索引文件的签名文件需同周期缓存与再验证。
- CacheEntry 聚合 IndexFile/PackageFile/SignatureFile 的落盘信息，用于策略判定。

## State/Transitions
- CacheEntry states: `miss` → `fetched` → `validated` (304) / `refreshed` (200) → `stale` (TTL 或上游 200 新内容) → `removed` (404 或清理)。
- Transitions triggered by upstream响应：304 保持内容，200 刷新，404 删除相关缓存。
