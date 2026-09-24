package security

import "net/url"

func MaskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "***"
	}
	q := u.Query()
	for _, key := range []string{"access_token", "token", "secret", "sign"} {
		if q.Has(key) {
			q.Set(key, "***")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
