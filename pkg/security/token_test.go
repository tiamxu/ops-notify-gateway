package security

import "testing"

func TestTokenMatchesBearerAndHeaderToken(t *testing.T) {
	if !TokenMatches("Bearer abc123", "", "abc123") {
		t.Fatal("expected bearer token to match")
	}
	if !TokenMatches("", "abc123", "abc123") {
		t.Fatal("expected X-Webhook-Token to match")
	}
}

func TestTokenDoesNotMatchEmptyOrWrongToken(t *testing.T) {
	if TokenMatches("Bearer wrong", "", "abc123") {
		t.Fatal("wrong token matched")
	}
	if TokenMatches("", "", "abc123") {
		t.Fatal("empty incoming token matched")
	}
	if TokenMatches("Bearer abc123", "", "") {
		t.Fatal("empty expected token matched")
	}
}

func TestMaskURLHidesSensitiveQuery(t *testing.T) {
	got := MaskURL("https://oapi.dingtalk.com/robot/send?access_token=abcdef&timestamp=1")
	want := "https://oapi.dingtalk.com/robot/send?access_token=%2A%2A%2A&timestamp=1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
