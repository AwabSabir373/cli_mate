package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type GlobTool struct {
	Root string
}

func NewGlobTool(root string) *GlobTool {
	return &GlobTool{Root: root}
}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Find files matching a glob pattern. Supports ** for recursive matching, exclude patterns, file metadata, and sorting by name/time/size.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"pattern"},
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match files. Use '**' for recursive matching, '*' for single directory. Examples: '**/*.go', 'internal/**/*.go', 'cmd/**/*'.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in (default: workspace root)",
				},
				"exclude": map[string]any{
					"type":        "string",
					"description": "Glob pattern to exclude, e.g. '*_test.go' to exclude test files",
				},
				"include_details": map[string]any{
					"type":        "boolean",
					"description": "If true, include file size and last modification time in output. Default: false",
				},
				"sort_by": map[string]any{
					"type":        "string",
					"description": "Sort order: 'name' (default), 'time', or 'size'",
					"enum":        []string{"name", "time", "size"},
				},
			},
			"description": "Find files matching a glob pattern. Supports ** for recursive matching, exclude patterns, and file metadata. Use this to discover files by name patterns instead of using shell find/ls.",
		},
	}
}

func (t *GlobTool) Execute(_ context.Context, call Call) (Result, error) {
	pattern, _ := call.Argument["pattern"].(string)
	if strings.TrimSpace(pattern) == "" {
		return Result{Error: "pattern is required"}, fmt.Errorf("pattern is required")
	}

	searchRoot := t.Root
	if path, ok := call.Argument["path"].(string); ok && strings.TrimSpace(path) != "" {
		resolved, err := resolveWorkspacePath(t.Root, path)
		if err != nil {
			return Result{Error: err.Error()}, err
		}
		if err := ensureExistingPathInWorkspace(t.Root, resolved); err != nil {
			return Result{Error: err.Error()}, err
		}
		searchRoot = resolved
	}

	excludePattern, _ := call.Argument["exclude"].(string)
	includeDetails, _ := call.Argument["include_details"].(bool)
	sortBy, _ := call.Argument["sort_by"].(string)
	if sortBy == "" {
		sortBy = "name"
	}

	// Normalize pattern to use forward slashes for cross-platform consistency
	pattern = filepath.ToSlash(pattern)

	// Walk and match
	type globMatch struct {
		path    string
		size    int64
		modTime time.Time
	}
	var matches []globMatch
	skipDirs := map[string]bool{".git": true, ".idea": true, "node_modules": true, "vendor": true, ".openclaude": true}

	err := filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(searchRoot, path)
		if err != nil {
			return nil
		}

		// Apply exclude pattern
		if excludePattern != "" {
			excluded, _ := filepath.Match(excludePattern, d.Name())
			if excluded {
				return nil
			}
			// Also try matching against the relative path
			excluded, _ = filepath.Match(excludePattern, filepath.ToSlash(rel))
			if excluded {
				return nil
			}
		}

		matched, err := matchGlob(pattern, filepath.ToSlash(rel))
		if err != nil {
			return nil
		}
		if matched {
			info, err := d.Info()
			m := globMatch{path: filepath.ToSlash(rel)}
			if err == nil {
				m.size = info.Size()
				m.modTime = info.ModTime()
			}
			matches = append(matches, m)
		}
		return nil
	})
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	if len(matches) == 0 {
		return Result{Content: "No files matched."}, nil
	}

	// Sort
	sort.Slice(matches, func(i, j int) bool {
		switch sortBy {
		case "time":
			return matches[i].modTime.After(matches[j].modTime)
		case "size":
			return matches[i].size > matches[j].size
		default:
			return strings.ToLower(matches[i].path) < strings.ToLower(matches[j].path)
		}
	})

	// Limit output
	const maxResults = 200
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	var b strings.Builder
	for _, m := range matches {
		if includeDetails {
			fmt.Fprintf(&b, "%s\t%s\t%s\n", m.path, formatSize(m.size), m.modTime.Format("2006-01-02 15:04:05"))
		} else {
			b.WriteString(m.path)
			b.WriteString("\n")
		}
	}

	if len(matches) > maxResults {
		fmt.Fprintf(&b, "... and %d more matches\n", len(matches)-maxResults)
	}

	return Result{Content: b.String()}, nil
}

// matchGlob implements glob matching with ** support
func matchGlob(pattern, name string) (bool, error) {
	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, name)
	}

	// Simple filepath.Match
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false, nil
	}
	return matched, nil
}

func matchDoubleStar(pattern, name string) (bool, error) {
	// Split on ** and try to match each segment
	parts := strings.Split(pattern, "**")

	// If pattern starts with **, the prefix is empty
	// If pattern ends with **, the suffix is empty
	prefix := strings.TrimSuffix(parts[0], "/")
	var suffix string
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
	}

	// Check prefix
	if prefix != "" {
		if !strings.HasPrefix(name, prefix) {
			return false, nil
		}
		// Remove prefix from name for remaining checks
		name = name[len(prefix):]
	}

	// No suffix means anything after prefix matches
	if suffix == "" {
		return true, nil
	}

	// Try to match suffix against any remaining segment
	// Walk through the name to find where the suffix could match
	name = strings.TrimPrefix(name, "/")
	for i := 0; i <= len(name); i++ {
		// Check the suffix against the substring starting at position i
		candidate := name[i:]
		matched, err := filepath.Match(suffix, candidate)
		if err == nil && matched {
			return true, nil
		}
		// Also try matching just the basename at each segment boundary
		if i < len(name) && name[i] == '/' {
			candidate := name[i+1:]
			matched, err := filepath.Match(suffix, candidate)
			if err == nil && matched {
				return true, nil
			}
		}
	}

	return false, nil
}
