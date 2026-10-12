package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/types"
	"gopkg.in/yaml.v3"
)

var channelNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func Load(path string) (types.Config, error) {
	if path == "" {
		path = "config/config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return types.Config{}, err
	}
	var raw struct {
		Server        types.ServerConfig `yaml:"server"`
		Auth          types.AuthConfig   `yaml:"auth"`
		ChannelSource struct {
			Mode     string    `yaml:"mode"`
			Platform yaml.Node `yaml:"platform"`
		} `yaml:"channel_source"`
		Channels yaml.Node `yaml:"channels"`
	}
	raw.Auth.Enabled = true
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return types.Config{}, err
	}
	cfg := types.Config{Server: raw.Server, Auth: raw.Auth, ChannelSource: types.ChannelSourceConfig{Mode: raw.ChannelSource.Mode}}
	if cfg.ChannelSource.Mode == "platform" {
		if err := raw.ChannelSource.Platform.Decode(&cfg.ChannelSource.Platform); err != nil {
			return types.Config{}, err
		}
	} else if cfg.ChannelSource.Mode == "file" || cfg.ChannelSource.Mode == "" {
		if err := raw.Channels.Decode(&cfg.Channels); err != nil {
			return types.Config{}, err
		}
	}
	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return types.Config{}, err
	}
	return cfg, nil
}

func applyDefaults(cfg *types.Config) {
	if cfg.ChannelSource.Mode == "" {
		cfg.ChannelSource.Mode = "file"
	}
	if cfg.ChannelSource.Mode == "platform" {
		if cfg.ChannelSource.Platform.SyncIntervalSeconds == 0 {
			cfg.ChannelSource.Platform.SyncIntervalSeconds = 30
		}
		if cfg.ChannelSource.Platform.TimeoutSeconds == 0 {
			cfg.ChannelSource.Platform.TimeoutSeconds = 5
		}
	}
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8801"
	}
	if cfg.Server.ReadTimeoutSeconds == 0 {
		cfg.Server.ReadTimeoutSeconds = 10
	}
	if cfg.Server.WriteTimeoutSeconds == 0 {
		cfg.Server.WriteTimeoutSeconds = 10
	}
}

func Validate(cfg types.Config) error {
	if cfg.Server.ReadTimeoutSeconds < 0 || cfg.Server.WriteTimeoutSeconds < 0 {
		return fmt.Errorf("server timeouts must not be negative")
	}
	if cfg.Auth.Enabled && strings.TrimSpace(cfg.Auth.Token) == "" {
		return fmt.Errorf("auth token is required when auth enabled")
	}
	switch cfg.ChannelSource.Mode {
	case "", "file":
	case "platform":
		return validatePlatformSource(cfg)
	default:
		return fmt.Errorf("channel_source.mode must be file or platform")
	}
	if len(cfg.Channels) == 0 {
		return fmt.Errorf("channels is required")
	}
	for name, ch := range cfg.Channels {
		if !channelNamePattern.MatchString(name) {
			return fmt.Errorf("invalid channel name: %s", name)
		}
		if ch.Platform != "dingtalk" && ch.Platform != "feishu" {
			return fmt.Errorf("unsupported platform for channel %s: %s", name, ch.Platform)
		}
		if ch.Webhook == "" {
			return fmt.Errorf("webhook is required for channel: %s", name)
		}
	}
	return nil
}

func validatePlatformSource(cfg types.Config) error {
	p := cfg.ChannelSource.Platform
	if p.URL == "" {
		return fmt.Errorf("channel_source.platform.url is required")
	}
	u, err := url.Parse(p.URL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return fmt.Errorf("channel_source.platform.url must be an HTTPS endpoint or allowlisted local HTTP endpoint without credentials, query or fragment")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLocalHTTPHost(u.Hostname())) {
		return fmt.Errorf("channel_source.platform.url requires HTTPS; HTTP is only allowed for localhost, 127.0.0.1 or ::1")
	}
	if !cfg.Auth.Enabled {
		return fmt.Errorf("platform mode requires inbound auth")
	}
	if strings.TrimSpace(p.Token) == "" || strings.ContainsAny(p.Token, "\r\n") {
		return fmt.Errorf("channel_source.platform.token is required and must be a valid header value")
	}
	if strings.TrimSpace(p.Token) == strings.TrimSpace(cfg.Auth.Token) {
		return fmt.Errorf("platform and inbound tokens must be different")
	}
	if p.SyncIntervalSeconds <= 0 || p.SyncIntervalSeconds > 86400 {
		return fmt.Errorf("sync_interval_seconds must be between 1 and 86400")
	}
	if p.TimeoutSeconds <= 0 || p.TimeoutSeconds > 300 {
		return fmt.Errorf("timeout_seconds must be between 1 and 300")
	}
	if strings.TrimSpace(p.RecoveryFile) == "" {
		return fmt.Errorf("channel_source.platform.recovery_file is required")
	}
	return nil
}

func isLocalHTTPHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
