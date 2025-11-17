# Quickstart: Proxy Module Delegation

1) 检出分支  
```bash
git checkout 005-proxy-module-delegation
```

2) 跑现有测试基线  
```bash
go test ./...
```

3) 添加新模块示例（开发）  
- 在 `internal/hubmodule/<module>` 下实现元数据，并在 `init()` 中调用 `hubmodule.MustRegister`。  
- 为该模块实现专属 handler（可基于 `internal/proxy/handler.go` 拓展），在启动时通过 `proxy.RegisterModule(proxy.ModuleRegistration{Key: "<module_key>", Handler: yourHandler})` 绑定。  
- 在 `config.toml` 中把目标 Hub 的 `Module` 字段设为这个 `module_key`。

4) 启动代理验证  
```bash
make run
# or
go run . --config ./config.toml
```

5) 验证缓存与日志  
- 对同一路径请求两次，观察首次 cache_hit=false、二次 cache_hit=true，日志包含 module_key/hub/domain/upstream_status。  
- 验证缺失 handler 时返回 5xx 且日志含错误字段。

6) 更新文档与计划  
- 填充 tasks 清单（/speckit.tasks）。  
- 提交前复查 `specs/005-proxy-module-delegation` 下的设计/研究文件。
