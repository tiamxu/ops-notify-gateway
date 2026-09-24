package types

type Config struct {
	Server   ServerConfig             `yaml:"server"`
	Auth     AuthConfig               `yaml:"auth"`
	Channels map[string]ChannelConfig `yaml:"channels"`
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
}
