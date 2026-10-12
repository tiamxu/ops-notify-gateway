package dingtalk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"
)

func GenerateSignedURL(webhook, secret string, now time.Time) (string, error) {
	if secret == "" {
		return webhook, nil
	}
	timestamp := now.UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	if _, err := h.Write([]byte(stringToSign)); err != nil {
		return "", err
	}
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
	separator := "&"
	if parsed, err := url.Parse(webhook); err == nil && parsed.RawQuery == "" {
		separator = "?"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", webhook, separator, timestamp, sign), nil
}
