# any-hub

any-hub 是一个面向个人与小型团队的多仓库代理，主打匿名访问、命令行部署与单一 `config.toml` 控制平面。当前阶段已交付 Phase 1：构建 Host→Hub 路由、磁盘缓存与示例代理。

## 单仓代理 (Phase 1)

Phase 1 的 HTTP 服务与磁盘缓存能力详见 [`specs/002-fiber-single-proxy/spec.md`](specs/002-fiber-single-proxy/spec.md) 与 [`plan.md`](specs/002-fiber-single-proxy/plan.md)。目标是：

- 构建 Fiber HTTP 服务 + Host 驱动（共享 `ListenPort`）的 Hub Registry，使 `docker.hub.local`、`npm.hub.local` 等域名在同一端口内路由到独立上游。
- 实现 `StoragePath/<Hub>/<path>` 目录下的磁盘缓存，依靠文件 `mtime` + 上游 `HEAD` 请求完成动态标签的再验证。
- 提供 Docker/NPM/PyPI 示例配置、quickstart、测试桩，运行 `go test ./tests/integration` 即可验证代理/缓存流程。

随着 Phase 1 推进，`cmd/any-hub` 将接入 server/cache/quickstart 章节，便于复用 Phase 0 的配置与日志骨架。

## ListenPort 与凭证迁移指南

1. **全局端口**：在配置全局段声明 `ListenPort = <port>`，所有 Hub 共享该端口；旧的 `[[Hub]].Port` 字段已弃用，`any-hub --check-config` 会在检测到遗留字段时直接失败。
2. **Hub 类型**：为每个 `[[Hub]]` 添加 `Type = "docker|npm|go|pypi"`，驱动日志中的 `hub_type` 字段并预留协议特定行为；非法值会被校验阻断。
3. **可选凭证**：如需突破上游限流，成对提供 `Username`/`Password`。CLI 仅在这两个字段同时出现时注入 Basic Auth，并在日志中输出掩码形式的 `auth_mode=credentialed`。
4. **验证命令**：使用 `any-hub --check-config --config ./config.toml` 快速确认迁移是否完成，成功时日志会显示 `listen_port`、`hub_type` 等字段。

## 凭证配置示例

```toml
[[Hub]]
Name = "secure"
Domain = "secure.hub.local"
Upstream = "https://registry.corp.local"
Type = "npm"
Username = "ci-user"
Password = "s3cr3t"
```

- CLI 日志不会打印明文凭证，而是输出 `credentials=["secure:credentialed"]`，可在 `any-hub --check-config --config secure.toml` 中验证。
- 建议结合环境变量或密钥管理器生成 `config.toml`，并通过 `chmod 600` 或 CI Secret 注入限制可见范围。

## 快速开始

1. 复制 `configs/config.example.toml` 为工作目录下的 `config.toml` 并调整 `[[Hub]]` 配置：
   - 在全局段添加/修改 `ListenPort`，并从每个 Hub 中移除 `Port`。
   - 为 Hub 填写 `Type`，并按需添加 `Username`/`Password`。
   - 根据 quickstart 示例设置 `Domain`、`Upstream`、`StoragePath` 等字段。
2. 参考 [`specs/003-hub-auth-fields/quickstart.md`](specs/003-hub-auth-fields/quickstart.md) 完成配置校验、凭证验证与日志检查。
3. 常用命令：
   - `any-hub --check-config --config ./config.toml`
   - `any-hub --config ./config.toml`
   - `any-hub --version`

## 示例代理

- `configs/docker.sample.toml`、`configs/npm.sample.toml` 展示了 Docker/NPM 的最小配置，复制后即可按需调整 Domain、Type、StoragePath 与凭证。
- 运行 `./scripts/demo-proxy.sh docker`（或 `npm`）即可加载示例配置并启动代理，便于快速验证 Host 路由与缓存命中。
- 示例操作手册、常见问题参见 [`specs/003-hub-auth-fields/quickstart.md`](specs/003-hub-auth-fields/quickstart.md)。

## CLI 标志

| Flag             | 描述 |
|------------------|------|
| `--config, -c`   | 指定配置文件路径，优先级高于 `ANY_HUB_CONFIG` |
| `--check-config` | 仅执行配置校验并退出，退出码区分成功/失败 |
| `--version`      | 打印语义化版本信息并立即退出 |

更多细节可查阅 [`contracts/cli-flags.md`](specs/001-config-bootstrap/contracts/cli-flags.md)。

> 优先级：`--config` ⬆ `ANY_HUB_CONFIG` ⬆ 默认 `./config.toml`。`--version` 会短路其他逻辑，`--check-config` 则在日志中记录 `action=check_config` 并返回退出码。

## 配置校验错误示例

```
$ any-hub --check-config --config broken.toml
Config.ListenPort: 必须在 1-65535
Hub[npm].Type: 不支持的值 "rubygems"
```

错误消息以 `字段路径: 原因` 形式展示，可根据 `quickstart.md` 的“常见错误排查”章节快速定位。
