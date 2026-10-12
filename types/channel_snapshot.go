package types

import "time"

type ChannelSnapshot struct {
	Version  int               `json:"version" yaml:"version"`
	Channels []SnapshotChannel `json:"channels" yaml:"channels"`
}

type SnapshotChannel struct {
	Code     string   `json:"code" yaml:"code"`
	Platform string   `json:"platform" yaml:"platform"`
	Webhook  string   `json:"webhook" yaml:"webhook"`
	Secret   string   `json:"secret" yaml:"secret"`
	At       []string `json:"at" yaml:"at"`
	Enabled  *bool    `json:"enabled" yaml:"enabled"`
}

type ChannelSourceStatus struct {
	Ready             bool       `json:"ready"`
	Mode              string     `json:"mode"`
	SyncStatus        string     `json:"sync_status,omitempty"`
	PersistenceStatus string     `json:"persistence_status,omitempty"`
	SyncError         string     `json:"sync_error,omitempty"`
	PersistenceError  string     `json:"persistence_error,omitempty"`
	LastSyncAt        *time.Time `json:"last_sync_at,omitempty"`
	LastSavedAt       *time.Time `json:"last_saved_at,omitempty"`
}
