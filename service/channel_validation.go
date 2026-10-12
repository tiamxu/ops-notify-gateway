package service

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/types"
)

var codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
var phonePattern = regexp.MustCompile(`^[0-9]{5,20}$`)
var userPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func validatePlatformChannel(code string, ch types.ChannelConfig) error {
	if len(code) > 100 || !codePattern.MatchString(code) {
		return fmt.Errorf("invalid channel code")
	}
	if ch.Enabled == nil {
		return fmt.Errorf("channel enabled is required")
	}
	if len(ch.Webhook) > 4096 || len(ch.Secret) > 4096 || strings.Contains(ch.Webhook, "\\") {
		return fmt.Errorf("channel configuration too large")
	}
	u, err := url.Parse(ch.Webhook)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.RawPath != "" || strings.HasSuffix(u.Host, ":") || (u.Port() != "" && u.Port() != "443") {
		return fmt.Errorf("invalid webhook URL")
	}
	switch ch.Platform {
	case "dingtalk":
		if !strings.EqualFold(u.Hostname(), "oapi.dingtalk.com") || u.Path != "/robot/send" || len(u.Query()) != 1 || strings.TrimSpace(u.Query().Get("access_token")) == "" {
			return fmt.Errorf("invalid dingtalk webhook")
		}
	case "feishu":
		const prefix = "/open-apis/bot/v2/hook/"
		suffix := strings.TrimPrefix(u.Path, prefix)
		if !strings.EqualFold(u.Hostname(), "open.feishu.cn") || !strings.HasPrefix(u.Path, prefix) || suffix == "" || strings.Contains(suffix, "/") || u.RawQuery != "" {
			return fmt.Errorf("invalid feishu webhook")
		}
	default:
		return fmt.Errorf("unsupported channel platform")
	}
	return validateRecipients(ch.Platform, ch.At)
}

func validateRecipients(platform string, at []string) error {
	if len(at) > 100 {
		return fmt.Errorf("too many recipients")
	}
	for _, value := range at {
		if platform == "dingtalk" && !phonePattern.MatchString(value) || platform == "feishu" && !userPattern.MatchString(value) {
			return fmt.Errorf("invalid recipient format")
		}
	}
	return nil
}
