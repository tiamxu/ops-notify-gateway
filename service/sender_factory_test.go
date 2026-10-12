package service

import (
	"context"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type mutableChannelStore struct {
	channels map[string]types.ChannelConfig
}

func (s *mutableChannelStore) Get(ctx context.Context, name string) (types.ChannelConfig, bool, error) {
	ch, ok := s.channels[name]
	return ch, ok, nil
}

type switchingChannelStore struct {
	calls int
	first types.ChannelConfig
	later types.ChannelConfig
}

func (s *switchingChannelStore) Get(ctx context.Context, name string) (types.ChannelConfig, bool, error) {
	s.calls++
	if s.calls == 1 {
		return s.first, true, nil
	}
	return s.later, true, nil
}

type captureSenderFactory struct {
	webhooks []string
	snd      sender.Sender
}

func (f *captureSenderFactory) New(ctx context.Context, channel string, ch types.ChannelConfig) (sender.Sender, error) {
	f.webhooks = append(f.webhooks, ch.Webhook)
	return f.snd, nil
}

func TestNotifyServiceCreatesSenderFromCurrentChannelConfig(t *testing.T) {
	store := &mutableChannelStore{channels: map[string]types.ChannelConfig{
		"ops-test": {Platform: "test", Webhook: "https://example.com/first", Template: "generic_test"},
	}}
	factory := &captureSenderFactory{snd: &captureSender{}}
	svc := NewNotifyServiceWithStore(store, factory, newMemoryRenderer(map[string]string{
		"generic_test": "{{.Content}}",
	}))

	req := types.GenericNotifyReq{Channel: "ops-test", Title: "demo", Content: "ok"}
	if _, err := svc.SendGeneric(context.Background(), req); err != nil {
		t.Fatalf("first SendGeneric returned error: %v", err)
	}

	store.channels["ops-test"] = types.ChannelConfig{Platform: "test", Webhook: "https://example.com/second", Template: "generic_test"}
	if _, err := svc.SendGeneric(context.Background(), req); err != nil {
		t.Fatalf("second SendGeneric returned error: %v", err)
	}

	if len(factory.webhooks) != 2 {
		t.Fatalf("factory calls = %d, want 2", len(factory.webhooks))
	}
	if factory.webhooks[0] != "https://example.com/first" || factory.webhooks[1] != "https://example.com/second" {
		t.Fatalf("webhooks = %#v", factory.webhooks)
	}
}

func TestSendGenericUsesSameChannelConfigForTemplateAndSender(t *testing.T) {
	store := &switchingChannelStore{
		first: types.ChannelConfig{Platform: "test", Webhook: "https://example.com/first", Template: "generic_first"},
		later: types.ChannelConfig{Platform: "test", Webhook: "https://example.com/second", Template: "generic_second"},
	}
	factory := &captureSenderFactory{snd: &captureSender{}}
	svc := NewNotifyServiceWithStore(store, factory, newMemoryRenderer(map[string]string{
		"generic_first":  "first={{.Content}}",
		"generic_second": "second={{.Content}}",
	}))

	_, err := svc.SendGeneric(context.Background(), types.GenericNotifyReq{Channel: "ops-test", Title: "demo", Content: "ok"})
	if err != nil {
		t.Fatalf("SendGeneric returned error: %v", err)
	}

	cap := factory.snd.(*captureSender)
	if cap.message.Text != "first=ok" {
		t.Fatalf("message text = %q, want first=ok", cap.message.Text)
	}
	if len(factory.webhooks) != 1 || factory.webhooks[0] != "https://example.com/first" {
		t.Fatalf("factory webhooks = %#v", factory.webhooks)
	}
	if store.calls != 1 {
		t.Fatalf("channel store calls = %d, want 1", store.calls)
	}
}
