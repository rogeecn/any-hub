package config

import "testing"

func TestLoadFailsWithMissingFields(t *testing.T) {
	if _, err := Load(testConfigPath(t, "missing.toml")); err == nil {
		t.Fatalf("缺失字段的配置应返回错误")
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	cfg := `
LogLevel = "info"
StoragePath = "./data"
CacheTTL = "boom"

[[Hub]]
Name = "docker"
Domain = "docker.local"
Type = "docker"
Upstream = "https://registry-1.docker.io"
`
	path := writeTempConfig(t, cfg)
	if _, err := Load(path); err == nil {
		t.Fatalf("无效 Duration 应失败")
	}
}
