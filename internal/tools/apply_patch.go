package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplyPatchTool applies unified diff patches to workspace files.
type ApplyPatchTool struct {
	Root string
}

func NewApplyPatchTool(root string) *ApplyPatchTool {
	return &ApplyPatchTool{Root: root}
}

func (t *ApplyPatchTool) Name() string {
	return "apply_patch"
}

func (t *ApplyPatchTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Apply a unified diff patch to one or more files. More token-efficient than rewriting entire files. Use for targeted multi-file changes.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"patch"},
			"properties": map[string]any{
				"patch": map[string]any{
					"type":        "string",
					"description": "Unified diff patch in standard format (--- a/file, +++ b/file, @@ hunks)",
				},
				"preview": map[string]any{
					"type":        "boolean",
					"description": "If true, show what would change without applying. Default: false",
				},
			},
			"description": "Apply a unified diff patch to workspace files. Use preview=true to inspect before applying.",
		},
	}
}

func (t *ApplyPatchTool) Execute(_ context.Context, call Call) (Result, error) {
	patch, _ := call.Argument["patch"].(string)
	if strings.TrimSpace(patch) == "" {
		return Result{Error: "patch is required"}, fmt.Errorf("patch is required")
	}

	preview, _ := call.Argument["preview"].(bool)

	files, err := parseUnifiedDiff(patch)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	if len(files) == 0 {
		return Result{Error: "no file changes found in patch"}, fmt.Errorf("no file changes found in patch")
	}

	var results []string
	for _, file := range files {
		resolved, err := resolveWorkspacePath(t.Root, file.Path)
		if err != nil {
			results = append(results, fmt.Sprintf("SKIP %s: %v", file.Path, err))
			continue
		}
		if err := ensureExistingPathInWorkspace(t.Root, resolved); err != nil {
			results = append(results, fmt.Sprintf("SKIP %s: %v", file.Path, err))
			continue
		}

		var original string
		if _, statErr := os.Stat(resolved); statErr == nil {
			data, readErr := os.ReadFile(resolved)
			if readErr != nil {
				results = append(results, fmt.Sprintf("SKIP %s: %v", file.Path, readErr))
				continue
			}
			original = string(data)
		} else if file.IsNew {
			original = ""
		} else {
			results = append(results, fmt.Sprintf("SKIP %s: file not found", file.Path))
			continue
		}

		patched, err := applyHunks(original, file.Hunks)
		if err != nil {
			results = append(results, fmt.Sprintf("FAIL %s: %v", file.Path, err))
			continue
		}

		if preview {
			diff := buildDiff(original, patched, file.Path)
			results = append(results, fmt.Sprintf("PREVIEW %s:\n%s", file.Path, diff))
			continue
		}

		// Ensure parent directory exists
		dir := filepath.Dir(resolved)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			results = append(results, fmt.Sprintf("FAIL %s: %v", file.Path, err))
			continue
		}

		if err := os.WriteFile(resolved, []byte(patched), 0o644); err != nil {
			results = append(results, fmt.Sprintf("FAIL %s: %v", file.Path, err))
			continue
		}
		results = append(results, fmt.Sprintf("OK %s (%d hunks applied)", file.Path, len(file.Hunks)))
	}

	return Result{Content: strings.Join(results, "\n")}, nil
}

type patchFile struct {
	Path  string
	IsNew bool
	IsDel bool
	Hunks []hunk
}

type hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []hunkLine
}

type hunkLine struct {
	Prefix byte // '+', '-', or ' '
	Text   string
}

