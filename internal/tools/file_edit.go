package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const maxEditFileSize = 1 << 30 // 1 GiB

type FileEditTool struct {
	Root string
}

func NewFileEditTool(root string) *FileEditTool {
	return &FileEditTool{Root: root}
}

func (t *FileEditTool) Name() string {
	return "file_edit"
}

func (t *FileEditTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Edit a workspace text file by replacing text, targeting by line number, or inserting at start/end. Use after reading the file.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file, relative to workspace root",
				},
				"old": map[string]any{
					"type":        "string",
					"description": "Exact text to replace (including whitespace). Must match exactly once in the file (unless replace_all=true)",
				},
				"new": map[string]any{
					"type":        "string",
					"description": "New text to replace the old text with",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "If true, replace ALL occurrences of old text. Use with caution. Default: false",
				},
				"line_number": map[string]any{
					"type":        "integer",
					"description": "1-based line number to replace. The entire line is replaced with 'new' text. Overrides 'old' parameter.",
				},
				"insert_at": map[string]any{
					"type":        "string",
					"description": "Insert mode: 'start' inserts at beginning of file, 'end' appends to end of file. Overrides 'old' and 'line_number'.",
					"enum":        []string{"start", "end"},
				},
				"preview": map[string]any{
					"type":        "boolean",
					"description": "If true, show the diff preview without actually modifying the file. Default: false",
				},
			},
			"description": "Edit a file by replacing exact text, targeting a line number, or inserting at start/end. Use preview=true to see changes before applying.",
		},
	}
}

func (t *FileEditTool) Execute(_ context.Context, call Call) (Result, error) {
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) == "" {
		err := fmt.Errorf("path is required")
		return Result{Error: err.Error()}, err
	}

	resolved, err := resolveWorkspacePath(t.Root, path)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := ensureExistingPathInWorkspace(t.Root, resolved); err != nil {
		return Result{Error: err.Error()}, err
	}

	// UNC path protection on Windows
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(resolved, "\\\\") || strings.HasPrefix(resolved, "//") {
			err := fmt.Errorf("UNC paths are not allowed for security reasons")
			return Result{Error: err.Error()}, err
		}
	}

	// Check file size limit
	info, err := os.Stat(resolved)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if info.Size() > maxEditFileSize {
		err := fmt.Errorf("file too large (%d bytes, max %d bytes)", info.Size(), maxEditFileSize)
		return Result{Error: err.Error()}, err
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	text := string(content)

	preview, _ := call.Argument["preview"].(bool)
	insertAt, _ := call.Argument["insert_at"].(string)
	lineNumber, _ := call.Argument["line_number"].(float64)
	oldText, _ := call.Argument["old"].(string)
	newText, _ := call.Argument["new"].(string)

	var updated string
	var editDesc string

	switch {
	case insertAt != "":
		if newText == "" {
			err := fmt.Errorf("new text is required for insert_at mode")
			return Result{Error: err.Error()}, err
		}
		switch insertAt {
		case "start":
			updated = newText + "\n" + text
			editDesc = fmt.Sprintf("inserted text at start of %s", path)
		case "end":
			if !strings.HasSuffix(text, "\n") {
				updated = text + "\n" + newText + "\n"
			} else {
				updated = text + newText + "\n"
			}
			editDesc = fmt.Sprintf("appended text at end of %s", path)
		default:
			err := fmt.Errorf("insert_at must be 'start' or 'end', got %q", insertAt)
			return Result{Error: err.Error()}, err
		}

	case lineNumber > 0:
		ln := int(lineNumber)
		if ln < 1 {
			err := fmt.Errorf("line_number must be >= 1, got %d", ln)
			return Result{Error: err.Error()}, err
		}
		lines := strings.Split(text, "\n")
		if ln > len(lines) {
			err := fmt.Errorf("line_number %d exceeds file length of %d lines", ln, len(lines))
			return Result{Error: err.Error()}, err
		}
		oldLine := lines[ln-1]
		lines[ln-1] = newText
		updated = strings.Join(lines, "\n")
		editDesc = fmt.Sprintf("edited line %d of %s: %q -> %q", ln, path, oldLine, newText)

	case oldText != "":
		if newText == "" && !preview {
			err := fmt.Errorf("new text is required")
			return Result{Error: err.Error()}, err
		}
		// Check for identical edit
		if oldText == newText {
			err := fmt.Errorf("old and new text are identical; no changes needed")
			return Result{Error: err.Error()}, err
		}
		replaceAll, _ := call.Argument["replace_all"].(bool)
		count := strings.Count(text, oldText)
		if count == 0 {
			err := fmt.Errorf("old text was not found in %s", path)
			return Result{Error: err.Error()}, err
		}
		if count > 1 && !replaceAll {
			err := fmt.Errorf("old text occurs %d times in %s; set replace_all=true or provide a more specific old string", count, path)
			return Result{Error: err.Error()}, err
		}
		n := 1
		if replaceAll {
			n = -1
		}
		updated = strings.Replace(text, oldText, newText, n)
		editDesc = fmt.Sprintf("edited %s (%d replacement(s))", path, count)

	default:
		err := fmt.Errorf("one of 'old', 'line_number', or 'insert_at' is required")
		return Result{Error: err.Error()}, err
	}

	// Preview mode: don't write, just show diff
	if preview {
		diff := buildDiff(text, updated, path)
		return Result{Content: "(preview only, no changes applied)\n" + diff}, nil
	}

	// Write the file
	if err := os.WriteFile(resolved, []byte(updated), 0o600); err != nil {
		return Result{Error: err.Error()}, err
	}
	return Result{Content: formatEditResult(path, editDesc, text, updated)}, nil
}

func formatEditResult(path string, desc string, oldText string, newText string) string {
	return desc + "\n" + buildDiff(oldText, newText, path)
}

func buildDiff(oldText, newText, path string) string {
	var b strings.Builder
	b.WriteString("```diff\n")

	oldLines := strings.Split(strings.TrimSuffix(oldText, "\n"), "\n")
	newLines := strings.Split(strings.TrimSuffix(newText, "\n"), "\n")

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}

	oldSuffix := len(oldLines)
	newSuffix := len(newLines)
	for oldSuffix > prefix && newSuffix > prefix && oldLines[oldSuffix-1] == newLines[newSuffix-1] {
		oldSuffix--
		newSuffix--
	}

	const contextLines = 3
	oldStart := maxInt(0, prefix-contextLines)
	newEnd := minInt(len(newLines), newSuffix+contextLines)

	fmt.Fprintf(&b, "@@ %s:%d @@\n", filepath.Base(path), oldStart+1)
	for _, line := range oldLines[oldStart:prefix] {
		fmt.Fprintf(&b, " %s\n", line)
	}
	for _, line := range oldLines[prefix:oldSuffix] {
		fmt.Fprintf(&b, "-%s\n", line)
	}
	for _, line := range newLines[prefix:newSuffix] {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	for _, line := range newLines[newSuffix:newEnd] {
		fmt.Fprintf(&b, " %s\n", line)
	}
	b.WriteString("```")
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
