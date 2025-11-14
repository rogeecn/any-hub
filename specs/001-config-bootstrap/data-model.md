# Data Model: 配置与骨架

## Entities

### GlobalConfig
- **Description**: 控制 any-hub 全局行为的配置段，位于 `config.toml` 根部。
- **Fields**:
  - `LogLevel` (string, required, enum: trace/debug/info/warn/error)
  - `LogFilePath` (string, optional, default "")
  - `LogMaxSize` (int, optional, default 100, MB)
  - `LogMaxBackups` (int, optional, default 10)
  - `LogCompress` (bool, optional, default true)
  - `StoragePath` (string, required, must be writable directory)
  - `CacheTTL` (duration seconds, optional, default 86400)
  - `MaxMemoryCacheSize` (bytes, optional, default 268435456)
  - `MaxRetries` (int >=0, default 3)
  - `InitialBackoff` (duration, default 1s)
  - `UpstreamTimeout` (duration, default 30s)
- **Validation Rules**: 路径必须存在或可创建；数值必须 >0；LogLevel 必须匹配允许枚举。
- **Relationships**: 被 `Config` 聚合并为 `HubConfig` 提供默认值。

### HubConfig
- **Description**: 描述单个代理仓库实例。
- **Fields**:
  - `Name` (string, required, unique)
  - `Domain` (string, required, FQDN)
  - `Port` (int, required, 1-65535)
  - `Upstream` (string, required, http/https URL)
  - `Proxy` (string, optional, URL)
  - `CacheTTL` (duration, optional, overrides global)
  - `EnableHeadCheck` (bool, optional, default true)
- **Validation Rules**: `Name` 必须唯一；`Domain` + `Port` 组合不得冲突；URL 必须可解析。
- **Relationships**: 属于 `Config`，在运行时用于初始化路由、缓存目录 `StoragePath/<Name>`。

### Config (Root)
- **Description**: 聚合 `GlobalConfig` 与一个或多个 `HubConfig` 条目。
- **Fields**:
  - `Global` (`GlobalConfig`, required)
  - `Hubs` (`[]HubConfig`, min length 1)
- **Validation Rules**: 至少一个 Hub；Hub 列表中所有必填字段存在；引用 `StoragePath` 时需组合 `Hub.Name` 生成可写路径。
- **State Transitions**:
  1. **Loaded**: 从 TOML 解析到结构体。
  2. **Validated**: 完成默认值填充 + 规则校验。
  3. **Active**: 提供给 CLI/服务器初始化。

### CLIFlagSet
- **Description**: 运行入口解析到的参数集合。
- **Fields**:
  - `ConfigPath` (string, default `./config.toml`)
  - `CheckOnly` (bool, set by `--check-config`)
  - `ShowVersion` (bool, set by `--version`)
- **Validation Rules**: 当 `ConfigPath` 指向不存在文件时须立即报错；`CheckOnly` 与正常启动互斥（若为 true 则不进入 server）。
- **Relationships**: 在主函数中决定执行路径（验证 / 启动 / 版本输出）。

### LogSinkConfig
- **Description**: 运行期日志输出策略，来源于 `GlobalConfig`。
- **Fields**:
  - `Level` (string, required)
  - `Output` (enum: stdout|file)
  - `FilePath`, `MaxSize`, `MaxBackups`, `Compress`（当 Output=file 时必填）
- **Validation Rules**: 当选择 file 时需验证写权限；level 必须映射到 Logrus 支持的等级。
- **Relationships**: `initLogger(cfg)` 根据该实体创建 logger，并将字段注入所有日志条目。
