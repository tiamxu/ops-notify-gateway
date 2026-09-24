package sender

import (
	"context"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender/message"
)

type Message = message.Message

type Sender interface {
	Send(ctx context.Context, message Message) error
	Platform() string
}

type Config struct {
	Platform string
	Webhook  string
	Secret   string
}
