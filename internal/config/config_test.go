package config

import (
	"testing"
	"time"
)

func TestLoadWithDefaults(t *testing.T) {
	cfgPath := testConfigPath(t, "valid.toml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if cfg.Global.CacheTTL.DurationValue() == 0 {
		t.Fatalf("CacheTTL 应该自动填充默认值")
	}
	if cfg.Global.StoragePath == "" {
		t.Fatalf("StoragePath 应该被保留")
	}
	if cfg.Global.ListenPort == 0 {
		t.Fatalf("ListenPort 应当被解析")
	}
	if cfg.EffectiveCacheTTL(cfg.Hubs[0]) != cfg.Global.CacheTTL.DurationValue() {
		t.Fatalf("Hub 未设置 TTL 时应退回全局 TTL")
	}
}

func TestValidateRejectsBadHub(t *testing.T) {
	cfgPath := testConfigPath(t, "missing.toml")

	if _, err := Load(cfgPath); err == nil {
		t.Fatalf("不合法的配置应返回错误")
	}
}

func TestEffectiveCacheTTLOverrides(t *testing.T) {
	cfg := &Config{Global: GlobalConfig{CacheTTL: Duration(time.Hour)}}
	hub := HubConfig{CacheTTL: Duration(2 * time.Hour)}
	if ttl := cfg.EffectiveCacheTTL(hub); ttl != 2*time.Hour {
		t.Fatalf("覆盖 TTL 应该优先生效")
	}
}

func TestValidateEnforcesListenPortRange(t *testing.T) {
	cfg := validConfig()
	cfg.Global.ListenPort = 70000
	if err := cfg.Validate(); err == nil {
		t.Fatalf("ListenPort 超出范围应当报错")
	}
}

func TestHubTypeValidation(t *testing.T) {
	testCases := []struct {
		name      string
		hubType   string
		shouldErr bool
	}{
		{"docker ok", "docker", false},
		{"npm ok", "npm", false},
		{"go ok", "go", false},
		{"missing type", "", true},
		{"unsupported type", "rubygems", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Hubs[0].Type = tc.hubType
			err := cfg.Validate()
			if tc.shouldErr && err == nil {
				t.Fatalf("expected error for type %q", tc.hubType)
			}
			if !tc.shouldErr && err != nil {
				t.Fatalf("unexpected error for type %q: %v", tc.hubType, err)
			}
		})
	}
}

func TestValidateRequiresCredentialPairs(t *testing.T) {
	cfg := validConfig()
	cfg.Hubs[0].Username = "foo"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("仅提供 Username 时应报错")
	}
}

func validConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			ListenPort:      5000,
			StoragePath:     "./data",
			CacheTTL:        Duration(time.Hour),
			MaxMemoryCache:  1,
			MaxRetries:      1,
			InitialBackoff:  Duration(time.Second),
			UpstreamTimeout: Duration(time.Second),
		},
		Hubs: []HubConfig{
			{
				Name:     "npm",
				Domain:   "npm.local",
				Type:     "npm",
				Upstream: "https://registry.npmjs.org",
			},
		},
	}
}
