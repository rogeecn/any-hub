// Package template 提供编写新模块时可复制的骨架示例。
package template

import "github.com/any-hub/any-hub/internal/hubmodule"
//
// 使用方式：复制整个目录到 internal/hubmodule/<module-key>/ 并替换字段。
// - 将 TemplateModule 重命名为实际模块类型。
// - 在 init() 中调用 hubmodule.MustRegister，注册新的 ModuleMetadata。
// - 在模块目录中实现自定义代理/缓存逻辑，然后在 main 中调用 proxy.RegisterModuleHandler。
//
// 注意：本文件仅示例 metadata 注册写法，不会参与编译。
var _ = hubmodule.ModuleMetadata{}
