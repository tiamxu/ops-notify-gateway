package sender

import (
	"fmt"
	"net/http"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender/dingtalk"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender/feishu"
)

func New(cfg Config, client *http.Client) (Sender, error) {
	switch cfg.Platform {
	case "dingtalk":
		return dingtalk.New(cfg.Webhook, cfg.Secret, client), nil
	case "feishu":
		return feishu.New(cfg.Webhook, cfg.Secret, client), nil
	default:
		return nil, fmt.Errorf("unsupported sender platform: %s", cfg.Platform)
	}
}
