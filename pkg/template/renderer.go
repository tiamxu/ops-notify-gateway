package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	texttemplate "text/template"
)

type Renderer interface {
	Render(name string, data interface{}) (string, error)
}

type FileRenderer struct {
	dir string
}

func NewFileRenderer(dir string) *FileRenderer {
	return &FileRenderer{dir: dir}
}

func (r *FileRenderer) Render(name string, data interface{}) (string, error) {
	if name == "" {
		return "", fmt.Errorf("template name is required")
	}
	content, err := os.ReadFile(filepath.Join(r.dir, name+".tmpl"))
	if err != nil {
		return "", err
	}
	tmpl, err := texttemplate.New(name).Funcs(FuncMap()).Parse(string(content))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
