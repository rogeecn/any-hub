package hubmodule

import (
	"path"
	"strings"
)

// DefaultLocatorRewrite 根据 Hub 类型返回通用的路径重写逻辑。
func DefaultLocatorRewrite(hubType string) LocatorRewrite {
	switch hubType {
	case "npm":
		return rewriteNPMLocator
	case "go":
		return rewriteGoLocator
	default:
		return nil
	}
}

func rewriteNPMLocator(loc Locator) Locator {
	pathVal := loc.Path
	if pathVal == "" {
		return loc
	}

	var qsSuffix string
	core := pathVal
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

func rewriteGoLocator(loc Locator) Locator {
	if loc.Path == "" {
		loc.Path = "/"
		return loc
	}
	clean := path.Clean("/" + loc.Path)
	if strings.HasPrefix(clean, "/sumdb/") {
		loc.Path = clean
		return loc
	}
	if strings.HasSuffix(clean, "/") {
		clean = strings.TrimSuffix(clean, "/")
		if clean == "" {
			clean = "/"
		}
	}
	loc.Path = clean
	return loc
}
