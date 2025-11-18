# Research Notes: APT/APK 包缓存模块

## Decision Records

### 1) Acquire-By-Hash 处理
- **Decision**: 路径原样透传，缓存按完整路径存储；不做额外本地哈希校验，交由上游与客户端校验。
- **Rationale**: APT 自带哈希校验，路径即校验信息；本地重复计算增加 CPU/IO 成本且风险与上游标准重复。
- **Alternatives**: 额外本地哈希校验并拒绝不匹配（增加复杂度、可能与上游行为不一致）；跳过缓存（失去加速价值）。

### 2) APT 索引再验证策略
- **Decision**: Release/InRelease/Packages* 请求统一带 If-None-Match/If-Modified-Since，缓存 RequireRevalidate=true；命中 304 继续用缓存，200 刷新。
- **Rationale**: 与现有代理模式一致，确保“latest” 索引及时更新，避免 stale。
- **Alternatives**: 固定 TTL 不再验证（风险：索引过期）；强制每次全量 GET（浪费带宽）。

### 3) APKINDEX 签名处理
- **Decision**: APKINDEX 及其签名文件原样透传并缓存，索引 RequireRevalidate=true，包体直接缓存。
- **Rationale**: Alpine 客户端依靠签名文件校验，代理不应修改或剥离；再验证确保索引更新。
- **Alternatives**: 不缓存 APKINDEX（失去加速效果）；仅缓存包体（无法验证包版本更新）。
