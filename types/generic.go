package types

type GenericNotifyReq struct {
	Channel string   `json:"channel"`
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	Level   string   `json:"level"`
	Content string   `json:"content"`
	URL     string   `json:"url"`
	At      []string `json:"at"`
}
