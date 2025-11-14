package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCLIFlagsPriority(t *testing.T) {
	t.Setenv("ANY_HUB_CONFIG", "/tmp/env.toml")

	opts, err := parseCLIFlags([]string{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if opts.configPath != "/tmp/env.toml" {
		t.Fatalf("应优先使用环境变量，得到 %s", opts.configPath)
	}

	opts, err = parseCLIFlags([]string{"--config", "/tmp/flag.toml"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if opts.configPath != "/tmp/flag.toml" {
		t.Fatalf("flag 应高于环境变量，得到 %s", opts.configPath)
	}
}

func TestRunCheckConfigSuccess(t *testing.T) {
	useBufferWriters(t)
	code := run(cliOptions{configPath: configFixture(t, "valid.toml"), checkOnly: true})
	if code != 0 {
		t.Fatalf("期望退出码 0，得到 %d", code)
	}
}

func TestRunCheckConfigFailure(t *testing.T) {
	useBufferWriters(t)
	code := run(cliOptions{configPath: configFixture(t, "missing.toml"), checkOnly: true})
	if code == 0 {
		t.Fatalf("无效配置应返回非零退出码")
	}
}

func TestRunVersionOutput(t *testing.T) {
	useBufferWriters(t)
	code := run(cliOptions{showVersion: true})
	if code != 0 {
		t.Fatalf("version 模式应成功退出，得到 %d", code)
	}
	if !strings.Contains(stdOut.(*bytes.Buffer).String(), "any-hub") {
		t.Fatalf("version 输出应包含 any-hub 标识")
	}
}
