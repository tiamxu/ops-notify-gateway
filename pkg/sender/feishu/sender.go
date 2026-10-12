package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
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
	payload := buildPayload(message)
	if s.secret != "" {
		timestamp := time.Now().Unix()
		payload.Timestamp = strconv.FormatInt(timestamp, 10)
		payload.Sign = GenerateSign(timestamp, s.secret)
	}
	body, err := json.Marshal(payload)
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
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return e.SendFailed("feishu webhook returned status %d", resp.StatusCode)
	}
	if err != nil {
		return e.SendFailed("read feishu response failed")
	}
	if len(respBody) > 4096 {
		return e.SendFailed("feishu webhook response too large")
	}
	var result struct {
		Code *int `json:"code"`
	}
	if json.Unmarshal(respBody, &result) != nil || result.Code == nil {
		return e.SendFailed("invalid feishu webhook response")
	}
	if *result.Code != 0 {
		return e.SendFailed("feishu webhook returned error code %d", *result.Code)
	}
	return nil
}
