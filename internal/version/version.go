package version

import "fmt"

// Version/Commit 可在构建时通过 -ldflags 注入，默认使用开发占位符。
var (
	Version = "0.1.0"
	Commit  = "dev"
)

// Full 返回便于 CLI 打印的完整版本信息。
func Full() string {
	return fmt.Sprintf("any-hub %s (%s)", Version, Commit)
}
