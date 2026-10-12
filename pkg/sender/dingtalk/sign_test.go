package dingtalk

import (
	"testing"
	"time"
)

func TestGenerateSignedURLUsesTimestampAndSecret(t *testing.T) {
	got, err := GenerateSignedURL("https://oapi.dingtalk.com/robot/send?access_token=token", "SEC000", time.UnixMilli(1700000000000))
	if err != nil {
		t.Fatalf("GenerateSignedURL returned error: %v", err)
	}

	want := "https://oapi.dingtalk.com/robot/send?access_token=token&timestamp=1700000000000&sign=ltBBey5eZrWKh1cPzFIdz3v3xpkc4Tjx4lLsPSHqdtA%3D"
	if got != want {
		t.Fatalf("signed URL mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestGenerateSignedURLReturnsWebhookWhenSecretEmpty(t *testing.T) {
	got, err := GenerateSignedURL("https://example.com/hook", "", time.UnixMilli(1700000000000))
	if err != nil {
		t.Fatalf("GenerateSignedURL returned error: %v", err)
	}
	if got != "https://example.com/hook" {
		t.Fatalf("got %q, want original webhook", got)
	}
}
