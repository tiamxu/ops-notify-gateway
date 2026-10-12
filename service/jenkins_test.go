package service

import (
	"context"
	"strings"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	tmpl "github.com/tiamxu/ops-notify-gateway/pkg/template"
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
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_test"},
	}}, map[string]sender.Sender{"test": &captureSender{}}, newMemoryRenderer(map[string]string{
		"jenkins_test": "{{.JobName}}",
	}))

	_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{Channel: "jenkins-test", Status: "BROKEN"})
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected status validation error, got %v", err)
	}
}

func TestSendJenkinsBuildRendersAndSendsMessage(t *testing.T) {
	cap := &captureSender{}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_test", At: []string{"ou_user"}},
	}}, map[string]sender.Sender{"test": cap}, newMemoryRenderer(map[string]string{
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

func TestSendJenkinsBuildRendersDeployText(t *testing.T) {
	cases := []struct {
		name     string
		deploy   *bool
		wantText string
	}{
		{name: "omitted", deploy: nil, wantText: "Deploy="},
		{name: "true", deploy: boolPtr(true), wantText: "Deploy=是"},
		{name: "false", deploy: boolPtr(false), wantText: "Deploy=否"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cap := &captureSender{}
			svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
				"jenkins-test": {Platform: "test", Template: "jenkins_test"},
			}}, map[string]sender.Sender{"test": cap}, newMemoryRenderer(map[string]string{
				"jenkins_test": "Deploy={{.DeployText}}",
			}))

			_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{
				Channel: "jenkins-test", Status: "SUCCESS", JobName: "demo", BuildNumber: "12", BuildURL: "https://jenkins/job/demo/12/", Deploy: tt.deploy,
			})
			if err != nil {
				t.Fatalf("SendJenkinsBuild returned error: %v", err)
			}
			if cap.message.Text != tt.wantText {
				t.Fatalf("text = %q, want %q", cap.message.Text, tt.wantText)
			}
		})
	}
}

