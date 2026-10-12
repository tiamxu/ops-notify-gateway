package service

type Renderer interface {
	Render(name string, data interface{}) (string, error)
}
