# Research: Hub 配置凭证字段

## Credential Handling in config.toml
- **Decision**: 允许在 `[[Hub]]` 中以明文写入 `Username`/`Password`，CLI 读取后立即掩码写日志，并鼓励与外部 Secret 管理（环境变量/Ansible Vault）组合。
- **Rationale**: 代理以 CLI 形式部署，最简单可靠的方式是继续沿用 TOML；通过 `config --check-config` 校验与日志掩码即可满足大多数场景。
- **Alternatives considered**:
  - 单独的凭证文件：需要额外路径与权限管理，易造成部署复杂化。
  - 环境变量注入：缺乏结构化校验，且与当前 TOML 驱动原则冲突。

## Hub Type 枚举与扩展
- **Decision**: 首批仅支持 `docker`、`npm`、`go`，在 `internal/config` 中实现枚举校验，并在 `internal/proxy`/`internal/server` 中通过 `switch type` 执行类型特定逻辑；保留 `default` 分支抛出“未支持类型”错误，为 apt/yum/composer 等新增项预留 hook。
- **Rationale**: 明确 types 可让日志/策略按仓库协议定制，同时使用枚举校验可避免错误输入；`switch` + 独立 helper 便于未来拆分成策略表。
- **Alternatives considered**:
  - 自定义字符串+运行期反射：增加复杂度且容易出错。
  - 大型插件机制：当前范围只需声明式扩展，无需插件框架。

## 单端口 Host 路由迁移
- **Decision**: 将监听端口移至全局（如 `ListenPort`），Fiber 仅在该端口启动一次；Hub 级 `Port` 字段在配置加载阶段报错提示迁移。路由层严格依赖 `Host`/`Host:port` 解析，必要时通过 `SERVER_PORT` 环境变量或 CLI flag override。
- **Rationale**: 单端口可简化暴露面并符合“Host 区分仓库”的目标；在 `internal/server` 中已有 Host registry，可直接复用。
- **Alternatives considered**:
  - 同时保留 per-Hub 端口：违背“统一端口”诉求且增加监听资源。
  - 动态多端口监听：需要额外 goroutine/生命周期管理，复杂度高。
