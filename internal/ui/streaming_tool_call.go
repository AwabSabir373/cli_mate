package ui

import (
	"encoding/json"
	"fmt"
	"strings"
)

const streamingTailLines = 5

// streamingToolCall represents an in-progress tool call being streamed.
type streamingToolCall struct {
	name      string
	path      string
	content   string
	args      string
	completed bool
	decoder   *streamingDecoder // incremental JSON decoder for content preview
}

// feedArgs incrementally feeds tool call arguments to the decoder for live
// content preview. The decoder extracts path and content lines as they stream.
func (tc *streamingToolCall) feedArgs(fragment string) {
	if tc.decoder == nil {
		tc.decoder = newStreamingDecoder()
	}
	tc.decoder.feed(fragment)
	// Update path from decoder
	if tc.decoder.path != "" {
		tc.path = tc.decoder.path
	}
	// Update content preview from decoder tail
	if tc.decoder.hasContent() {
		lines := tc.decoder.tailLines()
		tc.content = strings.Join(lines, "\n")
	}
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
	if tc == nil || strings.TrimSpace(tc.name) == "" {
		return ""
	}

	// Show path with status indicator
	pathDisplay := tc.path
	if pathDisplay == "" {
		pathDisplay = strings.TrimSpace(tc.name)
	}

	var b strings.Builder
	statusIcon := styles.spinner.Render(">")
	status := "preparing"
	if isFileWritingTool(tc.name) {
		status = "writing"
	} else if strings.Contains(strings.ToLower(tc.name), "read") || strings.Contains(strings.ToLower(tc.name), "grep") || strings.Contains(strings.ToLower(tc.name), "glob") {
		status = "reading"
	}
	b.WriteString(styles.roleTool.Render(fmt.Sprintf("%s %s %s", statusIcon, status, pathDisplay)))
	b.WriteString("\n")

	// Show content tail with syntax highlighting
	if tc.content != "" {
		lines := strings.Split(tc.content, "\n")
		totalLines := len(lines)
		if totalLines > streamingTailLines {
			lines = lines[totalLines-streamingTailLines:]
		}
		content := strings.Join(lines, "\n")

		// Try to highlight based on file extension
		lang := ""
		if tc.path != "" {
			lexer := cachedLexerForPath(tc.path)
			if lexer != nil {
				lang = lexer.Config().Name
			}
		}
		if lang != "" {
			content = highlightCode(content, lang)
		}

		b.WriteString(styles.softPanel.
			Width(max(20, width-8)).
			Render(content))
		b.WriteString("\n")
	}

	// Show progress: line count and byte size
	lineCount := 0
	byteCount := 0
	if tc.content != "" {
		lineCount = strings.Count(tc.content, "\n")
		if lineCount == 0 && tc.content != "" {
			lineCount = 1
		}
		byteCount = len(tc.content)
	}
	if lineCount > 0 {
		sizeStr := formatByteCount(byteCount)
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %d lines | %s", lineCount, sizeStr)))
		b.WriteString("\n")
	} else {
		b.WriteString(styles.muted.Render("  writing..."))
		b.WriteString("\n")
	}

	return b.String()
}

// skipJSONSpace advances past JSON insignificant whitespace.
func skipJSONSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

// jsonStringValueStart finds `"key"` then, tolerating whitespace around the colon,
// returns the index just past the opening quote of its string value, or -1.
func jsonStringValueStart(s, key string) int {
	marker := `"` + key + `"`
	idx := strings.Index(s, marker)
	if idx < 0 {
		return -1
	}
	i := skipJSONSpace(s, idx+len(marker))
	if i >= len(s) || s[i] != ':' {
		return -1
	}
	i = skipJSONSpace(s, i+1)
	if i >= len(s) || s[i] != '"' {
		return -1
	}
	return i + 1
}

// formatByteCount formats byte count as human-readable string.
func formatByteCount(bytes int) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
