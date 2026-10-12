package feishu

import "github.com/tiamxu/ops-notify-gateway/pkg/sender/message"

type cardPayload struct {
	Timestamp string `json:"timestamp,omitempty"`
	Sign      string `json:"sign,omitempty"`
	MsgType   string `json:"msg_type"`
	Card      card   `json:"card"`
}

type card struct {
	Schema string `json:"schema"`
	Header header `json:"header"`
	Body   body   `json:"body"`
}

type header struct {
	Title       textTag         `json:"title"`
	Template    string          `json:"template"`
	TextTagList []headerTextTag `json:"text_tag_list,omitempty"`
}

type headerTextTag struct {
	Tag   string  `json:"tag"`
	Text  textTag `json:"text"`
	Color string  `json:"color,omitempty"`
}

type textTag struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type body struct {
	Elements []element `json:"elements"`
}

type element struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

func buildPayload(message message.Message) cardPayload {
	return cardPayload{
		MsgType: "interactive",
		Card: card{
			Schema: "2.0",
			Header: header{
				Title:       textTag{Tag: "plain_text", Content: message.Title},
				Template:    colorTemplate(message.Color),
				TextTagList: buildHeaderTags(message),
			},
			Body: body{Elements: []element{{Tag: "markdown", Content: withAt(message.Text, message.At)}}},
		},
	}
}

func buildHeaderTags(message message.Message) []headerTextTag {
	tags := make([]headerTextTag, 0, 2)
	if message.Level != "" {
		tags = append(tags, newHeaderTextTag(message.Level, message.Color))
	}
	if message.Status != "" {
		tags = append(tags, newHeaderTextTag(message.Status, message.Color))
	}
	return tags
}

func newHeaderTextTag(content string, color string) headerTextTag {
	return headerTextTag{Tag: "text_tag", Text: textTag{Tag: "plain_text", Content: content}, Color: colorTemplate(color)}
}

func colorTemplate(color string) string {
	switch color {
	case "green":
		return "green"
	case "orange":
		return "orange"
	case "red":
		return "red"
	default:
		return "blue"
	}
}

func withAt(text string, at []string) string {
	for _, id := range at {
		if id != "" {
			text += "<at id=" + id + "></at>"
		}
	}
	return text
}
