package service

import (
	"context"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func (s *NotifyService) SendAlertmanager(ctx context.Context, channel string, req types.AlertmanagerReq, split bool) (types.NotifyResult, error) {
	if strings.TrimSpace(channel) == "" {
		return types.NotifyResult{}, e.BadRequest("channel is required")
	}
	if len(req.Alerts) == 0 {
		return types.NotifyResult{}, e.BadRequest("alerts is required")
	}
	channel, ch, err := s.channelConfig(ctx, channel)
	if err != nil {
		return types.NotifyResult{}, err
	}
	templateName := ch.Template
	if templateName == "" {
		templateName = defaultTemplate("alertmanager", ch.Platform)
	}
	if split {
		var result types.NotifyResult
		for _, alert := range req.Alerts {
			single := req
			single.Status = alert.Status
			single.Alerts = []types.AlertItem{alert}
			sent, err := s.sendAlertmanagerMessage(ctx, channel, ch, templateName, single, alertAt(ch.At, alert.Annotations), alertLevel(single, alert.Labels))
			if err != nil {
				return types.NotifyResult{}, err
			}
			result = sent
		}
		return result, nil
	}
	return s.sendAlertmanagerMessage(ctx, channel, ch, templateName, req, alertAt(ch.At, req.Alerts[0].Annotations), alertLevel(req, nil))
}

type alertmanagerTemplateData struct {
	types.AlertmanagerReq
	StatusText  string
	StatusColor string
	LevelText   string
}

type alertSummary struct {
	status string
	color  string
	hex    string
}

func (s *NotifyService) sendAlertmanagerMessage(ctx context.Context, channel string, ch types.ChannelConfig, templateName string, req types.AlertmanagerReq, at []string, level string) (types.NotifyResult, error) {
	summary := summarizeAlert(req)
	text, err := s.renderer.Render(templateName, newAlertmanagerTemplateData(req, summary, level))
	if err != nil {
		return types.NotifyResult{}, e.Internal("render alertmanager template failed")
	}
	title := req.GroupLabels["alertname"]
	if title == "" {
		title = "Alertmanager 告警通知"
	}
	return s.send(ctx, channel, ch, sender.Message{Title: title, Text: text, Status: summary.status, Level: level, Color: summary.color, At: at})
}

func newAlertmanagerTemplateData(req types.AlertmanagerReq, summary alertSummary, level string) alertmanagerTemplateData {
	return alertmanagerTemplateData{AlertmanagerReq: req, StatusText: summary.status, StatusColor: summary.hex, LevelText: level}
}

func summarizeAlert(req types.AlertmanagerReq) alertSummary {
	hasFiring := false
	hasResolved := false
	for _, alert := range req.Alerts {
		if strings.ToLower(strings.TrimSpace(alert.Status)) == "resolved" {
			hasResolved = true
			continue
		}
		hasFiring = true
	}
	if !hasFiring && !hasResolved && strings.ToLower(strings.TrimSpace(req.Status)) == "resolved" {
		hasResolved = true
	}
	switch {
	case hasFiring && hasResolved:
		return alertSummary{status: "故障", color: "orange", hex: "#faad14"}
	case hasResolved:
		return alertSummary{status: "恢复", color: "green", hex: "#02b340"}
	default:
		return alertSummary{status: "故障", color: "red", hex: "#FF0000"}
	}
}

func alertLevel(req types.AlertmanagerReq, labels map[string]string) string {
	for _, value := range []string{labels["level"], labels["severity"], req.CommonLabels["level"], req.CommonLabels["severity"]} {
		level := strings.ToLower(strings.TrimSpace(value))
		if level == "" {
			continue
		}
		if name, ok := alertLevelNames[level]; ok {
			return name
		}
		return level
	}
	return "unknown"
}

var alertLevelNames = map[string]string{
	"info":     "信息",
	"warning":  "警告",
	"critical": "严重",
	"0":        "信息",
	"1":        "警告",
	"2":        "一般严重",
	"3":        "严重",
	"4":        "灾难",
}

func alertAt(defaultAt []string, annotations map[string]string) []string {
	if annotations == nil {
		return defaultAt
	}
	at, ok := annotations["at"]
	if !ok {
		return defaultAt
	}
	parts := strings.Split(at, ",")
	ret := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			ret = append(ret, item)
		}
	}
	if len(ret) == 0 {
		return defaultAt
	}
	return ret
}
