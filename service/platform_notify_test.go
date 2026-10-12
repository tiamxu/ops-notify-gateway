package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/types"
)

type notificationTransport func(*http.Request) (*http.Response, error)

func (f notificationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPlatformSnapshotDrivesDirectNotifications(t *testing.T) {
	var enabled atomic.Bool
	enabled.Store(true)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := "false"
		if enabled.Load() {
			value = "true"
		}
		w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[{"code":"ops","platform":"dingtalk","webhook":"https://oapi.dingtalk.com/robot/send?access_token=fixture","secret":"","at":["13800138000"],"enabled":` + value + `}]}}`))
	}))
	defer server.Close()
	cfg := types.PlatformSourceConfig{URL: server.URL, Token: "read-fixture", SyncIntervalSeconds: 30, TimeoutSeconds: 5, RecoveryFile: filepath.Join(t.TempDir(), "recovery.yaml")}
	store, err := NewPlatformChannelStore(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	sends := 0
	senderClient := &http.Client{Transport: notificationTransport(func(r *http.Request) (*http.Response, error) {
		sends++
		if r.URL.Hostname() != "oapi.dingtalk.com" || r.URL.Query().Get("access_token") != "fixture" {
			t.Error("incorrect destination")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "13800138000") || !strings.Contains(string(body), "hello") {
			t.Error("snapshot recipients/default template not used")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"errcode":0}`)), Header: make(http.Header)}, nil
	})}
	svc := NewNotifyServiceWithStore(store, NewConfigSenderFactory(senderClient), newMemoryRenderer(map[string]string{"generic_dingtalk": "{{.Content}}"}))
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sends != 0 {
		t.Fatal("sync must not send notification")
	}
	req := types.GenericNotifyReq{Channel: "ops", Title: "test", Content: "hello"}
	if _, err := svc.SendGeneric(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatal("notification did not use platform channel")
	}
	req.At = []string{"invalid recipient"}
	if _, err := svc.SendGeneric(context.Background(), req); err == nil {
		t.Fatal("invalid overriding recipient accepted")
	}
	if sends != 1 {
		t.Fatal("invalid recipient reached sender")
	}
	req.At = nil
	enabled.Store(false)
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendGeneric(context.Background(), req); err == nil {
		t.Fatal("disabled channel sent")
	}
	if sends != 1 {
		t.Fatal("disabled channel reached sender")
	}
}
