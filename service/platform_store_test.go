package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiamxu/kit/log"
	"github.com/tiamxu/ops-notify-gateway/types"
	"gopkg.in/yaml.v3"
)

func TestPlatformStoreSyncRecoveryAndInvalidSnapshots(t *testing.T) {
	var mu sync.Mutex
	body := `{"code":200,"data":{"version":1,"channels":[{"code":"ops","platform":"dingtalk","webhook":"https://oapi.dingtalk.com/robot/send?access_token=fixture","secret":"fixture-secret","at":[],"enabled":true}]}}`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer read-fixture" {
			t.Error("missing read token")
			w.WriteHeader(401)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		w.Write([]byte(body))
	}))
	defer server.Close()
	cfg := types.PlatformSourceConfig{URL: server.URL, Token: "read-fixture", SyncIntervalSeconds: 30, TimeoutSeconds: 5, RecoveryFile: filepath.Join(t.TempDir(), "snapshot.yaml")}
	store, err := NewPlatformChannelStore(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Get(context.Background(), "ops"); err == nil {
		t.Fatal("uninitialized store must fail")
	}
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	ch, ok, err := store.Get(context.Background(), "ops")
	if err != nil || !ok || ch.Secret != "fixture-secret" {
		t.Fatalf("channel unavailable: %v", err)
	}
	data, err := os.ReadFile(cfg.RecoveryFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fixture-secret") {
		t.Fatal("recovery is not readable YAML")
	}
	var saved map[string]interface{}
	if err := yaml.Unmarshal(data, &saved); err != nil || saved["source"] != cfg.URL || saved["synced_at"] == nil || saved["snapshot"] == nil {
		t.Fatal("invalid YAML recovery metadata")
	}
	info, _ := os.Stat(cfg.RecoveryFile)
	if info.Mode().Perm() != 0600 {
		t.Fatal("unsafe file permissions")
	}
	restored, _ := NewPlatformChannelStore(cfg, server.Client())
	if err := restored.LoadRecovery(); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := restored.Get(context.Background(), "ops"); err != nil || !ok {
		t.Fatal("failed to recover")
	}
	changed := cfg
	changed.URL += "/other"
	wrongSource, _ := NewPlatformChannelStore(changed, server.Client())
	if wrongSource.LoadRecovery() == nil {
		t.Fatal("accepted wrong source")
	}
	for _, invalid := range []string{
		`{"code":200,"data":{"version":1}}`, `{"code":200,"data":{"version":1,"channels":null}}`,
		`{"code":500,"data":{"version":1,"channels":[]}}`, `{"code":200,"data":{"version":2,"channels":[]}}`,
		`{"code":200,"data":{"version":1,"channels":[{"code":"ops","platform":"feishu","webhook":"https://evil.example/hook","enabled":true}]}}`,
		`{"code":200,"data":{"version":1,"channels":[]}} trailing`,
	} {
		mu.Lock()
		body = invalid
		mu.Unlock()
		if store.Sync(context.Background()) == nil {
			t.Fatalf("accepted invalid snapshot %s", invalid)
		}
		if _, ok, err := store.Get(context.Background(), "ops"); err != nil || !ok {
			t.Fatal("invalid snapshot replaced old channels")
		}
	}
	mu.Lock()
	body = `{"code":200,"data":{"version":1,"channels":[{"code":"ops","platform":"dingtalk","webhook":"https://oapi.dingtalk.com/robot/send?access_token=fixture","enabled":false}]}}`
	mu.Unlock()
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	ch, _, _ = store.Get(context.Background(), "ops")
	if ch.Enabled == nil || *ch.Enabled {
		t.Fatal("disabled state not applied")
	}
	*ch.Enabled = true
	ch, _, _ = store.Get(context.Background(), "ops")
	if *ch.Enabled {
		t.Fatal("caller mutated shared snapshot")
	}
	mu.Lock()
	body = `{"code":200,"data":{"version":1,"channels":[]}}`
	mu.Unlock()
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Get(context.Background(), "ops"); err != nil || ok {
		t.Fatal("empty snapshot not applied")
	}
	if !store.Status().Ready {
		t.Fatal("empty snapshot must be ready")
	}
	encoded, _ := json.Marshal(store.Status())
	if strings.Contains(string(encoded), "fixture") {
		t.Fatal("status leaks config")
	}
	if err := restored.LoadRecovery(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := restored.Get(context.Background(), "ops"); ok {
		t.Fatal("empty snapshot not persisted")
	}
}

func TestPlatformStoreRejectsMissingEnabledAndDuplicateCodes(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"channels":[{"code":"ops","platform":"feishu","webhook":"https://open.feishu.cn/open-apis/bot/v2/hook/fixture"}]}`,
		`{"version":1,"channels":[{"code":"ops","platform":"feishu","webhook":"https://open.feishu.cn/open-apis/bot/v2/hook/fixture","enabled":true},{"code":"ops","platform":"feishu","webhook":"https://open.feishu.cn/open-apis/bot/v2/hook/fixture","enabled":false}]}`,
	} {
		var snapshot types.ChannelSnapshot
		if err := json.Unmarshal([]byte(body), &snapshot); err != nil {
			t.Fatal(err)
		}
		if _, err := validateSnapshot(snapshot); err == nil {
			t.Fatal("accepted incomplete or duplicate channel")
		}
	}
}

func TestPlatformStorePeriodicSyncConcurrentReadsAndOfflineRecovery(t *testing.T) {
	var unavailable atomic.Bool
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if unavailable.Load() {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[{"code":"ops","platform":"feishu","webhook":"https://open.feishu.cn/open-apis/bot/v2/hook/fixture","at":["ou_fixture"],"enabled":true}]}}`))
	}))
	defer server.Close()
	cfg := types.PlatformSourceConfig{URL: server.URL, Token: "fixture", TimeoutSeconds: 5, SyncIntervalSeconds: 1, RecoveryFile: filepath.Join(t.TempDir(), "recovery.yaml")}
	store, _ := NewPlatformChannelStore(cfg, server.Client())
	store.Initialize(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); store.Run(ctx) }()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				ch, ok, err := store.Get(ctx, "ops")
				if err != nil || !ok {
					t.Error("lost snapshot")
				}
				if len(ch.At) > 0 {
					ch.At[0] = "modified"
				}
				_ = store.Status()
			}
		}()
	}
	deadline := time.After(3 * time.Second)
	for requests.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("periodic sync did not run")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	<-done
	wg.Wait()
	ch, _, _ := store.Get(context.Background(), "ops")
	if ch.At[0] != "ou_fixture" {
		t.Fatal("recipient slice mutated")
	}
	unavailable.Store(true)
	recovered, _ := NewPlatformChannelStore(cfg, server.Client())
	recovered.Initialize(context.Background())
	if status := recovered.Status(); !status.Ready || status.SyncStatus != "error" {
		t.Fatalf("offline recovery: %+v", status)
	}
	if err := os.WriteFile(cfg.RecoveryFile, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	broken, _ := NewPlatformChannelStore(cfg, server.Client())
	broken.Initialize(context.Background())
	if broken.Status().Ready {
		t.Fatal("corrupt recovery must not be ready")
	}
}

