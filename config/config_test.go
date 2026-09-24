package config

import (
	"os"
	"path/filepath"
	"testing"
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
