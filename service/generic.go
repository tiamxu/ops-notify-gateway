package service

import (
	"context"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func (s *NotifyService) SendGeneric(ctx context.Context, req types.GenericNotifyReq) (types.NotifyResult, error) {
	if strings.TrimSpace(req.Channel) == "" {
		return types.NotifyResult{}, e.BadRequest("channel is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return types.NotifyResult{}, e.BadRequest("title is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return types.NotifyResult{}, e.BadRequest("content is required")
	}
	ch, _, err := s.channel(req.Channel)
	if err != nil {
		return types.NotifyResult{}, err
	}
	templateName := ch.Template
	if templateName == "" {
		templateName = defaultTemplate("generic", ch.Platform)
	}
	text, err := s.renderer.Render(templateName, req)
	if err != nil {
		return types.NotifyResult{}, e.Internal("render generic template failed")
	}
	at := ch.At
	if len(req.At) > 0 {
		at = req.At
	}
	return s.send(ctx, req.Channel, sender.Message{Title: req.Title, Text: text, Status: req.Status, Level: req.Level, Color: genericColor(req.Status), At: at})
}

func genericColor(status string) string {
	switch strings.ToUpper(status) {
	case "SUCCESS", "OK", "RESOLVED":
		return "green"
	case "WARNING", "UNSTABLE":
		return "orange"
	case "FAILURE", "FAILED", "FIRING", "ERROR":
		return "red"
	default:
		return "blue"
	}
}
