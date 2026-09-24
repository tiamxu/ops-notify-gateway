package types

type NotifyResult struct {
	Channel  string `json:"channel"`
	Platform string `json:"platform"`
	Sent     bool   `json:"sent"`
}
