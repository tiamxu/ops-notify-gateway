package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tiamxu/ops-notify-gateway/pkg/e"
	"github.com/tiamxu/ops-notify-gateway/pkg/sender/message"
)

func TestSenderSendsDingTalkMarkdownSuccessfully(t *testing.T) {
	var payload markdownPayload
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
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	sender := New(server.URL, "", server.Client())
	err := sender.Send(context.Background(), message.Message{Title: "demo", Text: "content", At: []string{"13800138000"}})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if payload.MsgType != "markdown" {
		t.Fatalf("msg type = %q, want markdown", payload.MsgType)
	}
	if payload.Markdown.Title != "demo" {
		t.Fatalf("title = %q, want demo", payload.Markdown.Title)
	}
	if payload.Markdown.Text != "content" {
		t.Fatalf("text = %q, want content", payload.Markdown.Text)
	}
	if len(payload.At.AtMobiles) != 1 || payload.At.AtMobiles[0] != "13800138000" {
		t.Fatalf("at mobiles = %#v", payload.At.AtMobiles)
	}
	if payload.At.IsAtAll {
		t.Fatal("isAtAll = true, want false")
	}
}

func TestSenderReturnsErrorWhenDingTalkBusinessCodeFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"keywords not in content"}`))
	}))
	defer server.Close()

	sender := New(server.URL, "", server.Client())
	err := sender.Send(context.Background(), message.Message{Title: "demo", Text: "content"})
	assertSendFailed(t, err, "dingtalk webhook returned error code 310000")
}

func TestSenderReturnsErrorWhenDingTalkHTTPStatusFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()

	sender := New(server.URL, "", server.Client())
	err := sender.Send(context.Background(), message.Message{Title: "demo", Text: "content"})
	assertSendFailed(t, err, "dingtalk webhook returned status 502")
}

func TestSenderValidatesSuccessResponse(t *testing.T) {
	success := `{"errcode":0}`
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
		{name: "null code", body: `{"errcode":null}`, wantError: true},
		{name: "null response", body: "null", wantError: true},
		{name: "string code", body: `{"errcode":"0"}`, wantError: true},
		{name: "nonzero code", body: `{"errcode":1}`, wantError: true},
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
