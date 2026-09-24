package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/httpclient"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender/message"
)

type Sender struct {
	webhook string
	secret  string
	client  *http.Client
}

func New(webhook, secret string, client *http.Client) *Sender {
	if client == nil {
		client = httpclient.New(10 * time.Second)
	}
	return &Sender{webhook: webhook, secret: secret, client: client}
}

func (s *Sender) Platform() string { return "feishu" }

func (s *Sender) Send(ctx context.Context, message message.Message) error {
	body, err := json.Marshal(buildPayload(message))
	if err != nil {
		return e.SendFailed("marshal feishu payload failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhook, bytes.NewReader(body))
	if err != nil {
		return e.SendFailed("create feishu request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return e.SendFailed("send feishu request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return e.SendFailed("feishu webhook returned status %d", resp.StatusCode)
	}
	return nil
}
