package routes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tiamxu/ops-notify-gateway/middleware"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiamxu/kit/log"
	"github.com/tiamxu/ops-notify-gateway/api"
	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender"
	"github.com/tiamxu/ops-notify-gateway/service"
	"github.com/tiamxu/ops-notify-gateway/types"
)

func init() {
	_ = log.InitLogger(&log.Config{Level: "info", Type: "stdout", Format: "json"})
}

type routeTestSender struct{}

func (s routeTestSender) Send(ctx context.Context, message sender.Message) error {
	return nil
}

func (s routeTestSender) Platform() string { return "test" }

func TestRegisterPingReturnsSuccessResponse(t *testing.T) {
	router := newTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body types.Response
	decodeResponse(t, rec, &body)
	if body.Code != http.StatusOK || body.Message != "操作成功" {
		t.Fatalf("response = %+v", body)
	}
	data, ok := body.Data.(map[string]interface{})
	if !ok || data["message"] != "pong" {
		t.Fatalf("data = %#v, want pong message", body.Data)
	}
}

func TestPlatformWithoutSnapshotIsAliveButNotReady(t *testing.T) {
	store, err := service.NewPlatformChannelStore(types.PlatformSourceConfig{URL: "https://platform.example/channels", Token: "fixture", TimeoutSeconds: 5, SyncIntervalSeconds: 30, RecoveryFile: filepath.Join(t.TempDir(), "recovery.yaml")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewNotifyServiceWithStore(store, nil, nil)
	router := gin.New()
	Register(router, api.NewHandler(svc), types.AuthConfig{Enabled: true, Token: "secret"})
	for _, tt := range []struct {
		path   string
		status int
	}{{"/ping", 200}, {"/ready", 503}} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", tt.path, nil))
		if rec.Code != tt.status {
			t.Fatalf("%s: %d", tt.path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedJSONRequest("POST", "/api/v1/notifications/generic", `{"channel":"ops","title":"test","content":"test"}`))
	if rec.Code != 503 {
		t.Fatalf("notification without snapshot: %d", rec.Code)
	}
}

func TestRegisterProtectedRouteRejectsMissingToken(t *testing.T) {
	router := newTestRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/generic", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body types.Response
	decodeResponse(t, rec, &body)
	if body.Code != e.CodeUnauthorized {
		t.Fatalf("code = %d, want %d", body.Code, e.CodeUnauthorized)
	}
}

func TestRegisterJenkinsRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter()

	rec := httptest.NewRecorder()
	req := authedJSONRequest(http.MethodPost, "/api/v1/notifications/jenkins", `{`)
	router.ServeHTTP(rec, req)

	assertBadRequest(t, rec, "invalid json body")
}

func TestRegisterGenericRejectsInvalidJSON(t *testing.T) {
	router := newTestRouter()

	rec := httptest.NewRecorder()
	req := authedJSONRequest(http.MethodPost, "/api/v1/notifications/generic", `{`)
	router.ServeHTTP(rec, req)

	assertBadRequest(t, rec, "invalid json body")
}

func TestRegisterJenkinsReturnsSuccessResponse(t *testing.T) {
	router := newTestRouter()
	payload := `{"channel":"jenkins-test","status":"SUCCESS","jobName":"demo","buildNumber":"12","buildUrl":"https://jenkins/job/demo/12/"}`

	rec := httptest.NewRecorder()
	req := authedJSONRequest(http.MethodPost, "/api/v1/notifications/jenkins", payload)
	router.ServeHTTP(rec, req)

	assertNotifySuccess(t, rec, "jenkins-test")
}

func TestRegisterGenericReturnsSuccessResponse(t *testing.T) {
	router := newTestRouter()
	payload := `{"channel":"ops-test","title":"demo","content":"ok","status":"SUCCESS"}`

	rec := httptest.NewRecorder()
	req := authedJSONRequest(http.MethodPost, "/api/v1/notifications/generic", payload)
	router.ServeHTTP(rec, req)

	assertNotifySuccess(t, rec, "ops-test")
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	notify := service.NewNotifyServiceWithStore(service.NewStaticChannelStore(map[string]types.ChannelConfig{
		"jenkins-test": {Platform: "test", Template: "jenkins_test"},
		"ops-test":     {Platform: "test", Template: "generic_test"},
	}), routeSenderFactory{}, routeRenderer{})
	Register(router, api.NewHandler(notify), types.AuthConfig{Enabled: true, Token: "secret"})
	return router
}

func authedJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	return req
}

func assertBadRequest(t *testing.T, rec *httptest.ResponseRecorder, wantMessage string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body types.Response
	decodeResponse(t, rec, &body)
	if body.Code != e.CodeBadRequest || body.Message != wantMessage {
		t.Fatalf("response = %+v", body)
	}
}

func assertNotifySuccess(t *testing.T, rec *httptest.ResponseRecorder, wantChannel string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body types.Response
	decodeResponse(t, rec, &body)
	if body.Code != http.StatusOK || body.Message != "操作成功" {
		t.Fatalf("response = %+v", body)
	}
	data, ok := body.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data = %#v, want object", body.Data)
	}
	if data["channel"] != wantChannel || data["platform"] != "test" || data["sent"] != true {
		t.Fatalf("data = %#v", data)
	}
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, rec.Body.String())
	}
}

