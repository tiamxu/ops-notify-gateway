package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func (s *Sender) Platform() string { return "dingtalk" }

func (s *Sender) Send(ctx context.Context, message message.Message) error {
	endpoint, err := GenerateSignedURL(s.webhook, s.secret, time.Now())
	if err != nil {
		return e.SendFailed("generate dingtalk sign failed")
	}
	payload, err := json.Marshal(buildPayload(message))
	if err != nil {
		return e.SendFailed("marshal dingtalk payload failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return e.SendFailed("create dingtalk request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return e.SendFailed("send dingtalk request failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return e.SendFailed("dingtalk webhook returned status %d", resp.StatusCode)
	}
	var result struct {
		ErrCode int `json:"errcode"`
	}
	if len(body) > 0 && json.Unmarshal(body, &result) == nil && result.ErrCode != 0 {
		return e.SendFailed("dingtalk webhook returned error code %d", result.ErrCode)
	}
	return nil
}
