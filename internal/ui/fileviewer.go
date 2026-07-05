package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileViewer is a full-screen file viewer with syntax highlighting and diff gutter.
type FileViewer struct {
	visible    bool
	filePath   string
	content    []string
	lineOffset int
	maxOffset  int
	height     int
	width      int
	diffLines  map[int]string // line number -> "+" or "-"
}

// NewFileViewer creates a new file viewer.
func NewFileViewer() *FileViewer {
	return &FileViewer{
		diffLines: make(map[int]string),
	}
}

// Open opens a file for viewing. Returns an error if the file can't be read.
func (fv *FileViewer) Open(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	fv.filePath = path
	content := string(data)
	fv.content = strings.Split(content, "\n")
	fv.lineOffset = 0
	fv.diffLines = make(map[int]string)

	// Calculate max offset
	fv.maxOffset = len(fv.content) - fv.height + 3
	if fv.maxOffset < 0 {
		fv.maxOffset = 0
	}

	fv.visible = true
	return nil
}

// OpenWithDiff opens a file and highlights diff lines.
func (fv *FileViewer) OpenWithDiff(path string, diffLines map[int]string) error {
	if err := fv.Open(path); err != nil {
		return err
	}
	fv.diffLines = diffLines
	return nil
}

// Close closes the file viewer.
func (fv *FileViewer) Close() {
	fv.visible = false
	fv.filePath = ""
	fv.content = nil
	fv.diffLines = make(map[int]string)
}

// IsVisible returns whether the file viewer is visible.
func (fv *FileViewer) IsVisible() bool {
	return fv.visible
}

// ScrollUp scrolls up by one line.
func (fv *FileViewer) ScrollUp() {
	if fv.lineOffset > 0 {
		fv.lineOffset--
	}
}

// ScrollDown scrolls down by one line.
func (fv *FileViewer) ScrollDown() {
	if fv.lineOffset < fv.maxOffset {
		fv.lineOffset++
	}
}

// ScrollPageUp scrolls up by one page.
func (fv *FileViewer) ScrollPageUp() {
	fv.lineOffset -= fv.height - 3
	if fv.lineOffset < 0 {
		fv.lineOffset = 0
	}
}

// ScrollPageDown scrolls down by one page.
func (fv *FileViewer) ScrollPageDown() {
	fv.lineOffset += fv.height - 3
	if fv.lineOffset > fv.maxOffset {
		fv.lineOffset = fv.maxOffset
	}
}

// SetSize updates the viewer dimensions.
func (fv *FileViewer) SetSize(width, height int) {
	fv.width = width
	fv.height = height
	fv.maxOffset = len(fv.content) - height + 3
	if fv.maxOffset < 0 {
		fv.maxOffset = 0
	}
}

// Render produces the file viewer display.
func (fv *FileViewer) Render(styles appStyles) string {
	if !fv.visible || len(fv.content) == 0 {
		return ""
	}

	var b strings.Builder

	// Header bar
	fileName := filepath.Base(fv.filePath)
	fileDir := filepath.Dir(fv.filePath)
	header := fmt.Sprintf(" %s — %s  (%d lines)", styles.accent.Render(fileName), styles.muted.Render(fileDir), len(fv.content))
	b.WriteString(styles.pill.Render(header))
	b.WriteString("\n\n")

	// Content lines with gutter
	visibleLines := fv.height - 4
	if visibleLines > len(fv.content)-fv.lineOffset {
		visibleLines = len(fv.content) - fv.lineOffset
	}
	if visibleLines <= 0 {
		visibleLines = 0
	}

	lineNumWidth := 4
	if len(fv.content) >= 1000 {
		lineNumWidth = 5
	}

	for i := 0; i < visibleLines; i++ {
		lineNum := fv.lineOffset + i + 1
		content := fv.content[lineNum-1]

		// Line number with gutter
		lineNumStr := fmt.Sprintf("%*d", lineNumWidth, lineNum)
		gutter := " "
		if diffType, ok := fv.diffLines[lineNum]; ok {
			switch diffType {
			case "+":
				gutter = styles.badgeAdd.Render("+")
			case "-":
				gutter = styles.badgeDel.Render("-")
			}
		}

		// Highlight the line using chroma lexer
		lexer := cachedLexerForPath(fv.filePath)
		lang := ""
		if lexer != nil {
			lang = lexer.Config().Name
		}
		highlighted := highlightCode(content, lang)

		lineNumStyled := styles.muted.Render(lineNumStr)
		b.WriteString(fmt.Sprintf("%s%s %s\n", gutter, lineNumStyled, highlighted))
	}

	// Scroll indicator
	if fv.maxOffset > 0 {
		scrollPercent := float64(fv.lineOffset) / float64(fv.maxOffset) * 100
		scrollHint := styles.muted.Render(fmt.Sprintf("── %.0f%% ── Press Esc to close, ↑/↓ to scroll ──", scrollPercent))
		b.WriteString(scrollHint)
	}

	return styles.panel.Width(fv.width - 4).Render(b.String())
}