func TestNotificationDeadlineReturnsJSONAndStopsSplit(t *testing.T) {
	var sends atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
		}
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer remote.Close()
	svc := service.NewNotifyServiceWithStore(service.NewStaticChannelStore(map[string]types.ChannelConfig{"ops": {Platform: "feishu", Webhook: remote.URL}}), service.NewConfigSenderFactory(remote.Client()), routeRenderer{})
	router := gin.New()
	Register(router, api.NewHandler(svc), types.AuthConfig{Enabled: true, Token: "secret"})
	server := httptest.NewUnstartedServer(middleware.RequestDeadline(router, 100*time.Millisecond, time.Second))
	server.Config.WriteTimeout = time.Second
	server.Start()
	defer server.Close()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/alerts/alertmanager?channel=ops&split=true", bytes.NewBufferString(`{"alerts":[{"status":"firing"},{"status":"firing"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("expected a JSON timeout response, got %v", err)
	}
	defer response.Body.Close()
	var body types.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 504 || body.Code != 50401 {
		t.Fatalf("HTTP %d body=%+v", response.StatusCode, body)
	}
	if sends.Load() != 1 {
		t.Fatalf("started %d requests after timeout", sends.Load())
	}
}

func TestNotificationDeadlineInterruptsSlowBody(t *testing.T) {
	server := httptest.NewUnstartedServer(middleware.RequestDeadline(newTestRouter(), 100*time.Millisecond, time.Second))
	server.Config.WriteTimeout = time.Second
	server.Start()
	defer server.Close()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = fmt.Fprintf(conn, "POST /api/v1/notifications/generic HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer secret\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("expected structured timeout, got %v", err)
	}
	defer response.Body.Close()
	var body types.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 504 || body.Code != 50401 {
		t.Fatalf("HTTP %d body=%+v", response.StatusCode, body)
	}
}

func TestNotificationDeadlineAllowsFastNotification(t *testing.T) {
	handler := middleware.RequestDeadline(newTestRouter(), time.Second, time.Second)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedJSONRequest("POST", "/api/v1/notifications/generic", `{"channel":"ops-test","title":"demo","content":"ok"}`))
	assertNotifySuccess(t, rec, "ops-test")
}

type routeSenderFactory struct{}

func (routeSenderFactory) New(context.Context, string, types.ChannelConfig) (sender.Sender, error) {
	return routeTestSender{}, nil
}

type routeRenderer struct{}

func (routeRenderer) Render(string, interface{}) (string, error) {
	return "test notification", nil
}
