# Quickstart: HTTP 服务与单仓代理

## 前置条件
- Go 1.25+，可运行 `go run` 与 `go test`。
- 访问 Docker Hub 或准备本地 fake upstream（见 tests/integration）。
- 端口 5000 可用。

## 步骤
1. **准备配置**
   ```bash
   cp configs/docker.sample.toml config.toml
   ```
   修改 `[[Hub]]` 中的 `Domain`(例如 `docker.hub.local`)，并确保 `/etc/hosts` 映射到 `127.0.0.1`。
2. **启动代理**
   ```bash
   go run ./cmd/any-hub --config ./config.toml
   ```
   终端将打印 `action=startup` 日志。
3. **发起请求并观察路由**
   ```bash
   curl -H "Host: docker.hub.local" http://127.0.0.1:5000/v2/
   ```
   响应中会包含 `X-Any-Hub-Upstream`、`X-Any-Hub-Cache-Hit: false` 与 `X-Request-ID`，日志记录 `action=proxy`、`hub=docker`、`upstream_status=200`。
4. **验证 Host 未配置时的 404**
   ```bash
   curl -i -H "Host: unknown.hub.local" http://127.0.0.1:5000/v2/
   ```
   代理会返回 `404 {"error":"host_unmapped"}`，日志显示 `action=host_lookup`。
5. **切换 NPM 示例**
   ```bash
   cp configs/npm.sample.toml config.toml
   npm config set registry http://127.0.0.1:5000 --global
   npm view lodash --registry http://127.0.0.1:5000 --fetch-timeout=60000
   ```
6. **运行路由集成测试**
   ```bash
   go test ./tests/integration -run HostRouting -v
   ```
7. **使用示例配置快速体验**
   ```bash
   # Docker Hub 示例
   ./scripts/demo-proxy.sh docker

   # NPM 示例 (监听 5001)
   ./scripts/demo-proxy.sh npm
   ```
   脚本会直接引用 `configs/docker.sample.toml` / `configs/npm.sample.toml` 并启动代理，确保对应域名已指向本机。

## 故障排查
- `host_unmapped`: 检查 Host 头与 `config.toml` 中 Domain 是否一致。
- `cache_write_failed`: 确认 `StoragePath` 可写，并有足够磁盘空间。
- 上游 401/403: 公共仓库可能需要额外匿名访问 header；可在配置中新增自定义 header（future work）。
