package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	tmpl "github.com/tiamxu/ops-notify-gateway/pkg/template"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type recordingSender struct {
	messages []sender.Message
}

func (s *recordingSender) Send(ctx context.Context, message sender.Message) error {
	s.messages = append(s.messages, message)
	return nil
}

func (s *recordingSender) Platform() string { return "test" }

func TestSendAlertmanagerSplitsAlertsAndUsesAnnotationAt(t *testing.T) {
	rec := &recordingSender{}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"alert-test": {Platform: "test", Template: "alert_test", At: []string{"default_user"}},
	}}, map[string]sender.Sender{"test": rec}, newMemoryRenderer(map[string]string{
		"alert_test": "{{range .Alerts}}{{.Labels.instance}} {{end}}",
	}))

	_, err := svc.SendAlertmanager(context.Background(), "alert-test", types.AlertmanagerReq{
		Status:      "firing",
		GroupLabels: map[string]string{"alertname": "服务异常告警"},
		CommonLabels: map[string]string{
			"level": "4",
		},
		Alerts: []types.AlertItem{
			{
				Status:      "firing",
				Labels:      map[string]string{"instance": "10.0.0.1"},
				Annotations: map[string]string{"at": "ou_user1, ou_user2"},
			},
			{
				Status:      "resolved",
				Labels:      map[string]string{"instance": "10.0.0.2"},
				Annotations: map[string]string{},
			},
		},
	}, true)
	if err != nil {
		t.Fatalf("SendAlertmanager returned error: %v", err)
	}
	if len(rec.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(rec.messages))
	}
	if rec.messages[0].Text != "10.0.0.1 " {
		t.Fatalf("first text = %q", rec.messages[0].Text)
	}
	if rec.messages[1].Text != "10.0.0.2 " {
		t.Fatalf("second text = %q", rec.messages[1].Text)
	}
	if got := rec.messages[0].At; len(got) != 2 || got[0] != "ou_user1" || got[1] != "ou_user2" {
		t.Fatalf("first at = %#v", got)
	}
	if got := rec.messages[1].At; len(got) != 1 || got[0] != "default_user" {
		t.Fatalf("second at = %#v", got)
	}
	if rec.messages[0].Level != "灾难" {
		t.Fatalf("level = %q, want 灾难", rec.messages[0].Level)
	}
	if rec.messages[0].Status != "故障" || rec.messages[1].Status != "恢复" {
		t.Fatalf("statuses = %q, %q", rec.messages[0].Status, rec.messages[1].Status)
	}
}

func TestSendAlertmanagerLevelSources(t *testing.T) {
	cases := []struct {
		name   string
		split  bool
		common map[string]string
		labels []map[string]string
		want   []string
	}{
		{name: "mixed split levels", split: true, labels: []map[string]string{{"severity": "warning"}, {"severity": "critical"}}, want: []string{"警告", "严重"}},
		{name: "individual level wins", split: true, common: map[string]string{"level": "info"}, labels: []map[string]string{{"level": " 4 ", "severity": "warning"}}, want: []string{"灾难"}},
		{name: "individual severity wins over common level", split: true, common: map[string]string{"level": "info"}, labels: []map[string]string{{"level": " ", "severity": " CRITICAL "}}, want: []string{"严重"}},
		{name: "fallback common level", split: true, common: map[string]string{"level": "2", "severity": "warning"}, labels: []map[string]string{nil}, want: []string{"一般严重"}},
		{name: "fallback common severity", split: true, common: map[string]string{"severity": "warning"}, labels: []map[string]string{{"level": " ", "severity": " "}}, want: []string{"警告"}},
		{name: "missing labels", split: true, labels: []map[string]string{nil}, want: []string{"unknown"}},
		{name: "custom level", split: true, common: map[string]string{"severity": "warning"}, labels: []map[string]string{{"level": " CUSTOM "}}, want: []string{"custom"}},
		{name: "combined single uses common", common: map[string]string{"severity": "info"}, labels: []map[string]string{{"level": "critical"}}, want: []string{"信息"}},
		{name: "combined mixed stays unknown", labels: []map[string]string{{"severity": "warning"}, {"severity": "critical"}}, want: []string{"unknown"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingSender{}
			svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
				"alert-test": {Platform: "test", Template: "alert_test"},
			}}, map[string]sender.Sender{"test": rec}, newMemoryRenderer(map[string]string{
				"alert_test": "{{.LevelText}}|{{.CommonLabels}}",
			}))
			req := types.AlertmanagerReq{CommonLabels: tt.common}
			for _, labels := range tt.labels {
				req.Alerts = append(req.Alerts, types.AlertItem{Status: "firing", Labels: labels})
			}
			commonBefore := fmt.Sprint(req.CommonLabels)
			_, err := svc.SendAlertmanager(context.Background(), "alert-test", req, tt.split)
			if err != nil {
				t.Fatalf("SendAlertmanager returned error: %v", err)
			}
			if len(rec.messages) != len(tt.want) {
				t.Fatalf("messages = %d, want %d", len(rec.messages), len(tt.want))
			}
			for i, want := range tt.want {
				got := rec.messages[i]
				if got.Level != want || got.Text != want+"|"+commonBefore {
					t.Fatalf("message %d: level=%q text=%q, want level=%q with original common labels", i, got.Level, got.Text, want)
				}
				if got.Status != "故障" || got.Color != "red" {
					t.Fatalf("unexpected status/color: %q/%q", got.Status, got.Color)
				}
			}
			if fmt.Sprint(req.CommonLabels) != commonBefore {
				t.Fatal("common labels were modified")
			}
		})
	}
}

