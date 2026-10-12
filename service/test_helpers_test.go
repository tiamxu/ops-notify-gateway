package service

import (
	"bytes"
	"context"
	"fmt"
	texttemplate "text/template"

	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/types"
)

type testNotifyConfig struct {
	Channels map[string]types.ChannelConfig
}

func newTestNotifyService(cfg testNotifyConfig, senders map[string]sender.Sender, renderer Renderer) *NotifyService {
	return NewNotifyServiceWithStore(NewStaticChannelStore(cfg.Channels), newStaticSenderFactory(senders), renderer)
}

type staticSenderFactory struct {
	senders map[string]sender.Sender
}

func newStaticSenderFactory(senders map[string]sender.Sender) *staticSenderFactory {
	return &staticSenderFactory{senders: senders}
}

func (f *staticSenderFactory) New(ctx context.Context, channel string, ch types.ChannelConfig) (sender.Sender, error) {
	snd, ok := f.senders[channel]
	if !ok {
		snd, ok = f.senders[ch.Platform]
	}
	if !ok || snd == nil {
		return nil, fmt.Errorf("sender not found for channel: %s", channel)
	}
	return snd, nil
}

type memoryRenderer struct {
	templates map[string]string
}

func newMemoryRenderer(templates map[string]string) *memoryRenderer {
	return &memoryRenderer{templates: templates}
}

func (r *memoryRenderer) Render(name string, data interface{}) (string, error) {
	text, ok := r.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}
	return renderInline(text, data)
}

func renderInline(text string, data interface{}) (string, error) {
	tmpl, err := texttemplate.New("inline").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
