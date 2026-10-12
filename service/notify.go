package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tiamxu/kit/log"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type NotifyService struct {
	store    ChannelStore
	factory  SenderFactory
	renderer Renderer
}

func NewNotifyServiceWithStore(store ChannelStore, factory SenderFactory, renderer Renderer) *NotifyService {
	return &NotifyService{store: store, factory: factory, renderer: renderer}
}

func (s *NotifyService) channelConfig(ctx context.Context, name string) (string, types.ChannelConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", types.ChannelConfig{}, e.BadRequest("channel is required")
	}
	ch, ok, err := s.store.Get(ctx, name)
	if err != nil {
		return "", types.ChannelConfig{}, err
	}
	if !ok {
		return "", types.ChannelConfig{}, e.BadRequest("channel not found: %s", name)
	}
	if ch.Enabled != nil && !*ch.Enabled {
		return "", types.ChannelConfig{}, e.BadRequest("channel disabled: %s", name)
	}
	return name, ch, nil
}

func (s *NotifyService) send(ctx context.Context, channel string, ch types.ChannelConfig, message sender.Message) (types.NotifyResult, error) {
	if err := ctx.Err(); err != nil {
		return types.NotifyResult{}, err
	}
	if err := validateRecipients(ch.Platform, message.At); err != nil {
		return types.NotifyResult{}, e.BadRequest("invalid recipients")
	}
	snd, err := s.factory.New(ctx, channel, ch)
	if err != nil || snd == nil {
		log.Errorf("create sender failed channel=%s platform=%s status=%s error=%v", channel, ch.Platform, message.Status, err)
		return types.NotifyResult{}, e.Internal("sender not found for channel: %s", channel)
	}
	if err := snd.Send(ctx, message); err != nil {
		log.Errorf("send notification failed channel=%s platform=%s status=%s error=%v", channel, ch.Platform, message.Status, err)
		return types.NotifyResult{}, err
	}
	platform := ch.Platform
	if platform == "" {
		platform = snd.Platform()
	}
	return types.NotifyResult{Channel: channel, Platform: platform, Sent: true}, nil
}

func (s *NotifyService) ChannelSourceStatus() types.ChannelSourceStatus {
	if provider, ok := s.store.(interface {
		Status() types.ChannelSourceStatus
	}); ok {
		return provider.Status()
	}
	return types.ChannelSourceStatus{Ready: true, Mode: "file"}
}

func defaultTemplate(source, platform string) string {
	if platform == "" {
		return source
	}
	return fmt.Sprintf("%s_%s", source, platform)
}
