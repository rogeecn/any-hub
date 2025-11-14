package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfigPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	return path
}
