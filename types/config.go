package types

type Config struct {
	ChannelSource ChannelSourceConfig      `yaml:"channel_source"`
	Server        ServerConfig             `yaml:"server"`
	Auth          AuthConfig               `yaml:"auth"`
	Channels      map[string]ChannelConfig `yaml:"channels"`
}

type ChannelSourceConfig struct {
	Mode     string               `yaml:"mode"`
	Platform PlatformSourceConfig `yaml:"platform"`
}

type PlatformSourceConfig struct {
	URL                 string `yaml:"url"`
	Token               string `yaml:"token"`
	SyncIntervalSeconds int    `yaml:"sync_interval_seconds"`
	TimeoutSeconds      int    `yaml:"timeout_seconds"`
	RecoveryFile        string `yaml:"recovery_file"`
}

type ServerConfig struct {
	Address             string `yaml:"address"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

type AuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

type ChannelConfig struct {
	Platform string   `yaml:"platform"`
	Webhook  string   `yaml:"webhook"`
	Secret   string   `yaml:"secret"`
	Template string   `yaml:"template"`
	At       []string `yaml:"at"`
	Enabled  *bool    `yaml:"enabled"`
}
