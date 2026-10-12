package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/tiamxu/kit/log"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/httpclient"
	"github.com/tiamxu/ops-notify-gateway/types"
)

const maxSnapshotBytes = 8 << 20

type PlatformChannelStore struct {
	cfg      types.PlatformSourceConfig
	client   *http.Client
	syncMu   sync.Mutex
	mu       sync.RWMutex
	channels map[string]types.ChannelConfig
	status   types.ChannelSourceStatus
	recovery *snapshotRecovery
}

func NewPlatformChannelStore(cfg types.PlatformSourceConfig, client *http.Client) (*PlatformChannelStore, error) {
	recovery, err := newSnapshotRecovery(cfg.RecoveryFile, cfg.URL)
	if err != nil {
		return nil, err
	}
	if cfg.TimeoutSeconds <= 0 || cfg.TimeoutSeconds > 300 || cfg.SyncIntervalSeconds <= 0 || cfg.SyncIntervalSeconds > 86400 {
		return nil, fmt.Errorf("invalid synchronization timing")
	}
	if client == nil {
		client = httpclient.New(time.Duration(cfg.TimeoutSeconds) * time.Second)
	}
	cloned := *client
	cloned.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	cloned.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &PlatformChannelStore{cfg: cfg, client: &cloned, recovery: recovery, status: types.ChannelSourceStatus{Mode: "platform", SyncStatus: "pending", PersistenceStatus: "pending"}}, nil
}

func (s *PlatformChannelStore) Get(_ context.Context, name string) (types.ChannelConfig, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.status.Ready {
		return types.ChannelConfig{}, false, e.New(e.CodeConfigUnavailable, "channel configuration unavailable")
	}
	ch, ok := s.channels[name]
	return cloneChannel(ch), ok, nil
}

func cloneChannel(ch types.ChannelConfig) types.ChannelConfig {
	ch.At = append([]string(nil), ch.At...)
	if ch.Enabled != nil {
		enabled := *ch.Enabled
		ch.Enabled = &enabled
	}
	return ch
}

func (s *PlatformChannelStore) Status() types.ChannelSourceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.status
	if status.LastSyncAt != nil {
		v := *status.LastSyncAt
		status.LastSyncAt = &v
	}
	if status.LastSavedAt != nil {
		v := *status.LastSavedAt
		status.LastSavedAt = &v
	}
	return status
}

func (s *PlatformChannelStore) Initialize(ctx context.Context) {
	if s.Sync(ctx) != nil {
		_ = s.LoadRecovery()
	}
}

func (s *PlatformChannelStore) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.SyncIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.Sync(ctx)
		}
	}
}

func (s *PlatformChannelStore) Sync(ctx context.Context) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	snapshot, err := s.fetch(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.mu.Lock()
		s.setResultLocked("sync", err.Error())
		s.mu.Unlock()
		return err
	}
	channels, err := validateSnapshot(snapshot)
	if err != nil {
		s.mu.Lock()
		s.setResultLocked("sync", err.Error())
		s.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.channels = channels
	s.status.Ready = true
	s.setResultLocked("sync", "")
	s.status.LastSyncAt = &now
	s.mu.Unlock()
	err = s.recovery.save(snapshot, now)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.setResultLocked("persistence", recoveryFailure("save recovery", err))
	} else {
		s.setResultLocked("persistence", "")
		s.status.LastSavedAt = &now
	}
	return nil
}

func (s *PlatformChannelStore) LoadRecovery() error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	snapshot, syncedAt, err := s.recovery.load()
	if err != nil {
		s.mu.Lock()
		s.setResultLocked("persistence", recoveryFailure("load recovery", err))
		s.mu.Unlock()
		return err
	}
	channels, err := validateSnapshot(snapshot)
	if err != nil {
		s.mu.Lock()
		s.setResultLocked("persistence", recoveryFailure("load recovery", err))
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels = channels
	s.status.Ready = true
	s.status.LastSyncAt = &syncedAt
	s.status.LastSavedAt = &syncedAt
	s.setResultLocked("persistence", "")
	return nil
}

func (s *PlatformChannelStore) fetch(ctx context.Context) (types.ChannelSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.URL, nil)
	if err != nil {
		return types.ChannelSnapshot{}, fmt.Errorf("invalid platform endpoint")
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return types.ChannelSnapshot{}, fmt.Errorf("platform request timed out")
		}
		return types.ChannelSnapshot{}, fmt.Errorf("platform request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return types.ChannelSnapshot{}, fmt.Errorf("platform returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes+1))
	if err != nil {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return types.ChannelSnapshot{}, fmt.Errorf("platform response timed out")
		}
		return types.ChannelSnapshot{}, fmt.Errorf("platform response incomplete")
	}
	if len(body) > maxSnapshotBytes {
		return types.ChannelSnapshot{}, fmt.Errorf("platform response too large")
	}
	var envelope struct {
		Code int                    `json:"code"`
		Data *types.ChannelSnapshot `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Code != 200 || envelope.Data == nil {
		return types.ChannelSnapshot{}, fmt.Errorf("invalid platform response")
	}
	return *envelope.Data, nil
}

func validateSnapshot(snapshot types.ChannelSnapshot) (map[string]types.ChannelConfig, error) {
	if snapshot.Version != 1 || snapshot.Channels == nil || len(snapshot.Channels) > 10000 {
		return nil, fmt.Errorf("invalid channel snapshot")
	}
	channels := make(map[string]types.ChannelConfig, len(snapshot.Channels))
	for i, item := range snapshot.Channels {
		ch := types.ChannelConfig{Platform: item.Platform, Webhook: item.Webhook, Secret: item.Secret, At: item.At, Enabled: item.Enabled}
		if err := validatePlatformChannel(item.Code, ch); err != nil {
			return nil, fmt.Errorf("invalid snapshot channel at index %d", i)
		}
		if _, exists := channels[item.Code]; exists {
			return nil, fmt.Errorf("duplicate channel at index %d", i)
		}
		channels[item.Code] = cloneChannel(ch)
	}
	return channels, nil
}

// Caller holds mu; diagnostic reasons must contain only safe summaries.
func (s *PlatformChannelStore) setResultLocked(stage, reason string) {
	status, detail := &s.status.SyncStatus, &s.status.SyncError
	if stage == "persistence" {
		status, detail = &s.status.PersistenceStatus, &s.status.PersistenceError
	}
	next := "ok"
	if reason != "" {
		next = "error"
	}
	if *status == next && *detail == reason {
		return
	}
	*status, *detail = next, reason
	if reason != "" {
		log.GetLogger().Warnf("channel source stage=%s status=%s reason=%s", stage, next, reason)
	} else {
		log.GetLogger().Infof("channel source stage=%s status=%s", stage, next)
	}
}

func recoveryFailure(action string, err error) string {
	reason := "invalid recovery file"
	switch {
	case errors.Is(err, os.ErrPermission):
		reason = "permission denied"
	case errors.Is(err, os.ErrNotExist):
		reason = "file not found"
	case errors.Is(err, syscall.ENOSPC):
		reason = "disk full"
	default:
		if action == "save recovery" {
			reason = "write failed"
		}
	}
	return action + ": " + reason
}
