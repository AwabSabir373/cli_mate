package ui

import (
	"strings"
	"unicode"
)



// markdownRenderer renders markdown text to terminal-formatted output.
type markdownRenderer struct {
	width   int
	styles  appStyles
}

// newMarkdownRenderer creates a new markdown renderer.
func newMarkdownRenderer(width int, styles appStyles) *markdownRenderer {
	return &markdownRenderer{
		width:   width,
		styles:  styles,
	}
}

// render converts markdown text to styled terminal output.
func (mr *markdownRenderer) render(text string) string {
	if text == "" {
		return ""
	}

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	var result []string
	var inCodeBlock bool
	var codeBlockLang string
	var codeBlockLines []string

	for _, line := range lines {
		// Check for code block start/end
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block — render it
				rendered := mr.renderCodeBlock(codeBlockLines, codeBlockLang)
				result = append(result, rendered)
				codeBlockLines = nil
				codeBlockLang = ""
				inCodeBlock = false
			} else {
				// Start code block
				inCodeBlock = true
				codeBlockLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			}
			continue
		}

		if inCodeBlock {
			codeBlockLines = append(codeBlockLines, line)
			continue
		}

		// Regular markdown line
		rendered := mr.renderInline(line)
		result = append(result, rendered)
	}

	// If we're still in a code block at end, render it
	if inCodeBlock && len(codeBlockLines) > 0 {
		rendered := mr.renderCodeBlock(codeBlockLines, codeBlockLang)
		result = append(result, rendered)
	}

	// Add breathing room between sections
	result = mr.addBreathingRoom(result)

	return strings.Join(result, "\n")
}

// renderInline renders inline markdown elements.
func (mr *markdownRenderer) renderInline(line string) string {
	if line == "" {
		return ""
	}

	// Handle headings
	if strings.HasPrefix(line, "### ") {
		return mr.styles.accent.Render(strings.TrimPrefix(line, "### "))
	}
	if strings.HasPrefix(line, "## ") {
		return mr.styles.title.Render(strings.TrimPrefix(line, "## "))
	}
	if strings.HasPrefix(line, "# ") {
		return mr.styles.logo.Render(strings.TrimPrefix(line, "# "))
	}

	// Handle horizontal rules
	if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "***") || strings.HasPrefix(line, "___") {
		return mr.styles.divider.Render(strings.Repeat("─", mr.width))
	}

	// Handle list items
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return "  • " + mr.parseInlineMarkup(line[2:])
	}
	if strings.HasPrefix(line, "  - ") || strings.HasPrefix(line, "  * ") {
		return "    • " + mr.parseInlineMarkup(line[4:])
	}

	// Handle numbered lists
	if len(line) > 2 && unicode.IsDigit(rune(line[0])) && line[1] == '.' && line[2] == ' ' {
		return "  " + mr.parseInlineMarkup(line)
	}

	// Handle blockquotes
	if strings.HasPrefix(line, "> ") {
		return mr.styles.muted.Render("▎ " + mr.parseInlineMarkup(line[2:]))
	}

	return mr.parseInlineMarkup(line)
}

// parseInlineMarkup parses bold, inline code, and links in a line.
func (mr *markdownRenderer) parseInlineMarkup(line string) string {
	runes := []rune(line)
	var b strings.Builder
	i := 0

	for i < len(runes) {
		// Inline code with backticks
		if runes[i] == '`' {
			j := i + 1
			for j < len(runes) && runes[j] != '`' {
				j++
			}
			if j < len(runes) {
				code := string(runes[i+1 : j])
				b.WriteString(mr.styles.codePanel.Render(code))
				i = j + 1
				continue
			}
		}

		// Bold with **
		if i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '*' {
			j := i + 2
			for j+1 < len(runes) && !(runes[j] == '*' && runes[j+1] == '*') {
				j++
			}
			if j+1 < len(runes) {
				boldText := string(runes[i+2 : j])
				b.WriteString(mr.styles.accent.Render(boldText))
				i = j + 2
				continue
			}
		}

		// Bold with __
		if i+1 < len(runes) && runes[i] == '_' && runes[i+1] == '_' {
			j := i + 2
			for j+1 < len(runes) && !(runes[j] == '_' && runes[j+1] == '_') {
				j++
			}
			if j+1 < len(runes) {
				boldText := string(runes[i+2 : j])
				b.WriteString(mr.styles.accent.Render(boldText))
				i = j + 2
				continue
			}
		}

		// Italic with *
		if runes[i] == '*' {
			j := i + 1
			for j < len(runes) && runes[j] != '*' {
				j++
			}
			if j < len(runes) && j > i+1 {
				italicText := string(runes[i+1 : j])
				b.WriteString(mr.styles.muted.Render(italicText))
				i = j + 1
				continue
			}
		}

		b.WriteRune(runes[i])
		i++
	}

	return b.String()
}

// renderCodeBlock renders a code block with syntax highlighting.
func (mr *markdownRenderer) renderCodeBlock(lines []string, lang string) string {
	if len(lines) == 0 {
		return ""
	}

	code := strings.Join(lines, "\n")

	// Try syntax highlighting
	highlighted := highlightCode(code, lang)

	// Wrap in a panel
	panelWidth := mr.width - 4
	if panelWidth < 20 {
		panelWidth = 20
	}

	return mr.styles.softPanel.
		Width(panelWidth).
		Render(highlighted)
}

// addBreathingRoom adds vertical spacing between sections.
func (mr *markdownRenderer) addBreathingRoom(lines []string) []string {
	if len(lines) <= 1 {
		return lines
	}

	var result []string
	for i, line := range lines {
		result = append(result, line)
		// Add empty line after headings, code blocks, lists, and horizontal rules
		if i < len(lines)-1 {
			nextLine := lines[i+1]
			if shouldAddSpace(line, nextLine) {
				result = append(result, "")
			}
		}
	}

	return result
}

// shouldAddSpace determines if a blank line should be inserted between two lines.
func shouldAddSpace(prev, next string) bool {
	// After headings
	if strings.HasPrefix(prev, "#") {
		return true
	}
	// After code blocks (rendered code blocks start with styled text, detect via panel)
	if strings.HasPrefix(prev, "┃") || strings.Contains(prev, "```") {
		return true
	}
	// Between list items and non-list items
	if (strings.HasPrefix(prev, "  •") || strings.HasPrefix(prev, "    •")) && !strings.HasPrefix(next, "  ") {
		return true
	}
	// After horizontal rules
	if strings.HasPrefix(prev, "─") {
		return true
	}
	return false
}

// renderMarkdown is the main entry point for markdown rendering.
func renderMarkdown(text string, width int, styles appStyles) string {
	renderer := newMarkdownRenderer(width, styles)
	return renderer.render(text)
}
