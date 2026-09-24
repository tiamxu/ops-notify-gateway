package security

import (
	"crypto/subtle"
	"strings"
)

func TokenMatches(authorization, webhookToken, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	incoming := strings.TrimSpace(webhookToken)
	if incoming == "" {
		authorization = strings.TrimSpace(authorization)
		if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			incoming = strings.TrimSpace(authorization[7:])
		}
	}
	if incoming == "" || len(incoming) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(incoming), []byte(expected)) == 1
}
