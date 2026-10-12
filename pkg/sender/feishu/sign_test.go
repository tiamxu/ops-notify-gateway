package feishu

import "testing"

func TestGenerateSign(t *testing.T) {
	if got := GenerateSign(1700000000, "fixture-secret"); got != "eSSBpPPdWfBl7avPl9BWSBfQOnLGZ91Xn8G5oZis0sc=" {
		t.Fatalf("unexpected signature: %s", got)
	}
	if got := GenerateSign(1700000000, ""); got != "" {
		t.Fatal("empty secret must not produce a signature")
	}
}
