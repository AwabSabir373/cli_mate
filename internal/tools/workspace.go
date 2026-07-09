package tools

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxToolOutputBytes = 64 * 1024

func workspaceRoot(root string) (string, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	return filepath.Clean(abs), nil
}

func resolveWorkspacePath(root string, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("path is required")
	}

	root, err := workspaceRoot(root)
	if err != nil {
		return "", err
	}

	path := filepath.FromSlash(input)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)

	// Evaluate symlinks on the deepest existing ancestor to block symlink escapes.
	resolved, err := evalSymlinksInPath(path)
	if err == nil {
		path = resolved
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", input, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside workspace", input)
	}
	return path, nil
}

// evalSymlinksInPath walks up from the given path to find the deepest existing
// ancestor and evaluates its symlinks. This handles paths that may not exist
// yet (e.g., for file creation) while still blocking symlink escapes.
func evalSymlinksInPath(path string) (string, error) {
	// Check if the path itself exists (or is a broken symlink target)
	if _, err := os.Lstat(path); err == nil {
		return filepath.EvalSymlinks(path)
	}
	// Walk up to find an existing parent
	dir := filepath.Dir(path)
	if dir == path || dir == "." || dir == "/" {
		return path, fmt.Errorf("no existing ancestor found")
	}
	resolved, err := evalSymlinksInPath(dir)
	if err != nil {
		return path, err
	}
	// Reconstruct the path from the resolved parent
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func ensureExistingPathInWorkspace(root string, path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	root, err = workspaceRoot(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q resolves outside workspace", path)
	}
	return nil
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string {
	out := b.buf.String()
	if b.truncated {
		out += "\n... output truncated ..."
	}
	return out
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}