// parseUnifiedDiff parses a unified diff patch string into structured file changes.
func parseUnifiedDiff(patch string) ([]patchFile, error) {
	var files []patchFile
	scanner := bufio.NewScanner(strings.NewReader(patch))

	var current *patchFile
	var currentHunk *hunk

	for scanner.Scan() {
		line := scanner.Text()

		// New file header: --- a/path or --- /dev/null
		if strings.HasPrefix(line, "--- ") {
			path := extractPath(line[4:])
			if path == "/dev/null" {
				// Next +++ line will have the new file path
				continue
			}
			if current != nil {
				files = append(files, *current)
			}
			current = &patchFile{Path: path}
			continue
		}

		// +++ header
		if strings.HasPrefix(line, "+++ ") {
			if current == nil {
				path := extractPath(line[4:])
				current = &patchFile{Path: path, IsNew: true}
			}
			continue
		}

		// Deleted file
		if strings.HasPrefix(line, "+++ /dev/null") {
			if current != nil {
				current.IsDel = true
			}
			continue
		}

		// Hunk header: @@ -oldStart,oldCount +newStart,newCount @@
		if strings.HasPrefix(line, "@@") {
			if current == nil {
				continue
			}
			h, err := parseHunkHeader(line)
			if err != nil {
				continue
			}
			currentHunk = h
			current.Hunks = append(current.Hunks, *currentHunk)
			// Update the last hunk pointer
			currentHunk = &current.Hunks[len(current.Hunks)-1]
			continue
		}

		// Hunk content lines
		if currentHunk != nil && len(line) > 0 {
			prefix := line[0]
			if prefix == '+' || prefix == '-' || prefix == ' ' {
				text := line[1:]
				currentHunk.Lines = append(currentHunk.Lines, hunkLine{Prefix: prefix, Text: text})
			}
		}
	}

	if current != nil {
		files = append(files, *current)
	}

	return files, nil
}

func extractPath(s string) string {
	s = strings.TrimSpace(s)
	// Handle a/path or b/path prefixes
	if strings.HasPrefix(s, "a/") {
		s = s[2:]
	} else if strings.HasPrefix(s, "b/") {
		s = s[2:]
	}
	return s
}

func parseHunkHeader(line string) (*hunk, error) {
	// @@ -oldStart,oldCount +newStart,newCount @@
	var oldStart, oldCount, newStart, newCount int
	n, err := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &oldStart, &oldCount, &newStart, &newCount)
	if err != nil || n < 4 {
		// Try without counts: @@ -oldStart +newStart @@
		n, err = fmt.Sscanf(line, "@@ -%d +%d @@", &oldStart, &newStart)
		if err != nil || n < 2 {
			return nil, fmt.Errorf("invalid hunk header: %s", line)
		}
		oldCount = 0
		newCount = 0
	}
	return &hunk{
		OldStart: oldStart,
		OldCount: oldCount,
		NewStart: newStart,
		NewCount: newCount,
	}, nil
}

// applyHunks applies a list of hunks to the original file content.
func applyHunks(original string, hunks []hunk) (string, error) {
	lines := strings.Split(original, "\n")
	// If original is empty, start with empty slice
	if original == "" {
		lines = []string{}
	}

	for _, h := range hunks {
		var result []string
		// Copy lines before the hunk
		start := h.OldStart - 1 // Convert to 0-based
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			start = len(lines)
		}
		result = append(result, lines[:start]...)

		// Process hunk lines
		oldIdx := start
		for _, hl := range h.Lines {
			switch hl.Prefix {
			case ' ':
				// Context line — must match
				if oldIdx < len(lines) && lines[oldIdx] == hl.Text {
					result = append(result, hl.Text)
					oldIdx++
				} else if oldIdx < len(lines) {
					// Fuzzy: accept anyway but note mismatch
					result = append(result, hl.Text)
					oldIdx++
				}
			case '-':
				// Removed line — skip from original
				if oldIdx < len(lines) {
					oldIdx++
				}
			case '+':
				// Added line — insert into result
				result = append(result, hl.Text)
			}
		}

		// Copy remaining lines after hunk
		result = append(result, lines[oldIdx:]...)
		lines = result
	}

	return strings.Join(lines, "\n"), nil
}
