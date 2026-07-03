package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type FileReadTool struct {
	Root string
}

func NewFileReadTool(root string) *FileReadTool {
	return &FileReadTool{Root: root}
}

func (t *FileReadTool) Name() string {
	return "file_read"
}

func (t *FileReadTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Read a UTF-8 text file from the workspace. Supports line ranges (line_start/line_end) and size limits (max_bytes). Paths must stay inside the workspace.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file, relative to workspace root or absolute",
				},
				"max_bytes": map[string]any{
					"type":        "integer",
					"description": "Maximum bytes to read (default: 524288 = 512KB, max: 10485760 = 10MB). Larger files will be truncated.",
				},
				"line_start": map[string]any{
					"type":        "integer",
					"description": "Starting line number (1-based). Default: 1",
				},
				"line_end": map[string]any{
					"type":        "integer",
					"description": "Ending line number (inclusive). If not set, reads to end of file or max_bytes limit.",
				},
			},
			"description": "Read a UTF-8 text file from the workspace with optional line-range and size-limit parameters.",
		},
	}
}

func (t *FileReadTool) Execute(_ context.Context, call Call) (Result, error) {
	path, _ := call.Argument["path"].(string)
	resolved, err := resolveWorkspacePath(t.Root, path)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := ensureExistingPathInWorkspace(t.Root, resolved); err != nil {
		return Result{Error: err.Error()}, err
	}

	maxBytes := 512 * 1024 // default 512KB
	if mb, ok := call.Argument["max_bytes"].(float64); ok && mb > 0 {
		maxBytes = int(mb)
		if maxBytes > 10*1024*1024 {
			maxBytes = 10 * 1024 * 1024 // cap at 10MB
		}
	}

	lineStart := 1
	if ls, ok := call.Argument["line_start"].(float64); ok && ls >= 1 {
		lineStart = int(ls)
	}
	lineEnd := 0 // 0 means no limit
	if le, ok := call.Argument["line_end"].(float64); ok && le >= 1 {
		lineEnd = int(le)
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	// Check if file is likely binary (null bytes in first 512 bytes)
	if hasNullBytes(content, 512) {
		return Result{Content: fmt.Sprintf("(binary file, %d bytes)", len(content))}, nil
	}

	// Apply line range if requested
	if lineStart > 1 || lineEnd > 0 {
		lines := splitLines(string(content))
		if lineStart > len(lines) {
			return Result{Content: fmt.Sprintf("(line %d exceeds file length of %d lines)", lineStart, len(lines))}, nil
		}
		if lineEnd == 0 || lineEnd > len(lines) {
			lineEnd = len(lines)
		}
		if lineStart > lineEnd {
			return Result{Content: fmt.Sprintf("(line_start %d is after line_end %d)", lineStart, lineEnd)}, nil
		}
		selected := lines[lineStart-1 : lineEnd]
		content = []byte(strings.Join(selected, "\n"))
		if !strings.HasSuffix(string(content), "\n") && len(selected) > 0 {
			content = append(content, '\n')
		}
		result := string(content)
		if lineEnd-lineStart+1 > 1 {
			result = fmt.Sprintf("// %s lines %d-%d (%d lines)\n%s", path, lineStart, lineEnd, lineEnd-lineStart+1, result)
		}
		return Result{Content: result}, nil
	}

	// Apply max_bytes limit
	if len(content) > maxBytes {
		truncated := string(content[:maxBytes])
		truncated += fmt.Sprintf("\n... truncated (%d of %d bytes, use line_start/line_end to read specific sections) ...", maxBytes, len(content))
		return Result{Content: truncated}, nil
	}

	return Result{Content: string(content)}, nil
}

func hasNullBytes(data []byte, checkLen int) bool {
	if checkLen > len(data) {
		checkLen = len(data)
	}
	for _, b := range data[:checkLen] {
		if b == 0 {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	for {
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			lines = append(lines, s)
			break
		}
		lines = append(lines, s[:idx])
		s = s[idx+1:]
	}
	return lines
}
