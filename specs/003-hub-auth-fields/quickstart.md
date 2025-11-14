# Quickstart: Hub 凭证与类型配置

## 前置检查

| 字段 | 说明 | 验证方式 |
|------|------|----------|
| `ListenPort` | 全局唯一监听端口，所有 Hub 共用 | `any-hub --check-config` 会输出 `listen_port=<port>` |
| `Username`/`Password` | 可选凭证字段，两个字段必须同时出现 | 成功日志含 `auth_mode=credentialed`；缺失时为 `anonymous` |
| `Type` | 仓库类型，当前支持 `docker`/`npm`/`go` | 错误输入将导致 `hub_type_invalid` 报错 |

**基本命令**

```bash
any-hub --check-config --config hub-auth.toml
any-hub --config hub-auth.toml
```

1. **准备配置**
   ```bash
   cp configs/config.example.toml hub-auth.toml
   ```
   - 在全局段添加 `ListenPort = 5000` 并移除 `[[Hub]]` 中的 `Port` 字段。
   - 为需要解锁 rate-limit 的 Hub 写入 `Username`/`Password`，保持小写字符串。
   - 设置 `Type` 为 `docker`、`npm` 或 `go`。其它值会被 `any-hub --check-config` 拒绝，并在日志中提示 `hub_type_unsupported`。若需扩展新的仓库类型，请遵循 plan.md 中的“类型扩展策略”提交补丁。

2. **运行校验**
   ```bash
   any-hub --check-config --config hub-auth.toml
   ```
   预期日志：`{"action":"check_config","credentials":["docker:credentialed"],"listen_port":5000,"result":"ok"}`（无凭证的 Hub 会显示 `hub:anonymous`）。

3. **启动代理**
   ```bash
   any-hub --config hub-auth.toml
   ```
   CLI 只监听 `ListenPort` 指定的端口，所有 Hub 通过 Host 头路由。

4. **验证凭证透传**
   ```bash
   # Docker CLI 方式
   curl -H "Host: docker.hub.local" http://127.0.0.1:5000/v2/

   # NPM 方式（匿名客户端，不需要 .npmrc 凭证）
   npm --registry http://127.0.0.1:5000 --always-auth=false view lodash
   ```
   - 代理会在回源请求中自动注入 `Username`/`Password`，而下游 `curl`/`npm` 无须提供任何 Authorization header。

5. **观察日志**
   - 每条请求日志包含 `hub`, `hub_type`, `auth_mode`, `upstream_status`。
   - 若凭证无效，会出现 `error=auth_failed`，需更新配置并重启。
   - `config.toml` 保存明文凭证，部署时请配合 `chmod 600` 或密钥注入工具限制读取范围。
   - 建议执行 `tail -n 20 logs/any-hub.log | jq '.auth_mode,.hub_type,.cache_hit'`，确认命中缓存与凭证状态。
