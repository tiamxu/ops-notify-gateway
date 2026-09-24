package config

import (
	"fmt"
	"os"
	"regexp"

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
	var cfg types.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return types.Config{}, err
	}
	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return types.Config{}, err
	}
	return cfg, nil
}

func applyDefaults(cfg *types.Config) {
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
	if cfg.Auth.Enabled && cfg.Auth.Token == "" {
		return fmt.Errorf("auth token is required when auth enabled")
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
