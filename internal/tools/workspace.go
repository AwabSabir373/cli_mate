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

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", input, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside workspace", input)
	}
	return path, nil
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
