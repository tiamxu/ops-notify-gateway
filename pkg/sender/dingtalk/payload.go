package dingtalk

import "github.com/tiamxu/ops-notify-gateway/pkg/sender/message"

type markdownPayload struct {
	MsgType  string          `json:"msgtype"`
	Markdown markdownContent `json:"markdown"`
	At       atContent       `json:"at"`
}

type markdownContent struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type atContent struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	IsAtAll   bool     `json:"isAtAll"`
}

func buildPayload(message message.Message) markdownPayload {
	return markdownPayload{
		MsgType:  "markdown",
		Markdown: markdownContent{Title: message.Title, Text: message.Text},
		At:       atContent{AtMobiles: message.At, IsAtAll: false},
	}
}
