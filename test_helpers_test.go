package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

var repoRoot string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if ok {
		repoRoot = filepath.Join(filepath.Dir(file), "..", "..")
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
