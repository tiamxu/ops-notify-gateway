package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/types"
)

func TestLoadDoesNotExpandEnv(t *testing.T) {
	t.Setenv("OPS_NOTIFY_TOKEN", "expanded-token")
	t.Setenv("FEISHU_WEBHOOK", "https://expanded.example")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
server:
  address: ":8801"
auth:
  enabled: true
  token: "${OPS_NOTIFY_TOKEN}"
channels:
  jenkins-test:
    platform: "feishu"
    webhook: "${FEISHU_WEBHOOK}"
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Auth.Token != "${OPS_NOTIFY_TOKEN}" {
		t.Fatalf("auth token should not expand env, got %q", cfg.Auth.Token)
	}
	if cfg.Channels["jenkins-test"].Webhook != "${FEISHU_WEBHOOK}" {
		t.Fatalf("webhook should not expand env, got %q", cfg.Channels["jenkins-test"].Webhook)
	}
}

func TestLoadAppliesServerDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
auth:
  enabled: false
channels:
  jenkins-test:
    platform: "feishu"
    webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/example"
`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Address != ":8801" {
		t.Fatalf("address = %q, want :8801", cfg.Server.Address)
	}
	if cfg.Server.ReadTimeoutSeconds != 10 {
		t.Fatalf("read timeout = %d, want 10", cfg.Server.ReadTimeoutSeconds)
	}
	if cfg.Server.WriteTimeoutSeconds != 10 {
		t.Fatalf("write timeout = %d, want 10", cfg.Server.WriteTimeoutSeconds)
	}
}

func TestLoadAuthDefaultsAndValidation(t *testing.T) {
	cases := []struct {
		name        string
		auth        string
		wantEnabled bool
		wantError   bool
	}{
		{name: "omitted enabled", auth: "auth:\n  token: test-token\n", wantEnabled: true},
		{name: "explicit enabled", auth: "auth:\n  enabled: true\n  token: test-token\n", wantEnabled: true},
		{name: "explicit disabled", auth: "auth:\n  enabled: false\n"},
		{name: "disabled with whitespace token", auth: "auth:\n  enabled: false\n  token: '   '\n"},
		{name: "omitted auth", wantError: true},
		{name: "empty auth", auth: "auth: {}\n", wantError: true},
		{name: "null auth", auth: "auth: null\n", wantError: true},
		{name: "null enabled", auth: "auth:\n  enabled: null\n", wantError: true},
		{name: "enabled without token", auth: "auth:\n  enabled: true\n", wantError: true},
		{name: "empty token", auth: "auth:\n  token: ''\n", wantError: true},
		{name: "whitespace token", auth: "auth:\n  token: '   '\n", wantError: true},
		{name: "enabled with whitespace token", auth: "auth:\n  enabled: true\n  token: '   '\n", wantError: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.auth + "channels:\n  ops-test:\n    platform: feishu\n    webhook: https://example.com/hook\n"
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := Load(path)
			if tt.wantError {
				assertConfigError(t, err, "auth token is required")
				return
			}
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if cfg.Auth.Enabled != tt.wantEnabled {
				t.Fatalf("auth enabled = %v, want %v", cfg.Auth.Enabled, tt.wantEnabled)
			}
		})
	}
}

func TestValidateRejectsWhitespaceAuthToken(t *testing.T) {
	for _, token := range []string{" ", "\t\n", "\u3000"} {
		t.Run(token, func(t *testing.T) {
			cfg := validConfig()
			cfg.Auth.Token = token
			assertConfigError(t, Validate(cfg), "auth token is required")
		})
	}
}

func TestValidateRejectsEnabledAuthWithoutToken(t *testing.T) {
	cfg := validConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Token = ""

	err := Validate(cfg)
	assertConfigError(t, err, "auth token is required")
}

func TestValidateRejectsEmptyChannels(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = nil

	err := Validate(cfg)
	assertConfigError(t, err, "channels is required")
}

func TestValidateRejectsInvalidChannelName(t *testing.T) {
	cfg := validConfig()
	cfg.Channels = map[string]types.ChannelConfig{
		"Jenkins Test": {Platform: "feishu", Webhook: "https://example.com/hook"},
	}

	err := Validate(cfg)
	assertConfigError(t, err, "invalid channel name")
}

func TestValidateRejectsUnsupportedPlatform(t *testing.T) {
	cfg := validConfig()
	cfg.Channels["jenkins-test"] = types.ChannelConfig{Platform: "wechat", Webhook: "https://example.com/hook"}

	err := Validate(cfg)
	assertConfigError(t, err, "unsupported platform")
}

func TestValidateRejectsEmptyWebhook(t *testing.T) {
	cfg := validConfig()
	cfg.Channels["jenkins-test"] = types.ChannelConfig{Platform: "feishu"}

	err := Validate(cfg)
	assertConfigError(t, err, "webhook is required")
}

func validConfig() types.Config {
	return types.Config{
		Auth: types.AuthConfig{Enabled: true, Token: "secret"},
		Channels: map[string]types.ChannelConfig{
			"jenkins-test": {Platform: "feishu", Webhook: "https://example.com/hook"},
		},
	}
}

func assertConfigError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want contains %q", err.Error(), want)
	}
}

func TestValidateRejectsNegativeServerTimeouts(t *testing.T) {
	for _, server := range []types.ServerConfig{{ReadTimeoutSeconds: -1}, {WriteTimeoutSeconds: -1}} {
		cfg := types.Config{Server: server, Channels: map[string]types.ChannelConfig{"ops": {Platform: "feishu", Webhook: "https://example.com/fixture"}}}
		if Validate(cfg) == nil {
			t.Fatal("negative server timeout accepted")
		}
	}
}
