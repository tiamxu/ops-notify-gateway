package service

import (
	"context"
	"net/http"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type SenderFactory interface {
	New(ctx context.Context, channel string, ch types.ChannelConfig) (sender.Sender, error)
}

type ConfigSenderFactory struct {
	client *http.Client
}

func NewConfigSenderFactory(client *http.Client) *ConfigSenderFactory {
	return &ConfigSenderFactory{client: client}
}

func (f *ConfigSenderFactory) New(ctx context.Context, channel string, ch types.ChannelConfig) (sender.Sender, error) {
	return sender.New(sender.Config{Platform: ch.Platform, Webhook: ch.Webhook, Secret: ch.Secret}, f.client)
}