func TestPlatformStoreSaveFailureKeepsNewMemory(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[]}}`))
	}))
	defer server.Close()
	cfg := types.PlatformSourceConfig{URL: server.URL, Token: "fixture", TimeoutSeconds: 5, SyncIntervalSeconds: 30, RecoveryFile: t.TempDir()}
	store, err := NewPlatformChannelStore(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := store.Status()
	if !status.Ready || status.PersistenceStatus != "error" || status.SyncStatus != "ok" {
		t.Fatalf("status: %+v", status)
	}
}

func TestPlatformStoreRejectsOversizedResponseAndRedirect(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(map[bool]string{false: "oversized", true: "redirect"}[redirect], func(t *testing.T) {
			var followed atomic.Bool
			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				followed.Store(true)
				w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[]}}`))
			}))
			defer target.Close()
			origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if redirect {
					http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
					return
				}
				w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[]},"padding":"` + strings.Repeat("x", maxSnapshotBytes) + `"}`))
			}))
			defer origin.Close()
			cfg := types.PlatformSourceConfig{URL: origin.URL, Token: "fixture", TimeoutSeconds: 5, SyncIntervalSeconds: 30, RecoveryFile: filepath.Join(t.TempDir(), "snapshot.yaml")}
			store, err := NewPlatformChannelStore(cfg, origin.Client())
			if err != nil {
				t.Fatal(err)
			}
			if store.Sync(context.Background()) == nil || store.Status().Ready {
				t.Fatal("invalid response accepted")
			}
			if followed.Load() {
				t.Fatal("followed redirect")
			}
			if _, err := os.Stat(cfg.RecoveryFile); !os.IsNotExist(err) {
				t.Fatal("invalid response persisted")
			}
		})
	}
}

func TestPlatformDiagnosticTransitions(t *testing.T) {
	var logs bytes.Buffer
	logger := log.GetLogger()
	original := logger.Out
	logger.SetOutput(&logs)
	defer logger.SetOutput(original)
	var status atomic.Int32
	status.Store(http.StatusUnauthorized)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := int(status.Load())
		w.WriteHeader(code)
		if code == http.StatusOK {
			_, _ = w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[]}}`))
		} else {
			_, _ = w.Write([]byte("private-fixture-response"))
		}
	}))
	defer server.Close()
	cfg := types.PlatformSourceConfig{URL: server.URL, Token: "private-fixture-token", TimeoutSeconds: 5, SyncIntervalSeconds: 30, RecoveryFile: filepath.Join(t.TempDir(), "snapshot.yaml")}
	store, err := NewPlatformChannelStore(cfg, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if store.Sync(context.Background()) == nil {
			t.Fatal("accepted unauthorized response")
		}
	}
	if strings.Count(logs.String(), "401") != 1 {
		t.Fatalf("expected one authentication failure log, got %q", logs.String())
	}
	encoded, _ := json.Marshal(store.Status())
	if !strings.Contains(string(encoded), "401") {
		t.Fatalf("missing diagnostic: %s", encoded)
	}
	status.Store(http.StatusOK)
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(store.Status())
	if strings.Contains(string(encoded), "401") {
		t.Fatalf("stale error retained: %s", encoded)
	}
	count := logs.Len()
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logs.Len() != count {
		t.Fatal("unchanged healthy state logged repeatedly")
	}
	for _, secret := range []string{cfg.Token, server.URL, "private-fixture-response"} {
		if strings.Contains(logs.String()+string(encoded), secret) {
			t.Fatal("diagnostics expose sensitive data")
		}
	}
	if !strings.Contains(logs.String(), "status=ok") {
		t.Fatal("recovery was not logged")
	}
}

func TestPlatformPersistenceDiagnosticRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"data":{"version":1,"channels":[]}}`))
	}))
	defer server.Close()
	dir := t.TempDir()
	store, err := NewPlatformChannelStore(types.PlatformSourceConfig{URL: server.URL, Token: "fixture", SyncIntervalSeconds: 30, TimeoutSeconds: 5, RecoveryFile: dir}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := store.Status()
	if !status.Ready || status.PersistenceError != "save recovery: write failed" {
		t.Fatalf("status: %+v", status)
	}
	store.recovery.path = filepath.Join(dir, "snapshot.yaml")
	if err := store.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := store.Status(); status.PersistenceError != "" || status.PersistenceStatus != "ok" {
		t.Fatalf("status: %+v", status)
	}
}

func TestPlatformSyncTimeoutDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	store, err := NewPlatformChannelStore(types.PlatformSourceConfig{URL: server.URL, Token: "fixture", SyncIntervalSeconds: 30, TimeoutSeconds: 1, RecoveryFile: filepath.Join(t.TempDir(), "snapshot.yaml")}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if store.Sync(context.Background()) == nil {
		t.Fatal("timeout accepted")
	}
	if got := store.Status().SyncError; got != "platform request timed out" {
		t.Fatalf("diagnostic: %q", got)
	}
}

func TestRecoveryDiagnosticsHideFilePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-fixture-path.yaml")
	store, err := NewPlatformChannelStore(types.PlatformSourceConfig{URL: "https://platform.example/channels", Token: "fixture", TimeoutSeconds: 5, SyncIntervalSeconds: 30, RecoveryFile: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if store.LoadRecovery() == nil {
		t.Fatal("accepted missing recovery")
	}
	if status := store.Status(); status.PersistenceError != "load recovery: file not found" || status.Ready {
		t.Fatalf("status: %+v", status)
	}
	if err := os.WriteFile(path, []byte("invalid snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	if store.LoadRecovery() == nil {
		t.Fatal("accepted invalid recovery")
	}
	encoded, _ := json.Marshal(store.Status())
	if strings.Contains(string(encoded), path) || strings.Contains(string(encoded), "private-fixture") {
		t.Fatal("status leaks recovery path")
	}
	if got := store.Status().PersistenceError; got != "load recovery: invalid recovery file" {
		t.Fatalf("diagnostic: %q", got)
	}
}
