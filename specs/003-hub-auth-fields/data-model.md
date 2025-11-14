# Data Model: Hub 配置凭证字段

## HubConfig
- **Fields**:
  - `Name` (string, required, unique)
  - `Domain` (string, required, Host header used for路由)
  - `Upstream` (URL, required)
  - `Proxy` (URL, optional)
  - `Username` (string, optional, may be empty)
  - `Password` (string, optional, may be empty)
  - `Type` (enum: docker | npm | go, required)
  - `CacheTTL` (duration override, optional)
- **Relationships**: 属于全局 `Config`，在运行期被 `HubRegistry` 索引。
- **Validation**: `Username` 与 `Password` 要么同时缺省要么同时提供；`Type` 仅允许受支持的值。

## GlobalConfig
- **Fields**:
  - `ListenPort` (int, required, 1-65535)
  - 现有全局字段（LogLevel、StoragePath、CacheTTL、MaxRetries 等）保持不变
- **Relationships**: CLI 仅在 `ListenPort` 启动一次 Fiber 服务；HubRegistry 使用该端口验证 Host:port。

## AuthProfile (runtime)
- **Fields**:
  - `HubName`
  - `AuthMode` (anonymous | credentialed)
  - `LastAttempt` timestamp
  - `Status` (success | failed)
- **Purpose**: 供日志/metrics 输出，帮助追踪凭证是否生效。
- **State transitions**: anonymous→credentialed 当检测到配置凭证；credentialed→anonymous 当凭证校验失败且被禁用（未来扩展）。

## HubType (enum)
- **Values**: docker, npm, go（后续 apt/yum/composer 等）
- **Usage**: 决定请求头/日志字段/未来仓库特定策略；存储于 HubConfig。
