package service

import (
	"context"
	"strings"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type captureSender struct {
	message sender.Message
}

func (s *captureSender) Send(ctx context.Context, message sender.Message) error {
	s.message = message
	return nil
}

func (s *captureSender) Platform() string { return "test" }

func TestSendJenkinsBuildRequiresCoreFields(t *testing.T) {
	svc := NewNotifyService(Config{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_test"},
	}}, map[string]sender.Sender{"test": &captureSender{}}, NewMemoryRenderer(map[string]string{
		"jenkins_test": "{{.JobName}}",
	}))

	_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{Channel: "jenkins-test", Status: "BROKEN"})
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected status validation error, got %v", err)
	}
}

func TestSendJenkinsBuildRendersAndSendsMessage(t *testing.T) {
	cap := &captureSender{}
	svc := NewNotifyService(Config{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_test", At: []string{"ou_user"}},
	}}, map[string]sender.Sender{"test": cap}, NewMemoryRenderer(map[string]string{
		"jenkins_test": "Job={{.JobName}} Status={{.Status}} Branch={{.Branch}}",
	}))

	resp, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{
		Channel: "jenkins-test", Status: "SUCCESS", JobName: "demo", BuildNumber: "12", BuildURL: "https://jenkins/job/demo/12/", Branch: "main",
	})
	if err != nil {
		t.Fatalf("SendJenkinsBuild returned error: %v", err)
	}
	if resp.Channel != "jenkins-test" || resp.Platform != "test" || !resp.Sent {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if cap.message.Title != "Jenkins 构建通知: demo #12" {
		t.Fatalf("title = %q", cap.message.Title)
	}
	if cap.message.Text != "Job=demo Status=SUCCESS Branch=main" {
		t.Fatalf("text = %q", cap.message.Text)
	}
	if len(cap.message.At) != 1 || cap.message.At[0] != "ou_user" {
		t.Fatalf("at = %#v", cap.message.At)
	}
}
