package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const streamingTailLines = 6

// streamingToolCall represents an in-progress tool call being streamed.
type streamingToolCall struct {
	name      string
	path      string
	content   string
	args      string
	completed bool
}

// isFileWritingTool checks if the tool writes to files.
func isFileWritingTool(name string) bool {
	switch name {
	case "write_file", "edit_file", "apply_patch", "file_write", "file_edit", "str_replace":
		return true
	}
	return false
}

// decodeStreamingJSONString attempts to extract a JSON string value from a buffer.
// Handles incomplete/unterminated strings from streaming.
func decodeStreamingJSONString(buf []byte, key string) string {
	// Try full JSON decode first
	var data map[string]interface{}
	if err := json.Unmarshal(buf, &data); err == nil {
		if v, ok := data[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}

	// Fallback: manual search for the key in the raw buffer
	searchKey := fmt.Sprintf(`"%s"`, key)
	idx := strings.Index(string(buf), searchKey)
	if idx < 0 {
		return ""
	}

	// Find the value after the key
	afterKey := string(buf[idx+len(searchKey):])
	colonIdx := strings.Index(afterKey, ":")
	if colonIdx < 0 {
		return ""
	}
	afterColon := strings.TrimSpace(afterKey[colonIdx+1:])

	if !strings.HasPrefix(afterColon, `"`) {
		return ""
	}

	// Extract until closing quote or end of buffer
	var val strings.Builder
	escaped := false
	for _, r := range afterColon[1:] {
		if escaped {
			switch r {
			case 'n':
				val.WriteByte('\n')
			case 't':
				val.WriteByte('\t')
			case '"':
				val.WriteByte('"')
			case '\\':
				val.WriteByte('\\')
			default:
				val.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			break
		}
		val.WriteRune(r)
	}
	return val.String()
}

// streamingFilePath extracts the file path from streaming tool arguments.
func streamingFilePath(args string) string {
	keys := []string{"path", "file", "file_path", "filepath", "filename"}
	buf := []byte(args)
	for _, key := range keys {
		if path := decodeStreamingJSONString(buf, key); path != "" {
			return path
		}
	}
	return ""
}

// streamingToolCallView renders the in-progress tool call UI.
func streamingToolCallView(tc *streamingToolCall, styles appStyles, width int) string {
	if tc == nil || !isFileWritingTool(tc.name) {
		return ""
	}

	// Show path
	pathDisplay := tc.path
	if pathDisplay == "" {
		pathDisplay = "loading..."
	}

	var b strings.Builder
	b.WriteString(styles.roleTool.Render(fmt.Sprintf("✎ %s", pathDisplay)))
	b.WriteString("\n")

	// Show content tail if available
	if tc.content != "" {
		lines := strings.Split(tc.content, "\n")
		if len(lines) > streamingTailLines {
			lines = lines[len(lines)-streamingTailLines:]
		}
		content := strings.Join(lines, "\n")
		b.WriteString(styles.softPanel.
			Width(width - 8).
			Render(lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				Render(content)))
		b.WriteString("\n")
	}

	// Show line count
	lineCount := 0
	if tc.content != "" {
		lineCount = strings.Count(tc.content, "\n")
		if lineCount == 0 && tc.content != "" {
			lineCount = 1
		}
	}
	if lineCount > 0 {
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %d lines", lineCount)))
		b.WriteString("\n")
	}

	return b.String()
}
