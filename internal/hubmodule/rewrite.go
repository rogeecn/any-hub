package hubmodule

import "strings"

// DefaultLocatorRewrite 针对内置 hub 类型提供缓存路径重写逻辑。
func DefaultLocatorRewrite(hubType string) LocatorRewrite {
	 switch hubType {
	 case "npm":
		return rewriteNPMLocator
	 default:
		return nil
	 }
}

func rewriteNPMLocator(loc Locator) Locator {
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
