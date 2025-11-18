# Quickstart: Module Hook Refactor

1) 检出分支并安装依赖
```bash
git checkout 006-module-hook-refactor
/home/rogee/.local/go/bin/go test ./...
```

2) 定义模块 Hook
```go
func init() {
    proxy.RegisterModule(proxy.ModuleRegistration{
        Key: "npm",
        Handler: proxyHandler,
    })
    hooks.MustRegister("npm", hooks.Hooks{
        NormalizePath:   myNormalize,
        ResolveUpstream: myResolve,
        RewriteResponse: myRewrite,
        CachePolicy:     myPolicy,
        ContentType:     myContentType,
    })
}
```

3) 迁移逻辑
- 读取 `internal/proxy/handler.go` 里的类型分支，对应迁移到模块 Hook。
- 更新模块单元测试验证缓存、路径、响应重写等行为。

4) 验证
```bash
/home/rogee/.local/go/bin/go test ./...
```
- 针对迁移模块执行“第一次 miss → 第二次 hit”端到端测试。
- 触发缺失 handler/panic，确保返回 `module_handler_missing`/`module_handler_panic`。

5) 诊断检查
```bash
curl -s http://localhost:8080/-/modules | jq '.modules[].hook_status'
```
- 确认新模块标记为 `registered`，未注册模块显示 `missing`，legacy handler 仍可作为兜底。
- 如果需要查看全局状态，可检查 `hook_registry` 字段，它返回每个 module_key 的注册情况。
- `hubs[].module_key` 应与配置中的 `Type` 对齐；legacy 模块仅作为兜底存在，推荐尽快替换为协议专用模块。
- 启动阶段会验证每个模块是否注册 Hook，缺失则直接退出，避免运行期静默回退。
