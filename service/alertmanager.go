package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func (s *NotifyService) SendAlertmanager(ctx context.Context, channel string, req types.AlertmanagerReq) (types.NotifyResult, error) {
	if strings.TrimSpace(channel) == "" {
		return types.NotifyResult{}, e.BadRequest("channel is required")
	}
	if len(req.Alerts) == 0 {
		return types.NotifyResult{}, e.BadRequest("alerts is required")
	}
	ch, _, err := s.channel(channel)
	if err != nil {
		return types.NotifyResult{}, err
	}
	templateName := ch.Template
	if templateName == "" {
		templateName = defaultTemplate("alertmanager", ch.Platform)
	}
	text, err := s.renderer.Render(templateName, req)
	if err != nil {
		return types.NotifyResult{}, e.Internal("render alertmanager template failed")
	}
	title := req.GroupLabels["alertname"]
	if title == "" {
		title = "Alertmanager 告警通知"
	}
	status := alertStatus(req)
	return s.send(ctx, channel, sender.Message{Title: fmt.Sprintf("%s [%s]", title, status), Text: text, Status: status, Level: alertLevel(req), Color: alertColor(status), At: ch.At})
}

func alertStatus(req types.AlertmanagerReq) string {
	if strings.ToLower(req.Status) == "resolved" {
		return "恢复"
	}
	return "故障"
}

func alertColor(status string) string {
	if status == "恢复" {
		return "green"
	}
	return "red"
}

func alertLevel(req types.AlertmanagerReq) string {
	level := req.CommonLabels["level"]
	if level == "" {
		level = req.CommonLabels["severity"]
	}
	if level == "" {
		return "unknown"
	}
	return level
}
