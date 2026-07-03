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

type FileListTool struct {
	Root string
}

func NewFileListTool(root string) *FileListTool {
	return &FileListTool{Root: root}
}

func (t *FileListTool) Name() string {
	return "file_list"
}

func (t *FileListTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "List files and directories at a path. Supports recursive listing, depth control, type filtering, and sorting. Returns a sorted directory listing with type and size indicators.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to list (relative to workspace root or absolute). Use '.' for workspace root.",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "If true, list files recursively. Default: false",
				},
				"depth": map[string]any{
					"type":        "integer",
					"description": "Maximum directory depth for recursive listing (default: unlimited, only used when recursive=true)",
				},
				"show_hidden": map[string]any{
					"type":        "boolean",
					"description": "If true, show hidden files/dirs (starting with '.'). Default: false",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Filter by type: 'all' (default), 'file', or 'dir'",
					"enum":        []string{"all", "file", "dir"},
				},
				"sort_by": map[string]any{
					"type":        "string",
					"description": "Sort order: 'name' (default), 'time', or 'size'",
					"enum":        []string{"name", "time", "size"},
				},
			},
		},
	}
}

func (t *FileListTool) Execute(_ context.Context, call Call) (Result, error) {
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) == "" {
		return Result{Error: "path is required"}, fmt.Errorf("path is required")
	}

	resolved, err := resolveWorkspacePath(t.Root, path)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	recursive, _ := call.Argument["recursive"].(bool)
	showHidden, _ := call.Argument["show_hidden"].(bool)
	typeFilter, _ := call.Argument["type"].(string)
	if typeFilter == "" {
		typeFilter = "all"
	}
	sortBy, _ := call.Argument["sort_by"].(string)
	if sortBy == "" {
		sortBy = "name"
	}

	maxDepth := -1 // unlimited
	if recursive {
		if depth, ok := call.Argument["depth"].(float64); ok {
			maxDepth = int(depth)
		}
	}

	if !info.IsDir() {
		return Result{Content: fmt.Sprintf("%s (file, %s)", filepath.Base(resolved), formatSize(info.Size()))}, nil
	}

	var entries []listEntry

	if recursive {
		skipDirs := map[string]bool{".git": true, ".idea": true, "node_modules": true, "vendor": true, ".openclaude": true}
		err = filepath.WalkDir(resolved, func(walkPath string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, relErr := filepath.Rel(resolved, walkPath)
			if relErr != nil {
				return nil
			}
			if rel == "." {
				return nil // skip root
			}
			depth := strings.Count(rel, string(filepath.Separator))
			if maxDepth >= 0 && depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if !showHidden && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				if typeFilter == "all" || typeFilter == "dir" {
					fi, fiErr := d.Info()
					if fiErr == nil {
						entries = append(entries, listEntry{name: rel + "/", isDir: true, size: fi.Size(), modTime: fi.ModTime()})
					}
				}
				return nil
			}
			if !showHidden && strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			if typeFilter == "all" || typeFilter == "file" {
				fi, fiErr := d.Info()
				if fiErr == nil {
					entries = append(entries, listEntry{name: rel, isDir: false, size: fi.Size(), modTime: fi.ModTime()})
				}
			}
			return nil
		})
	} else {
		dirEntries, readErr := os.ReadDir(resolved)
		if readErr != nil {
			return Result{Error: readErr.Error()}, readErr
		}
		for _, entry := range dirEntries {
			if !showHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if entry.IsDir() {
				if typeFilter == "all" || typeFilter == "dir" {
					fi, fiErr := entry.Info()
					if fiErr == nil {
						entries = append(entries, listEntry{name: entry.Name() + "/", isDir: true, size: fi.Size(), modTime: fi.ModTime()})
					}
				}
			} else {
				if typeFilter == "all" || typeFilter == "file" {
					fi, fiErr := entry.Info()
					if fiErr == nil {
						entries = append(entries, listEntry{name: entry.Name(), isDir: false, size: fi.Size(), modTime: fi.ModTime()})
					}
				}
			}
		}
	}

	if err != nil {
		return Result{Error: err.Error()}, err
	}

	// Sort entries
	sort.Slice(entries, func(i, j int) bool {
		// Directories first
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		switch sortBy {
		case "time":
			return entries[i].modTime.After(entries[j].modTime)
		case "size":
			return entries[i].size > entries[j].size
		default:
			return strings.ToLower(entries[i].name) < strings.ToLower(entries[j].name)
		}
	})

	rel, _ := filepath.Rel(t.Root, resolved)
	if rel == "." {
		rel = "/"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s/\n", rel)
	for _, e := range entries {
		if e.isDir {
			fmt.Fprintf(&b, "  %s\n", e.name)
		} else {
			fmt.Fprintf(&b, "  %s (%s)\n", e.name, formatSize(e.size))
		}
	}
	fmt.Fprintf(&b, "\n%d entries", len(entries))

	// Limit output
	output := b.String()
	const maxOutputBytes = 16000
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes] + "\n... truncated ..."
	}

	return Result{Content: output}, nil
}

type listEntry struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}
