# CLI Contract: 配置与骨架

## Command Overview
```
any-hub [--config <path>] [--check-config] [--version]
```

## Flags
| Flag | Type | Default | Description | Behavior |
|------|------|---------|-------------|----------|
| `--config, -c` | string | `./config.toml` | 指定配置文件路径；优先级高于 `ANY_HUB_CONFIG` | 解析后记录在日志字段 `configPath`，路径不存在则退出并提示 |
| `--check-config` | bool | false | 启用只校验模式，不启动 HTTP 服务 | 运行完整加载+校验链路；成功退出码 0，失败非 0 |
| `--version` | bool | false | 输出版本信息并退出 | 打印语义化版本（含 commit/hash），忽略其他标志 |

## Exit Codes
| Code | Meaning |
|------|---------|
| 0 | 操作成功（验证通过或正常退出） |
| 1 | 配置解析/校验失败 |
| 2 | CLI 参数错误（未知标志、冲突） |

## Logging Guarantees
- 每条日志包含：`timestamp`, `level`, `action` (`check_config`, `startup`, `version`), `configPath`, `result`, `hub`(若适用), `domain`(若适用).
- 当写文件失败时会降级到 stdout，并再记录一条 `action=logger_fallback` 的警告。

## Sample Interactions
1. `any-hub --check-config --config /etc/any-hub.toml`
   - 校验文件，通过则输出 `configuration_valid`，否则列出字段错误。
2. `any-hub --version`
   - 输出 `any-hub version 0.1.0 (commit abc1234)` 并退出。
3. `ANY_HUB_CONFIG=/etc/any-hub.toml any-hub`
   - 若未显式传 `--config`，程序读取环境变量路径并记录来源。
