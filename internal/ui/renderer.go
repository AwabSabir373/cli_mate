package ui

// Renderer wraps the custom markdown renderer to replace the glamour dependency.
// glamour v1.0.0 pulls in cellbuf v0.0.13 which conflicts with the newer
// ansi/cellbuf versions required by charm.land/bubbletea/v2. The custom
// markdownRenderer in assistant_markdown.go handles all rendering.
type Renderer struct {
	width  int
	styles appStyles
}

func NewRenderer(width int) (*Renderer, error) {
	if width < 20 {
		width = 20
	}
	return &Renderer{width: width}, nil
}

// SetStyles updates the styles used by the renderer. Called after theme changes.
func (r *Renderer) SetStyles(styles appStyles) {
	if r != nil {
		r.styles = styles
	}
}

func (r *Renderer) Render(markdown string) string {
	if r == nil {
		return markdown
	}
	renderer := newMarkdownRenderer(r.width, r.styles)
	return renderer.render(markdown)
}