func TestSendAlertmanagerUsesStatusColor(t *testing.T) {
	cases := []struct {
		name       string
		alerts     []types.AlertItem
		wantStatus string
		wantColor  string
	}{
		{
			name: "firing",
			alerts: []types.AlertItem{
				{Status: "firing", Annotations: map[string]string{}},
			},
			wantStatus: "故障",
			wantColor:  "red",
		},
		{
			name: "resolved",
			alerts: []types.AlertItem{
				{Status: "resolved", Annotations: map[string]string{}},
			},
			wantStatus: "恢复",
			wantColor:  "green",
		},
		{
			name: "mixed",
			alerts: []types.AlertItem{
				{Status: "firing", Annotations: map[string]string{}},
				{Status: "resolved", Annotations: map[string]string{}},
			},
			wantStatus: "故障",
			wantColor:  "orange",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingSender{}
			svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
				"alert-test": {Platform: "test", Template: "alert_test"},
			}}, map[string]sender.Sender{"test": rec}, newMemoryRenderer(map[string]string{
				"alert_test": "{{.StatusText}} {{.StatusColor}}",
			}))

			_, err := svc.SendAlertmanager(context.Background(), "alert-test", types.AlertmanagerReq{
				GroupLabels: map[string]string{"alertname": "服务异常告警"},
				Alerts:      tt.alerts,
			}, false)
			if err != nil {
				t.Fatalf("SendAlertmanager returned error: %v", err)
			}
			if len(rec.messages) != 1 {
				t.Fatalf("messages = %d, want 1", len(rec.messages))
			}
			if rec.messages[0].Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", rec.messages[0].Status, tt.wantStatus)
			}
			if rec.messages[0].Color != tt.wantColor {
				t.Fatalf("color = %q, want %q", rec.messages[0].Color, tt.wantColor)
			}
			if rec.messages[0].Title != "服务异常告警" {
				t.Fatalf("title = %q", rec.messages[0].Title)
			}
		})
	}
}

func TestSendAlertmanagerTemplatesRenderStatus(t *testing.T) {
	cases := []struct {
		name     string
		template string
		alerts   []types.AlertItem
		wantText []string
	}{
		{
			name:     "dingtalk mixed",
			template: "alertmanager_dingtalk",
			alerts: []types.AlertItem{
				{Status: "firing", Labels: map[string]string{"instance": "10.0.0.1"}, Annotations: map[string]string{"summary": "CPU 高"}},
				{Status: "resolved", Labels: map[string]string{"instance": "10.0.0.2"}, Annotations: map[string]string{"recovery_description": "已恢复"}},
			},
			wantText: []string{"#faad14", "服务异常告警 [警告] [故障]", "<font color=\"#faad14\">触发时间</font>"},
		},
		{
			name:     "feishu resolved",
			template: "alertmanager_feishu",
			alerts: []types.AlertItem{
				{Status: "resolved", Labels: map[string]string{"instance": "10.0.0.1"}, Annotations: map[string]string{"recovery_description": "已恢复"}},
			},
			wantText: []string{"<font color=\"#02b340\">恢复时间</font>"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recordingSender{}
			svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
				"alert-test": {Platform: "test", Template: tt.template},
			}}, map[string]sender.Sender{"test": rec}, tmpl.NewFileRenderer("../templates"))

			_, err := svc.SendAlertmanager(context.Background(), "alert-test", types.AlertmanagerReq{
				GroupLabels:  map[string]string{"alertname": "服务异常告警"},
				CommonLabels: map[string]string{"severity": "warning"},
				Alerts:       tt.alerts,
			}, false)
			if err != nil {
				t.Fatalf("SendAlertmanager returned error: %v", err)
			}
			if len(rec.messages) != 1 {
				t.Fatalf("messages = %d, want 1", len(rec.messages))
			}
			for _, want := range tt.wantText {
				if !strings.Contains(rec.messages[0].Text, want) {
					t.Fatalf("text missing %q:\n%s", want, rec.messages[0].Text)
				}
			}
		})
	}
}

func TestSendAlertmanagerMapsSeverityLevel(t *testing.T) {
	rec := &recordingSender{}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"alert-test": {Platform: "test", Template: "alert_test"},
	}}, map[string]sender.Sender{"test": rec}, newMemoryRenderer(map[string]string{
		"alert_test": "{{range .Alerts}}{{.Status}}{{end}}",
	}))

	_, err := svc.SendAlertmanager(context.Background(), "alert-test", types.AlertmanagerReq{
		GroupLabels:  map[string]string{"alertname": "服务异常告警"},
		CommonLabels: map[string]string{"severity": "warning"},
		Alerts: []types.AlertItem{
			{Status: "firing", Annotations: map[string]string{}},
		},
	}, false)
	if err != nil {
		t.Fatalf("SendAlertmanager returned error: %v", err)
	}
	if len(rec.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(rec.messages))
	}
	if rec.messages[0].Level != "警告" {
		t.Fatalf("level = %q, want 警告", rec.messages[0].Level)
	}
}

func TestCanceledRequestDoesNotStartAnotherAlert(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	snd := cancelAfterSend{cancel: cancel, calls: &calls}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{"ops": {Platform: "test"}}}, map[string]sender.Sender{"ops": snd}, newMemoryRenderer(map[string]string{"alertmanager_test": "alert"}))
	_, err := svc.SendAlertmanager(ctx, "ops", types.AlertmanagerReq{Alerts: []types.AlertItem{{Status: "firing"}, {Status: "firing"}}}, true)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v sends=%d, want cancellation and one send", err, calls)
	}
}

type cancelAfterSend struct {
	cancel context.CancelFunc
	calls  *int
}

func (s cancelAfterSend) Platform() string { return "test" }
func (s cancelAfterSend) Send(context.Context, sender.Message) error {
	*s.calls++
	s.cancel()
	return nil
}
