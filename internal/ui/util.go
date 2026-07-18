package ui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func fallback(value, fallbackValue string) string {
	if value == "" {
		return fallbackValue
	}
	return value
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func keyText(key string) (string, bool) {
	if key == "space" {
		return " ", true
	}
	if len(key) == 1 {
		return key, true
	}
	return "", false
}

func ansiSequenceEnd(value string, start int) int {
	if start >= len(value) || value[start] != '\x1b' {
		return start
	}
	index := start + 1
	if index >= len(value) {
		return index
	}

	switch value[index] {
	case '[':
		for index++; index < len(value); index++ {
			if value[index] >= 0x40 && value[index] <= 0x7e {
				return index + 1
			}
		}
		return len(value)
	case ']':
		for index++; index < len(value); index++ {
			if value[index] == '\a' {
				return index + 1
			}
			if value[index] == '\x1b' && index+1 < len(value) && value[index+1] == '\\' {
				return index + 2
			}
		}
		return len(value)
	default:
		return min(start+2, len(value))
	}
}

func fitStyledLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	return truncateStyledLine(line, width)
}

func truncateStyledLine(line string, width int) string {
	const resetANSI = "\x1b[0m"

	ellipsis := "…"
	ellipsisWidth := lipgloss.Width(ellipsis)
	if width <= ellipsisWidth {
		return ellipsis
	}

	targetWidth := width - ellipsisWidth
	usedWidth := 0
	sawANSI := false
	openLink := false

	var builder strings.Builder
	for index := 0; index < len(line); {
		if line[index] == '\x1b' {
			end := ansiSequenceEnd(line, index)
			if end > index {
				sequence := line[index:end]
				builder.WriteString(sequence)
				sawANSI = true
				if strings.HasPrefix(sequence, "\x1b]8;") {
					openLink = sequence != "\x1b]8;;\x1b\\" && sequence != "\x1b]8;;\a"
				}
				index = end
				continue
			}
		}

		glyph, size := utf8.DecodeRuneInString(line[index:])
		if glyph == utf8.RuneError && size == 0 {
			break
		}

		glyphWidth := lipgloss.Width(string(glyph))
		if usedWidth+glyphWidth > targetWidth {
			break
		}
		builder.WriteString(line[index : index+size])
		usedWidth += glyphWidth
		index += size
	}

	if openLink {
		builder.WriteString("\x1b]8;;\x1b\\")
	}
	builder.WriteString(ellipsis)
	if sawANSI {
		builder.WriteString(resetANSI)
	}
	return builder.String()
}
