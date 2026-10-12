package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSelectsOnlyActiveChannelSource(t *testing.T) {
	t.Setenv("NOTIFY_CONFIG_ENCRYPTION_KEY", "")
	cases := []struct{ name, body, want string }{
		{"file ignores platform type errors", "channel_source:\n  mode: file\n  platform:\n    timeout_seconds: unused\nchannels:\n  local:\n    platform: feishu\n    webhook: local-placeholder\n", ""},
		{"file ignores incomplete platform", "channel_source:\n  mode: file\n  platform:\n    url: invalid\n    timeout_seconds: -1\nchannels:\n  local:\n    platform: feishu\n    webhook: local-placeholder\n", ""},
		{"platform ignores invalid local channel", "channel_source:\n  mode: platform\n  platform:\n    url: https://platform.example/channels\n    token: read-fixture\n    recovery_file: /tmp/fixture.yaml\nchannels:\n  Invalid Name:\n    platform: invalid\n", ""},
		{"unknown mode", "channel_source:\n  mode: remote\nchannels:\n  local:\n    platform: feishu\n    webhook: local-placeholder\n", "mode"},
		{"platform requires URL", "channel_source:\n  mode: platform\n", "url"},
		{"platform rejects HTTP", "channel_source:\n  mode: platform\n  platform:\n    url: http://platform.example/channels\n", "HTTPS"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("auth:\n  enabled: true\n  token: inbound-fixture\n"+tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestPlatformModeLoadsDefaultsAndIgnoresLocalChannels(t *testing.T) {
	t.Setenv("NOTIFY_CONFIG_ENCRYPTION_KEY", "")
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `auth:
  token: inbound-fixture
channel_source:
  mode: platform
  platform:
    url: https://platform.example/api/v1/internal/notification-channels
    token: read-fixture
    recovery_file: /tmp/recovery-fixture.yaml
channels:
  Invalid Channel:
    platform: invalid
    enabled: unused
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.ChannelSource.Platform
	if p.SyncIntervalSeconds != 30 || p.TimeoutSeconds != 5 {
		t.Fatal("platform defaults missing")
	}
	for _, tt := range []struct {
		name string
		edit func()
	}{
		{"shared token", func() { cfg.ChannelSource.Platform.Token = cfg.Auth.Token }},
		{"shared token with spaces", func() { cfg.ChannelSource.Platform.Token = " " + cfg.Auth.Token + " " }},
		{"negative interval", func() { cfg.ChannelSource.Platform.SyncIntervalSeconds = -1 }},
		{"zero timeout", func() { cfg.ChannelSource.Platform.TimeoutSeconds = 0 }},
		{"missing file", func() { cfg.ChannelSource.Platform.RecoveryFile = "" }},
		{"auth disabled", func() { cfg.Auth.Enabled = false }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg.ChannelSource.Platform = p
			cfg.Auth.Enabled = true
			tt.edit()
			if Validate(cfg) == nil {
				t.Fatal("accepted invalid platform configuration")
			}
		})
	}
}

func TestPlatformModeDoesNotUseRecoveryKey(t *testing.T) {
	for _, key := range []string{"", "obsolete-invalid-key"} {
		t.Setenv("NOTIFY_CONFIG_ENCRYPTION_KEY", key)
		path := filepath.Join(t.TempDir(), "config.yaml")
		body := "auth:\n  token: inbound-fixture\nchannel_source:\n  mode: platform\n  platform:\n    url: https://platform.example/channels\n    token: read-fixture\n    recovery_file: /tmp/channels.snapshot.yaml\n"
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err != nil {
			t.Fatalf("YAML recovery must not require a key: %v", err)
		}
	}
}

func TestPlatformEndpointHTTPAllowlist(t *testing.T) {
	for _, tt := range []struct {
		endpoint string
		allowed  bool
	}{
		{"http://localhost:8080/api/v1/internal/notification-channels", true},
		{"http://LOCALHOST:8080/channels", true},
		{"http://127.0.0.1:8080/channels", true},
		{"http://[::1]:8080/channels", true},
		{"http://localhost/channels", true},
		{"https://platform.example/channels", true},
		{"https://127.0.0.1:8443/channels", true},
		{"http://platform.example/channels", false},
		{"http://192.168.1.2:8080/channels", false},
		{"http://127.0.0.2/channels", false},
		{"http://0.0.0.0/channels", false},
		{"http://host.docker.internal/channels", false},
		{"http://localhost.example/channels", false},
		{"http://127.0.0.1.example/channels", false},
		{"http://[::ffff:127.0.0.1]/channels", false},
		{"http://localhost@platform.example/channels", false},
		{"http://fixture@localhost/channels", false},
		{"http://localhost/channels?token=fixture", false},
		{"http://localhost/channels#fragment", false},
		{"ftp://localhost/channels", false},
		{"http:///channels", false},
		{"http://localhost:invalid/channels", false},
	} {
		t.Run(tt.endpoint, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			body := fmt.Sprintf("auth:\n  token: inbound-fixture\nchannel_source:\n  mode: platform\n  platform:\n    url: %q\n    token: read-fixture\n    recovery_file: ./data/channels.snapshot.yaml\n", tt.endpoint)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if (err == nil) != tt.allowed {
				t.Fatalf("allowed=%t, error=%v", tt.allowed, err)
			}
		})
	}
}
