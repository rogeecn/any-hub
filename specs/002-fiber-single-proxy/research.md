# Research: HTTP 服务与单仓代理

## Host → Hub 路由策略
- **Decision**: 使用 Fiber 自定义中间件读取 `Host` Header 与监听端口，查询预构建的 `HubRegistry`，命中才放行，未命中返回 404 并记录 `host_unmapped`。
- **Rationale**: 保持单一入口、避免误路由带来安全风险，同时兼容将来多 Hub 扩展。
- **Alternatives considered**: 依赖 Fiber 原生多 App（导致重复配置/资源浪费）；将未知 Host 回退到默认 Hub（难以追踪错误）。

## 缓存与条件回源
- **Decision**: 采用磁盘文件 + `.meta` 元数据，写入时使用临时文件 + rename；记录 `ETag/Last-Modified/StoredAt`，并在重验证时携带 `If-None-Match`/`If-Modified-Since`。
- **Rationale**: 磁盘缓存简单可靠，临时文件避免半写入；条件请求可显著降低带宽，满足 SC-001/SC-004。
- **Alternatives considered**: 内存缓存（受限于 256MB）；数据库存储（超出 Phase 1 范畴）。

## 上游 HTTP 客户端
- **Decision**: 共享 `http.Client`，启用 `Transport` 连接复用、超时/Proxy/Retry 设置来自 config；在 `internal/proxy/upstream.go` 中封装请求构造。
- **Rationale**: 避免为每个请求重建 client，提高吞吐并可统一超时；复用 config 中 Proxy 设定。
- **Alternatives considered**: 每 Hub 创建独立 client（增加内存占用）、第三方 HTTP 库（违背依赖约束）。

## 示例仓库选择
- **Decision**: 默认提供 Docker Hub 示例（manifest + layer）和 NPM 示例（package metadata）；在 quickstart 中支持切换到本地 fake upstream（tests/integration/stub）。
- **Rationale**: Docker/NPM 覆盖二进制/JSON 两类典型仓库；fake upstream 便于离线/CI；满足 spec 对示例的要求。
- **Alternatives considered**: 只做 Docker（覆盖面不足）、引入真实 registry 模拟器（复杂度高）。

## 观测性与性能指标
- **Decision**: 在 proxy handler 中记录 `hub`, `domain`, `cache_hit`, `upstream_status`, `elapsed_ms`; 通过 `logrus.WithFields` 输出 JSON；同时统计请求计数（后续可暴露 metrics）。
- **Rationale**: 满足 FR-006 / SC-001/002；方便后续扩展 metrics；JSON 便于日志采集。
- **Alternatives considered**: 文本日志（难以机器解析）；延迟到未来阶段再实现（不符成功标准）。
