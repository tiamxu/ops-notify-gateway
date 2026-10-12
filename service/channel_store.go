package service

import (
	"context"

	"github.com/tiamxu/ops-notify-gateway/types"
)

type ChannelStore interface {
	Get(ctx context.Context, name string) (types.ChannelConfig, bool, error)
}

type StaticChannelStore struct {
	channels map[string]types.ChannelConfig
}

func NewStaticChannelStore(channels map[string]types.ChannelConfig) *StaticChannelStore {
	return &StaticChannelStore{channels: channels}
}

func (s *StaticChannelStore) Get(ctx context.Context, name string) (types.ChannelConfig, bool, error) {
	ch, ok := s.channels[name]
	return ch, ok, nil
}
