package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var repoRoot string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			repoRoot = dir
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	if repoRoot == "" {
		t.Fatal("无法定位项目根目录")
	}
	return repoRoot
}

func configFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(projectRoot(t), "internal", "config", "testdata", name)
}
