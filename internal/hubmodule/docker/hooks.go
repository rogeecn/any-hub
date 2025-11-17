package docker

import (
	"strings"

	"github.com/any-hub/any-hub/internal/proxy/hooks"
)

func init() {
	hooks.MustRegister("docker", hooks.Hooks{
		NormalizePath: normalizePath,
		CachePolicy:   cachePolicy,
		ContentType:   contentType,
	})
}

func normalizePath(ctx *hooks.RequestContext, clean string, rawQuery []byte) (string, []byte) {
	if !isDockerHubHost(ctx.UpstreamHost) {
		return clean, rawQuery
	}
	repo, rest, ok := splitDockerRepoPath(clean)
	if !ok || repo == "" || strings.Contains(repo, "/") || repo == "library" {
		return clean, rawQuery
	}
	return "/v2/library/" + repo + rest, rawQuery
}

func cachePolicy(_ *hooks.RequestContext, locatorPath string, current hooks.CachePolicy) hooks.CachePolicy {
	clean := locatorPath
	if clean == "/v2" || clean == "v2" || clean == "/v2/" {
		return hooks.CachePolicy{}
	}
	if strings.Contains(clean, "/_catalog") {
		return hooks.CachePolicy{}
	}
	if isDockerImmutablePath(clean) {
		current.AllowCache = true
		current.AllowStore = true
		current.RequireRevalidate = false
		return current
	}
	current.AllowCache = true
	current.AllowStore = true
	current.RequireRevalidate = true
	return current
}

func contentType(_ *hooks.RequestContext, locatorPath string) string {
	switch {
	case strings.Contains(locatorPath, "/tags/list"):
		return "application/json"
	case strings.Contains(locatorPath, "/blobs/"):
		return "application/octet-stream"
	default:
		return ""
	}
}

func isDockerHubHost(host string) bool {
	switch strings.ToLower(host) {
	case "registry-1.docker.io", "docker.io", "index.docker.io":
		return true
	default:
		return false
	}
}

func splitDockerRepoPath(path string) (string, string, bool) {
	if !strings.HasPrefix(path, "/v2/") {
		return "", "", false
	}
	suffix := strings.TrimPrefix(path, "/v2/")
	if suffix == "" || suffix == "/" {
		return "", "", false
	}
	segments := strings.Split(suffix, "/")
	var repoSegments []string
	for i, seg := range segments {
		if seg == "" {
			return "", "", false
		}
		switch seg {
		case "manifests", "blobs", "tags", "referrers":
			if len(repoSegments) == 0 {
				return "", "", false
			}
			rest := "/" + strings.Join(segments[i:], "/")
			return strings.Join(repoSegments, "/"), rest, true
		case "_catalog":
			return "", "", false
		}
		repoSegments = append(repoSegments, seg)
	}
	return "", "", false
}

func isDockerImmutablePath(path string) bool {
	if strings.Contains(path, "/blobs/sha256:") {
		return true
	}
	if strings.Contains(path, "/manifests/sha256:") {
		return true
	}
	return false
}
