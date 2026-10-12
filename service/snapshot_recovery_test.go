package service

import (
	"context"
	"github.com/tiamxu/ops-notify-gateway/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLRecoveryRejectsInvalidFiles(t *testing.T) {
	valid := "version: 1\nsource: https://platform.example/channels\nsynced_at: 2026-10-11T00:00:00Z\nsnapshot:\n  version: 1\n  channels: []\n"
	for _, tt := range []struct{ name, body string }{
		{"empty", ""},
		{"malformed", "snapshot: ["},
		{"old binary", string([]byte{0xff, 0x00, 0x01})},
		{"wrong source", strings.Replace(valid, "https://platform.example/channels", "https://other.example/channels", 1)},
		{"wrong version", strings.Replace(valid, "version: 1", "version: 2", 1)},
		{"missing time", strings.Replace(valid, "synced_at: 2026-10-11T00:00:00Z\n", "", 1)},
		{"unknown field", valid + "unexpected: true\n"},
		{"multiple documents", valid + "---\nversion: 1\n"},
		{"duplicate field", valid + "version: 1\n"},
		{"missing channels", strings.Replace(valid, "  channels: []\n", "", 1)},
		{"invalid channel", strings.Replace(valid, "channels: []", "channels:\n    - code: ops\n      platform: feishu\n      webhook: https://open.feishu.cn/open-apis/bot/v2/hook/fixture", 1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "channels.snapshot.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			store, err := NewPlatformChannelStore(types.PlatformSourceConfig{URL: "https://platform.example/channels", RecoveryFile: path, TimeoutSeconds: 5, SyncIntervalSeconds: 30}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.LoadRecovery(); err == nil {
				t.Fatal("accepted invalid YAML recovery")
			}
			if store.Status().Ready {
				t.Fatal("invalid recovery is ready")
			}
			if _, _, err := store.Get(context.Background(), "ops"); err == nil {
				t.Fatal("invalid recovery usable for sending")
			}
		})
	}
	for _, kind := range []string{"permissions", "symlink", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "channels.snapshot.yaml")
			if kind == "symlink" {
				target := path + ".target"
				if err := os.WriteFile(target, []byte(valid), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(path, []byte(valid), 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "permissions" {
					if err := os.Chmod(path, 0644); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Truncate(path, maxRecoveryBytes+1); err != nil {
						t.Fatal(err)
					}
				}
			}
			recovery, err := newSnapshotRecovery(path, "https://platform.example/channels")
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := recovery.load(); err == nil {
				t.Fatal("accepted unsafe recovery file")
			}
		})
	}
}
