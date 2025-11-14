# Research: 配置与骨架

## 配置加载与校验
- **Decision**: 使用 Viper 读取单一 `config.toml`，解析到 `GlobalConfig` + `[]HubConfig` 结构，先合并默认值再执行字段级校验（必填、范围、路径可写），校验失败即返回结构化错误并阻断启动。
- **Rationale**: 单一入口与结构化错误能满足宪法“单一控制平面 + 阻断非法配置”的要求，也方便 `--check-config` 模式直接复用相同逻辑。
- **Alternatives considered**: 直接手写 `encoding/toml` 解析（缺乏灵活默认/环境覆盖），多文件拆分（违反宪法单一配置要求）。

## CLI 标志与执行模式
- **Decision**: 使用 `pflag`/`flag` 解析 `--config`, `--check-config`, `--version`，解析顺序：标志 > `ANY_HUB_CONFIG` 环境变量 > 默认路径；`--check-config` 触发只读模式，`--version` 输出后立即退出。
- **Rationale**: 可预测的优先级防止多来源冲突；独立模式让 CI 可执行校验并快速确认版本，贴合用户故事 1/2。
- **Alternatives considered**: 将 `--check-config` 拆成单独子命令（增加学习成本）、允许多配置文件（违背宪法）。

## 日志初始化策略
- **Decision**: 统一在 CLI 启动与校验路径调用 `initLogger(cfg)`，基于 Logrus + Lumberjack：默认 stdout，若配置文件路径可写则启用滚动文件，同时注入字段（hub、domain、action、configPath、result）。
- **Rationale**: 结构化日志 + 滚动文件满足宪法可观测性；单初始化路径减少重复配置并确保 `--check-config` 也有可追踪日志。
- **Alternatives considered**: log/slog（失去与现有宪法保持一致的依赖）；多 logger 实例（复杂度高且容易导致字段缺失）。

## 测试与验证覆盖
- **Decision**: 编写 `internal/config` 单元测试覆盖：必填字段缺失、类型错误、默认值填充、`[[Hub]]` 唯一约束；CLI 集成测试使用 `httptest`/临时目录模拟不同标志组合；日志初始化通过接口注入 writer 以断言字段。
- **Rationale**: 完成宪法要求的配置/缓存/路由主路径测试，确保 Phase 0 可在 `go test ./...` 下复现；临时目录避免污染本地环境。
- **Alternatives considered**: 仅手动测试（无法满足 Gate）、依赖真实文件系统路径（降低可靠性）。

## 依赖与最佳实践
- **Decision**: 保持依赖集为 Go 标准库 + Viper + Logrus + Lumberjack + pflag（随 Viper 引入），不新增网络/数据库库；使用 `fsnotify` 仅作为 Viper 间接依赖，不启用热加载。
- **Rationale**: 满足宪法“受控依赖”原则并保持二进制小巧；禁用热加载可降低错配风险并符合单一配置理念。
- **Alternatives considered**: 引入 Cobra CLI（依赖过重且 Phase 0 命令简单）、使用 zap 日志（与既定依赖不符）。
