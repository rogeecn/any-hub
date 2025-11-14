package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/any-hub/any-hub/internal/config"
)

func TestConfigureDefaultsToStdout(t *testing.T) {
	logger, err := InitLogger(config.GlobalConfig{LogLevel: "info"})
	if err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	if logger.Out != os.Stdout {
		t.Fatalf("未指定文件时应输出到 stdout")
	}
}

func TestInitLoggerFallbackOnPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatalf("设置目录权限失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	cfg := config.GlobalConfig{
		LogLevel:    "info",
		LogFilePath: filepath.Join(blocked, "sub", "any-hub.log"),
	}
	logger, err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("初始化不应失败: %v", err)
	}
	if logger.Out != os.Stdout {
		t.Fatalf("fallback 时应退回 stdout")
	}
}

func TestConfigureCreatesRotatingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "any-hub.log")
	cfg := config.GlobalConfig{LogLevel: "debug", LogFilePath: path}
	logger, err := InitLogger(cfg)
	if err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	logger.Info("test")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("预期创建日志文件: %v", err)
	}
}
