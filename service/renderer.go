package service

import "fmt"

type Renderer interface {
	Render(name string, data interface{}) (string, error)
}

type MemoryRenderer struct {
	templates map[string]string
}

func NewMemoryRenderer(templates map[string]string) *MemoryRenderer {
	return &MemoryRenderer{templates: templates}
}

func (r *MemoryRenderer) Render(name string, data interface{}) (string, error) {
	text, ok := r.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}
	return renderInline(text, data)
}
