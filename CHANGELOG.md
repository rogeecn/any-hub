# Changelog

## 2025-11-16
- 完成 Phase 5（US3）收尾：配置校验加入 Type 表驱动测试，proxy/router 对未知 `Type` 直接返回 `hub_type_unsupported`，日志包含 `hub_type`/`auth_mode`。
- 运行 `gofmt -w ./cmd ./internal ./tests` 与 `GOCACHE=/tmp/go-build go test ./...`，并将命令写入 DEVELOPMENT.md，形成日常校验基线。
- 更新 quickstart 与示例配置，演练 `curl` + `npm` 匿名请求流程，同时记录如何通过日志 `credentials`/`auth_mode` 字段手动验收 docker 与 npm 代理。

## 2025-11-15
- 将监听端口上移为全局 `ListenPort`，`[[Hub]].Port` 被视为非法配置，`--check-config` 会提醒迁移。
- 为 Hub 引入 `Username`/`Password` 凭证与必填 `Type` 枚举，代理会在带凭证时掩码日志并注入 Basic Auth。
- 更新 `README.md`、`DEVELOPMENT.md` 与 `specs/003-hub-auth-fields/quickstart.md`，记录新的字段说明、迁移步骤及验证命令。

## 2025-11-13
- 引入 Phase 0 "配置与骨架"：提供配置加载/校验、CLI flag 优先级与结构化日志。
- 新增示例 `configs/config.example.toml` 与 Quickstart，方便复现 `--check-config`/`--version` 流程。
- 增加日志 fallback 与字段 helper，确保所有 CLI 输出可追踪。

## 2025-11-14
- 完成 Phase 1 HTTP 服务：实现 Host→Hub Fiber 路由、磁盘缓存与条件回源，CLI 现可监听多个端口。
- 新增 Docker/NPM 示例配置、`scripts/demo-proxy.sh` 以及缓存/样例集成测试，quickstart/README 覆盖完整操作流程。
