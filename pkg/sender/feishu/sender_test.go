package feishu

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender/message"
)

func TestSenderOptionalSigning(t *testing.T) {
	for _, tc := range []struct {
		name      string
		secret    string
		response  string
		wantError bool
	}{
		{name: "unsigned", response: `{"code":0}`},
		{name: "signed", secret: "fixture-secret", response: `{"code":0}`},
		{name: "signature rejected without retry", secret: "fixture-secret", response: `{"code":19021,"msg":"signature rejected"}`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var payload map[string]json.RawMessage
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			before := time.Now().Unix()
			err := New(server.URL, tc.secret, server.Client()).Send(context.Background(), message.Message{Title: "demo", Text: "content"})
			after := time.Now().Unix()
			if tc.wantError {
				assertSendFailed(t, err, "19021")
			} else if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("requests = %d, want 1", calls)
			}
			if tc.secret == "" {
				if _, ok := payload["timestamp"]; ok {
					t.Error("unsigned request contains timestamp")
				}
				if _, ok := payload["sign"]; ok {
					t.Error("unsigned request contains sign")
				}
				return
			}
			var timestamp, signature string
			if err := json.Unmarshal(payload["timestamp"], &timestamp); err != nil {
				t.Fatalf("timestamp: %v", err)
			}
			if err := json.Unmarshal(payload["sign"], &signature); err != nil {
				t.Fatalf("sign: %v", err)
			}
			seconds, err := strconv.ParseInt(timestamp, 10, 64)
			if err != nil || seconds < before || seconds > after {
				t.Fatalf("timestamp is not current Unix seconds: %q", timestamp)
			}
			mac := hmac.New(sha256.New, []byte(timestamp+"\n"+tc.secret))
			if signature != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
				t.Fatal("invalid signature")
			}
		})
	}
}

func TestSenderSendsFeishuCardSuccessfully(t *testing.T) {
	var payload cardPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	sender := New(server.URL, "", server.Client())
	err := sender.Send(context.Background(), message.Message{Title: "demo", Text: "content", Level: "警告", Status: "故障", Color: "green", At: []string{"ou_user"}})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if payload.MsgType != "interactive" {
		t.Fatalf("msg type = %q, want interactive", payload.MsgType)
	}
	if payload.Card.Header.Title.Content != "demo" {
		t.Fatalf("title = %q, want demo", payload.Card.Header.Title.Content)
	}
	if payload.Card.Header.Template != "green" {
		t.Fatalf("template = %q, want green", payload.Card.Header.Template)
	}
	if len(payload.Card.Header.TextTagList) != 2 {
		t.Fatalf("header tags = %#v, want 2 tags", payload.Card.Header.TextTagList)
	}
	if payload.Card.Header.TextTagList[0].Text.Content != "警告" || payload.Card.Header.TextTagList[1].Text.Content != "故障" {
		t.Fatalf("header tags = %#v", payload.Card.Header.TextTagList)
	}
	if payload.Card.Header.TextTagList[0].Color != "green" || payload.Card.Header.TextTagList[1].Color != "green" {
		t.Fatalf("header tag colors = %#v", payload.Card.Header.TextTagList)
	}
	if len(payload.Card.Body.Elements) != 1 || !strings.Contains(payload.Card.Body.Elements[0].Content, "content") {
		t.Fatalf("body elements = %#v", payload.Card.Body.Elements)
	}
	if !strings.Contains(payload.Card.Body.Elements[0].Content, "<at id=ou_user></at>") {
		t.Fatalf("body content = %q, want at user", payload.Card.Body.Elements[0].Content)
	}
}

func TestBuildPayloadOmitsEmptyHeaderTags(t *testing.T) {
	payload := buildPayload(message.Message{Title: "demo", Text: "content"})
	if len(payload.Card.Header.TextTagList) != 0 {
		t.Fatalf("header tags = %#v, want empty", payload.Card.Header.TextTagList)
	}
}

func TestSenderReturnsErrorWhenFeishuBusinessCodeFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":9499,"msg":"bad request"}`))
	}))
	defer server.Close()

	sender := New(server.URL, "", server.Client())
	err := sender.Send(context.Background(), message.Message{Title: "demo", Text: "content"})
	assertSendFailed(t, err, "feishu webhook returned error code 9499")
}

func TestSenderReturnsErrorWhenFeishuHTTPStatusFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	sender := New(server.URL, "", server.Client())
	err := sender.Send(context.Background(), message.Message{Title: "demo", Text: "content"})
	assertSendFailed(t, err, "feishu webhook returned status 500")
}

func TestSenderValidatesSuccessResponse(t *testing.T) {
	success := `{"code":0}`
	cases := []struct {
		name       string
		body       string
		incomplete bool
		wantError  bool
	}{
		{name: "explicit zero", body: success},
		{name: "at size limit", body: success + strings.Repeat(" ", 4096-len(success))},
		{name: "empty", wantError: true},
		{name: "invalid json", body: "invalid-response-marker", wantError: true},
		{name: "missing code", body: `{"msg":"ok"}`, wantError: true},
		{name: "null code", body: `{"code":null}`, wantError: true},
		{name: "null response", body: "null", wantError: true},
		{name: "string code", body: `{"code":"0"}`, wantError: true},
		{name: "nonzero code", body: `{"code":1}`, wantError: true},
		{name: "trailing json", body: success + "{}", wantError: true},
		{name: "over size limit", body: success + strings.Repeat(" ", 4097-len(success)), wantError: true},
		{name: "incomplete body", body: success, incomplete: true, wantError: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.incomplete {
					w.Header().Set("Content-Length", "100")
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			snd := New(server.URL, "", server.Client())
			err := snd.Send(context.Background(), message.Message{Title: "demo", Text: "content"})
			if !tt.wantError {
				if err != nil {
					t.Fatalf("Send returned error: %v", err)
				}
				return
			}
			assertSendFailed(t, err, "")
			if strings.Contains(err.Error(), "invalid-response-marker") {
				t.Fatalf("error exposes response body: %v", err)
			}
		})
	}
}

func assertSendFailed(t *testing.T, err error, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected send failed error")
	}
	var biz *e.Error
	if !errors.As(err, &biz) {
		t.Fatalf("error type = %T, want *e.Error", err)
	}
	if biz.Code != e.CodeSendFailed {
		t.Fatalf("code = %d, want %d", biz.Code, e.CodeSendFailed)
	}
	if !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error = %q, want contains %q", err.Error(), wantMessage)
	}
}
