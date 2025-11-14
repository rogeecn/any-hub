# Quickstart: 配置与骨架

## 前置条件
- 已安装 Go 1.25+，并能运行 `go test ./...`。
- 提供一份最小 `config.toml`，包含全局设置与至少一个 `[[Hub]]`。
- 当前分支：`001-config-bootstrap`。

## 步骤
1. **编写配置文件**
   - 复制 `config.example.toml`（若无则创建）到项目根目录 `config.toml`。
   - 填写必填字段：`StoragePath`、`[[Hub]]` 的 `Name/Domain/Port/Upstream`。
2. **运行配置校验**
   - 执行 `go run ./cmd/any-hub --check-config --config ./config.toml`。
   - 观察输出：成功时显示“配置验证通过”；失败时按日志提示修复字段。
3. **启动 CLI**
   - `go run ./cmd/any-hub --config ./config.toml`。
   - 启动日志会打印版本、配置路径、监听端口。
4. **查看版本**
   - `go run ./cmd/any-hub --version`。
   - 确认输出如 `any-hub version 0.1.0 (commit abc1234)`。
5. **检查日志**
   - 若配置写入文件：查看 `LogFilePath` 位置的滚动日志。
   - 默认 stdout：在终端确认日志字段（level, action, hub, domain, result）。

## 验证清单
- [ ] `--check-config` 能阻断非法配置并返回非零退出码。
- [ ] CLI 成功读取默认或自定义配置路径。
- [ ] `--version` 立即输出版本信息。
- [ ] 日志初始化遵循配置的级别与输出。

## 常见错误排查
- 出现 `Hub[docker].Domain: Domain 不允许包含协议头`：检查 Hub 配置中是否误写 `http://` 前缀。
- 出现 `Global.StoragePath: 不能为空`：确保 `config.toml` 中填写路径或使用默认值。
- 若日志提示 `logger_fallback`，说明文件不可写，删除 `LogFilePath` 或放宽目录权限。

## 日志字段说明
- `action`: 表示当前操作（`check_config`、`startup` 等）。
- `configPath`: 实际使用的配置文件路径，方便验证 flag/环境变量生效。
- `result`: 操作结果（`ok`、`error`）。
- `hub` / `domain` / `cacheHit`: 由 `internal/logging.RequestFields` 提供，后续代理请求会自动填充。
