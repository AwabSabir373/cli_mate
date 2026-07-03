package tools

import (
	"context"
	"os"
	"path/filepath"
)

type FileWriteTool struct {
	Root string
}

func NewFileWriteTool(root string) *FileWriteTool {
	return &FileWriteTool{Root: root}
}

func (t *FileWriteTool) Name() string {
	return "file_write"
}

func (t *FileWriteTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Write a UTF-8 text file inside the workspace, creating parent directories as needed.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"path", "content"},
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path for the new file, relative to workspace root. Parent directories are created automatically.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full file content to write",
				},
			},
			"description": "Write a new file or overwrite an existing one. Prefer file_edit for small changes to existing files.",
		},
	}
}

func (t *FileWriteTool) Execute(_ context.Context, call Call) (Result, error) {
	path, _ := call.Argument["path"].(string)
	content, _ := call.Argument["content"].(string)

	resolved, err := resolveWorkspacePath(t.Root, path)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return Result{Error: err.Error()}, err
	}
	if _, err := os.Lstat(resolved); err == nil {
		if err := ensureExistingPathInWorkspace(t.Root, resolved); err != nil {
			return Result{Error: err.Error()}, err
		}
	}
	if err := ensureExistingPathInWorkspace(t.Root, filepath.Dir(resolved)); err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
		return Result{Error: err.Error()}, err
	}
	return Result{Content: path}, nil
}
