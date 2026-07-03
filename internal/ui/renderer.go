package ui

import "github.com/charmbracelet/glamour"

type Renderer struct {
	renderer *glamour.TermRenderer
}

func NewRenderer(width int) (*Renderer, error) {
	if width < 20 {
		width = 20
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}
	return &Renderer{renderer: renderer}, nil
}

func (r *Renderer) Render(markdown string) string {
	if r == nil || r.renderer == nil {
		return markdown
	}
	out, err := r.renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return out
}