func TestSendJenkinsBuildRendersImageRefs(t *testing.T) {
	cap := &captureSender{}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_test"},
	}}, map[string]sender.Sender{"test": cap}, newMemoryRenderer(map[string]string{
		"jenkins_test": "{{range $i, $v := .ImageRefs}}{{if $i}},{{end}}{{$v}}{{end}}",
	}))

	_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{
		Channel: "jenkins-test", Status: "SUCCESS", JobName: "demo", BuildNumber: "12", BuildURL: "https://jenkins/job/demo/12/",
		ImageRefs: []string{"registry.example.com/demo/user-service:test-12", "registry.example.com/demo/order-service:test-12"},
	})
	if err != nil {
		t.Fatalf("SendJenkinsBuild returned error: %v", err)
	}
	if cap.message.Text != "registry.example.com/demo/user-service:test-12,registry.example.com/demo/order-service:test-12" {
		t.Fatalf("text = %q", cap.message.Text)
	}
}
func TestSendJenkinsBuildDingTalkTemplateUsesCompactFields(t *testing.T) {
	cap := &captureSender{}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_dingtalk"},
	}}, map[string]sender.Sender{"test": cap}, tmpl.NewFileRenderer("../templates"))

	_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{
		Channel:     "jenkins-test",
		Status:      "SUCCESS",
		JobName:     "demo-job",
		BuildNumber: "12",
		BuildURL:    "https://jenkins/job/demo/12/",
		Services:    []string{"demo-service"},
		Branch:      "release",
		Commit:      "a1b2c3d admin update service",
		DeployEnv:   "prod",
		ImageRefs:   []string{"registry.example.com/demo/demo-service:prod-12"},
		PackageType: "image",
		Deploy:      boolPtr(false),
		DeployType:  "none",
		TriggerUser: "admin",
	})
	if err != nil {
		t.Fatalf("SendJenkinsBuild returned error: %v", err)
	}
	for _, want := range []string{
		"- **结果**：SUCCESS",
		"- **环境**：prod",
		"- **服务**：demo-service",
		"- **分支**：release",
		"- **提交**：a1b2c3d admin update service",
		"- **镜像**：registry.example.com/demo/demo-service:prod-12",
		"- **部署**：否",
		"- **Job**：demo-job #12",
		"- **触发人**：admin",
	} {
		if !strings.Contains(cap.message.Text, want) {
			t.Fatalf("text missing %q:\n%s", want, cap.message.Text)
		}
	}
	for _, unwanted := range []string{"构建号", "产物类型", "部署方式", "镜像地址", "（none）"} {
		if strings.Contains(cap.message.Text, unwanted) {
			t.Fatalf("text contains unwanted %q:\n%s", unwanted, cap.message.Text)
		}
	}
}
func TestSendJenkinsBuildFeishuTemplateUsesCompactFields(t *testing.T) {
	cap := &captureSender{}
	svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_feishu"},
	}}, map[string]sender.Sender{"test": cap}, tmpl.NewFileRenderer("../templates"))

	_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{
		Channel:     "jenkins-test",
		Status:      "SUCCESS",
		JobName:     "demo-job",
		BuildNumber: "12",
		BuildURL:    "https://jenkins/job/demo/12/",
		Services:    []string{"demo-service"},
		Branch:      "main",
		Commit:      "c3d4e5f admin deploy front",
		DeployEnv:   "test",
		ImageRefs:   []string{"registry-test.example.com/demo/demo-service:test-12"},
		PackageType: "image",
		Deploy:      boolPtr(true),
		DeployType:  "ssh",
		TriggerUser: "admin",
	})
	if err != nil {
		t.Fatalf("SendJenkinsBuild returned error: %v", err)
	}
	for _, want := range []string{
		"- **结果**：SUCCESS",
		"- **环境**：test",
		"- **服务**：demo-service",
		"- **分支**：main",
		"- **提交**：c3d4e5f admin deploy front",
		"- **镜像**：registry-test.example.com/demo/demo-service:test-12",
		"- **部署**：是（ssh）",
		"- **Job**：demo-job #12",
		"- **触发人**：admin",
	} {
		if !strings.Contains(cap.message.Text, want) {
			t.Fatalf("text missing %q:\n%s", want, cap.message.Text)
		}
	}
	for _, unwanted := range []string{"构建号", "产物类型", "部署方式", "镜像地址"} {
		if strings.Contains(cap.message.Text, unwanted) {
			t.Fatalf("text contains unwanted %q:\n%s", unwanted, cap.message.Text)
		}
	}
}
func TestJenkinsTemplatesShowDeployTypeOnlyWhenDeployed(t *testing.T) {
	cases := []struct {
		name       string
		deploy     *bool
		deployType string
		want       string
	}{
		{name: "omitted", deployType: "ssh"},
		{name: "not deployed", deploy: boolPtr(false), deployType: "ssh", want: "- **部署**：否"},
		{name: "deployed", deploy: boolPtr(true), deployType: "ssh", want: "- **部署**：是（ssh）"},
		{name: "empty type", deploy: boolPtr(true), want: "- **部署**：是"},
		{name: "none type", deploy: boolPtr(true), deployType: "none", want: "- **部署**：是"},
	}
	for _, name := range []string{"jenkins_dingtalk", "jenkins_feishu"} {
		for _, tt := range cases {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				cap := &captureSender{}
				svc := newTestNotifyService(testNotifyConfig{Channels: map[string]types.ChannelConfig{
					"jenkins-test": {Platform: "test", Template: name},
				}}, map[string]sender.Sender{"test": cap}, tmpl.NewFileRenderer("../templates"))
				_, err := svc.SendJenkinsBuild(context.Background(), types.JenkinsNotifyReq{
					Channel: "jenkins-test", Status: "SUCCESS", JobName: "demo", BuildNumber: "12",
					BuildURL: "https://example.com/build/12", Deploy: tt.deploy, DeployType: tt.deployType,
				})
				if err != nil {
					t.Fatalf("SendJenkinsBuild returned error: %v", err)
				}
				var deployLine string
				for _, line := range strings.Split(cap.message.Text, "\n") {
					if strings.HasPrefix(line, "- **部署**：") {
						deployLine = line
					}
				}
				if deployLine != tt.want {
					t.Fatalf("deploy line = %q, want %q", deployLine, tt.want)
				}
			})
		}
	}
}

func boolPtr(value bool) *bool {
	return &value
}
