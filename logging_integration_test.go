package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggingFallbackToStdout(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("设置目录权限失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	logPath := filepath.Join(blocked, "sub", "any-hub.log")
	configPath := writeConfigFile(t, fmt.Sprintf(`
LogLevel = "info"
LogFilePath = "%s"
StoragePath = "%s"
ListenPort = 5000

[[Hub]]
Name = "docker"
Domain = "docker.local"
Upstream = "https://registry-1.docker.io"
Type = "docker"
`, logPath, filepath.Join(dir, "storage")))

	useBufferWriters(t)
	code := run(cliOptions{configPath: configPath, checkOnly: true})
	if code != 0 {
		t.Fatalf("日志 fallback 不应导致失败，得到 %d", code)
	}
	t.Log(stdOut.(*bytes.Buffer).String())
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(file, []byte(strings.TrimSpace(content)), 0o600); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}
	return file
}
