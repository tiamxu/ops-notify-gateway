package service

import (
	"bytes"
	texttemplate "text/template"
)

func renderInline(text string, data interface{}) (string, error) {
	tmpl, err := texttemplate.New("inline").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
