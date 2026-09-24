package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type Config struct {
	Channels map[string]types.ChannelConfig
}

type NotifyService struct {
	cfg      Config
	senders  map[string]sender.Sender
	renderer Renderer
}

func NewNotifyService(cfg Config, senders map[string]sender.Sender, renderer Renderer) *NotifyService {
	return &NotifyService{cfg: cfg, senders: senders, renderer: renderer}
}

func (s *NotifyService) channel(name string) (types.ChannelConfig, sender.Sender, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return types.ChannelConfig{}, nil, e.BadRequest("channel is required")
	}
	ch, ok := s.cfg.Channels[name]
	if !ok {
		return types.ChannelConfig{}, nil, e.BadRequest("channel not found: %s", name)
	}
	snd, ok := s.senders[name]
	if !ok {
		snd, ok = s.senders[ch.Platform]
	}
	if !ok || snd == nil {
		return types.ChannelConfig{}, nil, e.Internal("sender not found for channel: %s", name)
	}
	return ch, snd, nil
}

func (s *NotifyService) send(ctx context.Context, channel string, message sender.Message) (types.NotifyResult, error) {
	ch, snd, err := s.channel(channel)
	if err != nil {
		return types.NotifyResult{}, err
	}
	if err := snd.Send(ctx, message); err != nil {
		return types.NotifyResult{}, err
	}
	platform := ch.Platform
	if platform == "" {
		platform = snd.Platform()
	}
	return types.NotifyResult{Channel: channel, Platform: platform, Sent: true}, nil
}

func defaultTemplate(source, platform string) string {
	if platform == "" {
		return source
	}
	return fmt.Sprintf("%s_%s", source, platform)
}
