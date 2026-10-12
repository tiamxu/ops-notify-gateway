package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

var allowedJenkinsStatus = map[string]bool{
	"SUCCESS":  true,
	"FAILURE":  true,
	"ABORTED":  true,
	"UNSTABLE": true,
}

type jenkinsTemplateData struct {
	types.JenkinsNotifyReq
	DeployText string
}

func (s *NotifyService) SendJenkinsBuild(ctx context.Context, req types.JenkinsNotifyReq) (types.NotifyResult, error) {
	req.Status = strings.ToUpper(strings.TrimSpace(req.Status))
	if err := validateJenkins(req); err != nil {
		return types.NotifyResult{}, err
	}
	channel, ch, err := s.channelConfig(ctx, req.Channel)
	if err != nil {
		return types.NotifyResult{}, err
	}
	templateName := ch.Template
	if templateName == "" {
		templateName = defaultTemplate("jenkins", ch.Platform)
	}
	text, err := s.renderer.Render(templateName, newJenkinsTemplateData(req))
	if err != nil {
		return types.NotifyResult{}, e.Internal("render jenkins template failed")
	}
	message := sender.Message{
		Title:  fmt.Sprintf("Jenkins 构建通知: %s #%s", req.JobName, req.BuildNumber),
		Text:   text,
		Status: req.Status,
		Level:  jenkinsLevel(req.Status),
		Color:  jenkinsColor(req.Status),
		At:     ch.At,
	}
	return s.send(ctx, channel, ch, message)
}

func newJenkinsTemplateData(req types.JenkinsNotifyReq) jenkinsTemplateData {
	return jenkinsTemplateData{JenkinsNotifyReq: req, DeployText: deployText(req.Deploy)}
}

func deployText(deploy *bool) string {
	if deploy == nil {
		return ""
	}
	if *deploy {
		return "是"
	}
	return "否"
}

func validateJenkins(req types.JenkinsNotifyReq) error {
	if strings.TrimSpace(req.Channel) == "" {
		return e.BadRequest("channel is required")
	}
	if !allowedJenkinsStatus[req.Status] {
		return e.BadRequest("status must be one of: SUCCESS, FAILURE, ABORTED, UNSTABLE")
	}
	if strings.TrimSpace(req.JobName) == "" {
		return e.BadRequest("jobName is required")
	}
	if strings.TrimSpace(req.BuildNumber) == "" {
		return e.BadRequest("buildNumber is required")
	}
	if strings.TrimSpace(req.BuildURL) == "" {
		return e.BadRequest("buildUrl is required")
	}
	return nil
}

func jenkinsLevel(status string) string {
	switch status {
	case "SUCCESS":
		return "info"
	case "UNSTABLE":
		return "warning"
	default:
		return "critical"
	}
}

func jenkinsColor(status string) string {
	switch status {
	case "SUCCESS":
		return "green"
	case "UNSTABLE":
		return "orange"
	default:
		return "red"
	}
}
