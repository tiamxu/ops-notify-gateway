package service

import (
	"github.com/tiamxu/ops-notify-gateway/types"
	"testing"
)

func TestValidatePlatformChannelAddresses(t *testing.T) {
	enabled := true
	for _, tt := range []struct {
		url   string
		valid bool
	}{
		{"https://oapi.dingtalk.com/robot/send?access_token=fixture", true},
		{"https://OAPI.DINGTALK.COM/robot/send?access_token=fixture", true},
		{"http://oapi.dingtalk.com/robot/send?access_token=fixture", false},
		{"https://oapi.dingtalk.com.evil.example/robot/send?access_token=fixture", false},
		{"https://user@oapi.dingtalk.com/robot/send?access_token=fixture", false},
		{"https://oapi.dingtalk.com:8443/robot/send?access_token=fixture", false},
		{"https://oapi.dingtalk.com:/robot/send?access_token=fixture", false},
		{"https://oapi.dingtalk.com/robot/send?access_token=fixture&other=value", false},
		{"https://oapi.dingtalk.com/other?access_token=fixture", false},
		{"https://oapi.dingtalk.com/robot/send", false},
	} {
		if err := validatePlatformChannel("ops", types.ChannelConfig{Platform: "dingtalk", Webhook: tt.url, Enabled: &enabled}); (err == nil) != tt.valid {
			t.Errorf("%s: %v", tt.url, err)
		}
	}
	if validateRecipients("feishu", []string{`ou_x><at id=all`}) == nil {
		t.Fatal("accepted mention injection")
	}
	if validateRecipients("dingtalk", []string{"not-a-number"}) == nil {
		t.Fatal("accepted invalid phone")
	}
	for _, url := range []string{"https://OPEN.FEISHU.CN/open-apis/bot/v2/hook/fixture", "https://open.feishu.cn/open-apis/bot/v2/hook/fixture?"} {
		if err := validatePlatformChannel("ops", types.ChannelConfig{Platform: "feishu", Webhook: url, Enabled: &enabled}); err != nil {
			t.Fatalf("platform-compatible URL rejected: %v", err)
		}
	}
}

func TestValidateFeishuOptionalSigning(t *testing.T) {
	enabled := true
	for _, secret := range []string{"", "fixture-secret"} {
		if err := validatePlatformChannel("ops", types.ChannelConfig{Platform: "feishu", Webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/fixture", Secret: secret, Enabled: &enabled}); err != nil {
			t.Fatalf("valid feishu configuration rejected: %v", err)
		}
	}
}
