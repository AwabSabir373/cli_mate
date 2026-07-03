// Package sessiontitle auto-generates session titles from conversation content.
package sessiontitle

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTitleLength is the maximum length of a generated title.
const MaxTitleLength = 60

// Generate creates a concise title from the first user message or conversation.
// Falls back to a generic title if no meaningful content is found.
func Generate(messages []string) string {
	// Find the first non-empty user message
	var firstMessage string
	for _, msg := range messages {
		msg = strings.TrimSpace(msg)
		if msg != "" {
			firstMessage = msg
			break
		}
	}

	if firstMessage == "" {
		return "New session"
	}

	// Extract the first meaningful line or sentence
	title := extractTitle(firstMessage)
	if title == "" {
		return "New session"
	}

	// Truncate to max length with ellipsis
	title = truncate(title, MaxTitleLength)

	return title
}

// extractTitle pulls a concise title from the beginning of a message.
func extractTitle(text string) string {
	// Remove workspace context prefix if present
	if idx := strings.Index(text, "User Request:"); idx >= 0 {
		text = text[idx+len("User Request:"):]
	}
	if idx := strings.Index(text, "Act as a senior"); idx >= 0 {
		if lastNewline := strings.LastIndex(text[:idx], "\n"); lastNewline >= 0 {
			text = text[lastNewline+1:]
		}
	}

	text = strings.TrimSpace(text)

	// Take the first sentence (up to ., !, ? followed by space or end)
	sentenceEnd := -1
	for i, r := range text {
		if (r == '.' || r == '!' || r == '?') && (i+utf8.RuneLen(r) >= len(text) || isSpaceOrEnd(text[i+utf8.RuneLen(r):])) {
			sentenceEnd = i + utf8.RuneLen(r)
			break
		}
		if r == '\n' && i > 0 {
			sentenceEnd = i
			break
		}
	}

	if sentenceEnd > 0 {
		text = text[:sentenceEnd]
	}

	text = strings.TrimSpace(text)

	// Clean up common prefixes
	text = strings.TrimPrefix(text, "implement ")
	text = strings.TrimPrefix(text, "Implement ")
	text = strings.TrimPrefix(text, "add ")
	text = strings.TrimPrefix(text, "Add ")
	text = strings.TrimPrefix(text, "create ")
	text = strings.TrimPrefix(text, "Create ")
	text = strings.TrimPrefix(text, "fix ")
	text = strings.TrimPrefix(text, "Fix ")
	text = strings.TrimPrefix(text, "refactor ")
	text = strings.TrimPrefix(text, "Refactor ")
	text = strings.TrimPrefix(text, "update ")
	text = strings.TrimPrefix(text, "Update ")
	text = strings.TrimPrefix(text, "please ")
	text = strings.TrimPrefix(text, "Please ")

	text = strings.TrimSpace(text)

	// Capitalize first letter
	if len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = string(unicode.ToUpper(r)) + text[size:]
	}

	return text
}

// truncate shortens a string to maxLen, adding ellipsis if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Try to break at a word boundary
	truncated := s[:maxLen-1]
	if idx := strings.LastIndex(truncated, " "); idx > maxLen/2 {
		truncated = truncated[:idx]
	}

	return strings.TrimSpace(truncated) + "…"
}

func isSpaceOrEnd(s string) bool {
	if len(s) == 0 {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsSpace(r)
}
