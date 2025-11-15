package npm

import (
	"strings"

	"github.com/any-hub/any-hub/internal/cache"
)

// rewriteLocator 将 npm metadata JSON 落盘至 package.json，避免与 tarball
// 路径的 `/-/` 子目录冲突，同时保持 tarball 使用原始路径。
func rewriteLocator(loc cache.Locator) cache.Locator {
	path := loc.Path
	if path == "" {
		return loc
	}

	var qsSuffix string
	core := path
	if idx := strings.Index(core, "/__qs/"); idx >= 0 {
		qsSuffix = core[idx:]
		core = core[:idx]
	}

	if strings.Contains(core, "/-/") {
		loc.Path = core + qsSuffix
		return loc
	}

	clean := strings.TrimSuffix(core, "/")
	if clean == "" {
		clean = "/"
	}

	if clean == "/" {
		loc.Path = "/package.json" + qsSuffix
		return loc
	}

	loc.Path = clean + "/package.json" + qsSuffix
	return loc
}
