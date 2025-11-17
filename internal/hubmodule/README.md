# hubmodule

集中定义和实现 Any-Hub 的“代理 + 缓存”模块体系。

## 目录结构

```
internal/hubmodule/
├── doc.go            # 包级说明与约束
├── README.md         # 本文件
├── registry.go       # 模块注册/发现入口（后续任务）
└── <module-key>/     # 各仓类型模块，例如 legacy、npm、docker
```

## 模块约束
- **单一接口**：每个模块需要同时实现代理与缓存接口，避免跨包耦合。
- **注册流程**：在模块 `init()` 中调用 `hubmodule.Register(ModuleMetadata{...})`，注册失败必须 panic 以阻止启动。
- **缓存布局**：一律使用 `StoragePath/<Hub>/<path>`，即与上游请求完全一致的磁盘路径；当某个路径既要保存正文又要作为子目录父节点时，会在该目录下写入 `__content` 文件以存放正文。
- **配置注入**：模块仅通过依赖注入获取 `HubConfigEntry` 和全局参数，禁止直接读取文件或环境变量。
- **可观测性**：所有模块必须输出 `module_key`、命中/回源状态等日志字段，并在返回错误时附带 Hub 名称。

## 开发流程
1. 复制 `internal/hubmodule/template/`（由 T010 提供）作为起点。
2. 填写模块特有逻辑与缓存策略，并确保包含中文注释解释设计。
3. 在模块目录添加 `module_test.go`，使用 `httptest.Server` 与 `t.TempDir()` 复现真实流量。
4. 运行 `make modules-test` 验证模块单元测试。
5. `[[Hub]].Module` 留空时会优先选择与 `Type` 同名的模块，实际迁移时仍建议显式填写，便于 diagnostics 标记 rollout。

## 术语
- **Module Key**：模块唯一标识（如 `legacy`、`npm-tarball`）。
- **Cache Strategy Profile**：定义 TTL、验证策略、磁盘布局等策略元数据。
- **Legacy Adapter**：包装当前共享实现，确保迁移期间仍可运行。
