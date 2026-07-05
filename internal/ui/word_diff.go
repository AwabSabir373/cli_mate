package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// changedSpan finds the first differing position and the end of differences
// between two rune slices. Returns (start, aEnd, bEnd).
func changedSpan(a, b []rune) (int, int, int) {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	// Find first difference from the start
	start := 0
	for start < minLen && a[start] == b[start] {
		start++
	}

	// Find end of differences from the back
	aEnd := len(a)
	bEnd := len(b)
	for aEnd > start && bEnd > start && a[aEnd-1] == b[bEnd-1] {
		aEnd--
		bEnd--
	}

	return start, aEnd, bEnd
}

// diffWord represents a word-level diff segment.
type diffWord struct {
	text string
	kind string // "same", "added", "removed"
}

// wordDiff computes word-level differences between two strings.
func wordDiff(oldText, newText string) []diffWord {
	if oldText == newText {
		return []diffWord{{text: newText, kind: "same"}}
	}

	a := []rune(oldText)
	b := []rune(newText)

	start, aEnd, bEnd := changedSpan(a, b)

	var result []diffWord

	// Text before changes (same)
	if start > 0 {
		result = append(result, diffWord{text: string(a[:start]), kind: "same"})
	}

	// Deleted text
	if aEnd > start {
		result = append(result, diffWord{text: string(a[start:aEnd]), kind: "removed"})
	}

	// Added text
	if bEnd > start {
		result = append(result, diffWord{text: string(b[start:bEnd]), kind: "added"})
	}

	// Text after changes (same)
	if aEnd < len(a) {
		result = append(result, diffWord{text: string(a[aEnd:]), kind: "same"})
	}

	return result
}

// renderWordDiff renders a line with word-level diff highlighting.
func renderWordDiff(oldLine, newLine string, styles appStyles) string {
	words := wordDiff(oldLine, newLine)
	var b strings.Builder

	for _, w := range words {
		switch w.kind {
		case "added":
			b.WriteString(styles.diffAdd.Render(w.text))
		case "removed":
			b.WriteString(styles.diffRemove.Render(w.text))
		default:
			b.WriteString(w.text)
		}
	}

	return b.String()
}

// diffContext holds the previous line for computing word-level diffs.
type diffContext struct {
	prevLine string
}

// newDiffContext creates a new diff context.
func newDiffContext() *diffContext {
	return &diffContext{}
}

// renderDiffLine renders a single diff line with word-level highlighting.
func (dc *diffContext) renderDiffLine(line string, lineType string, styles appStyles) string {
	switch lineType {
	case "+":
		if dc.prevLine != "" {
			return "+ " + renderWordDiff(dc.prevLine, line, styles)
		}
		return styles.diffAdd.Render("+ " + line)
	case "-":
		dc.prevLine = line
		return styles.diffRemove.Render("- " + line)
	default:
		dc.prevLine = ""
		return "  " + line
	}
}

// computeWordDiffs processes a diff hunk and produces lines with inline word-level changes.
func (dc *diffContext) computeWordDiffs(lines []string) []string {
	var result []string
	dc.prevLine = ""

	for _, line := range lines {
		if strings.HasPrefix(line, "-") {
			dc.prevLine = strings.TrimPrefix(line, "-")
			result = append(result, line)
		} else if strings.HasPrefix(line, "+") {
			content := strings.TrimPrefix(line, "+")
			if dc.prevLine != "" {
				words := wordDiff(dc.prevLine, content)
				var b strings.Builder
				b.WriteString("+ ")
				for _, w := range words {
					switch w.kind {
					case "added":
						b.WriteString("{")
						b.WriteString(w.text)
						b.WriteString("}")
					case "removed":
						b.WriteString("[-")
						b.WriteString(w.text)
						b.WriteString("-]")
					default:
						b.WriteString(w.text)
					}
				}
				result = append(result, b.String())
				dc.prevLine = ""
			} else {
				result = append(result, line)
			}
		} else {
			dc.prevLine = ""
			result = append(result, line)
		}
	}

	return result
}

// renderWordDiffLine renders a single line with word-level diff highlighting
// using the new diff format markers.
func renderWordDiffLine(line string, styles appStyles) string {
	if !strings.Contains(line, "{-") && !strings.Contains(line, "{+") {
		return line
	}

	var b strings.Builder
	i := 0
	runes := []rune(line)
	for i < len(runes) {
		if i+2 < len(runes) && string(runes[i:i+2]) == "[-" {
			j := i + 2
			for j < len(runes) && (j+1 >= len(runes) || string(runes[j:j+2]) != "-]") {
				j++
			}
			if j+2 <= len(runes) {
				b.WriteString(styles.diffRemove.Render(string(runes[i+2 : j])))
				i = j + 2
			} else {
				b.WriteRune(runes[i])
				i++
			}
		} else if i+2 < len(runes) && string(runes[i:i+2]) == "{+" {
			j := i + 2
			for j < len(runes) && (j+1 >= len(runes) || string(runes[j:j+2]) != "+}") {
				j++
			}
			if j+2 <= len(runes) {
				b.WriteString(styles.diffAdd.Render(string(runes[i+2 : j])))
				i = j + 2
			} else {
				b.WriteRune(runes[i])
				i++
			}
		} else {
			b.WriteRune(runes[i])
			i++
		}
	}

	return b.String()
}

// computeWordDiffForLines computes word-level diff markers for a set of diff lines.
// Takes alternating -/+ lines and produces marked-up output.
func computeWordDiffForLines(lines []string) []string {
	var dc diffContext
	return dc.computeWordDiffs(lines)
}

// wordDiffToken represents a styled token in a word diff.
type wordDiffToken struct {
	text  string
	style lipgloss.Style
}

// highlightWordDiff applies word-level highlighting to a diff line and returns styled tokens.
func highlightWordDiff(line string, styles appStyles) []wordDiffToken {
	if !strings.Contains(line, "[-") && !strings.Contains(line, "{+") {
		return []wordDiffToken{{text: line, style: lipgloss.NewStyle()}}
	}

	var tokens []wordDiffToken
	runes := []rune(line)
	i := 0
	currentText := strings.Builder{}

	flushText := func() {
		if currentText.Len() > 0 {
			tokens = append(tokens, wordDiffToken{
				text:  currentText.String(),
				style: lipgloss.NewStyle(),
			})
			currentText.Reset()
		}
	}

	for i < len(runes) {
		if i+2 < len(runes) && string(runes[i:i+2]) == "[-" {
			flushText()
			j := i + 2
			for j < len(runes) && (j+1 >= len(runes) || string(runes[j:j+2]) != "-]") {
				currentText.WriteRune(runes[j])
				j++
			}
			if j+2 <= len(runes) {
				tokens = append(tokens, wordDiffToken{
					text:  currentText.String(),
					style: styles.diffRemove,
				})
				currentText.Reset()
				i = j + 2
			} else {
				currentText.WriteRune(runes[i])
				i++
			}
		} else if i+2 < len(runes) && string(runes[i:i+2]) == "{+" {
			flushText()
			j := i + 2
			for j < len(runes) && (j+1 >= len(runes) || string(runes[j:j+2]) != "+}") {
				currentText.WriteRune(runes[j])
				j++
			}
			if j+2 <= len(runes) {
				tokens = append(tokens, wordDiffToken{
					text:  currentText.String(),
					style: styles.diffAdd,
				})
				currentText.Reset()
				i = j + 2
			} else {
				currentText.WriteRune(runes[i])
				i++
			}
		} else {
			currentText.WriteRune(runes[i])
			i++
		}
	}
	flushText()

	return tokens
}
