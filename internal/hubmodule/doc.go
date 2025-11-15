// Package hubmodule 聚合任意仓类型的代理 + 缓存模块，并提供统一的注册入口。
//
// 模块作者需要：
//   1. 在 internal/hubmodule/<module-key>/ 目录下实现代理与缓存接口；
//   2. 通过本包暴露的 Register 函数在 init() 中注册模块元数据；
//   3. 保证缓存写入仍遵循 StoragePath/<Hub>/<path> 原始路径布局，并补充中文注释说明实现细节。
//
// 该包同时负责提供模块发现、可观测信息以及迁移状态的对外查询能力。
package hubmodule
