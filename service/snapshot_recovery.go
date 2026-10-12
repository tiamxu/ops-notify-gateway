package service

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tiamxu/ops-notify-gateway/types"
	"gopkg.in/yaml.v3"
)

const maxRecoveryBytes = 32 << 20

type snapshotRecovery struct {
	path, source string
}
type savedSnapshot struct {
	Version  int                   `yaml:"version"`
	Source   string                `yaml:"source"`
	SyncedAt time.Time             `yaml:"synced_at"`
	Snapshot types.ChannelSnapshot `yaml:"snapshot"`
}

func newSnapshotRecovery(path, source string) (*snapshotRecovery, error) {
	if path == "" || source == "" {
		return nil, fmt.Errorf("invalid recovery configuration")
	}
	return &snapshotRecovery{path: path, source: source}, nil
}

func (r *snapshotRecovery) save(snapshot types.ChannelSnapshot, at time.Time) error {
	data, err := yaml.Marshal(savedSnapshot{Version: 1, Source: r.source, SyncedAt: at, Snapshot: snapshot})
	if err != nil {
		return err
	}
	if len(data) > maxRecoveryBytes {
		return fmt.Errorf("recovery snapshot too large")
	}
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".channels-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), r.path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (r *snapshotRecovery) load() (types.ChannelSnapshot, time.Time, error) {
	fail := func() (types.ChannelSnapshot, time.Time, error) {
		return types.ChannelSnapshot{}, time.Time{}, fmt.Errorf("invalid recovery file")
	}
	info, err := os.Lstat(r.path)
	if err != nil {
		return types.ChannelSnapshot{}, time.Time{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fail()
	}
	f, err := os.Open(r.path)
	if err != nil {
		return types.ChannelSnapshot{}, time.Time{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxRecoveryBytes+1))
	if err != nil || len(data) > maxRecoveryBytes {
		return fail()
	}
	var saved savedSnapshot
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if decoder.Decode(&saved) != nil || saved.Version != 1 || saved.Source != r.source || saved.SyncedAt.IsZero() {
		return fail()
	}
	var trailing interface{}
	if decoder.Decode(&trailing) != io.EOF {
		return fail()
	}
	return saved.Snapshot, saved.SyncedAt, nil
}
