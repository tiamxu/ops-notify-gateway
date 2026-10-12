package service

import (
	"context"
	"strings"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func TestStaticChannelStoreReturnsChannel(t *testing.T) {
	store := NewStaticChannelStore(map[string]types.ChannelConfig{
		"alert-test": {Platform: "test"},
	})

	ch, ok, err := store.Get(context.Background(), "alert-test")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected channel to exist")
	}
	if ch.Platform != "test" {
		t.Fatalf("platform = %q, want test", ch.Platform)
	}
}

func TestNotifyServiceRejectsDisabledChannel(t *testing.T) {
	disabled := false
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"alert-test": {Platform: "test", Template: "alert_test", Enabled: &disabled},
	}}, map[string]sender.Sender{"test": &captureSender{}}, newMemoryRenderer(map[string]string{
		"alert_test": "{{.Status}}",
	}))

	_, err := svc.SendAlertmanager(context.Background(), "alert-test", types.AlertmanagerReq{
		Status:      "firing",
		GroupLabels: map[string]string{"alertname": "demo"},
		Alerts: []types.AlertItem{
			{Status: "firing"},
		},
	}, false)
	if err == nil || !strings.Contains(err.Error(), "channel disabled") {
		t.Fatalf("expected disabled channel error, got %v", err)
	}
}
