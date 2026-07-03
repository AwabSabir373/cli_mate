package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cli_mate/internal/secrets"
)

// SecretScanTool scans files for leaked secrets, API keys, and credentials.
type SecretScanTool struct {
	workspace string
}

func NewSecretScanTool(workspace string) *SecretScanTool {
	return &SecretScanTool{workspace: workspace}
}

func (t *SecretScanTool) Name() string {
	return "secret_scan"
}

func (t *SecretScanTool) Definition() Definition {
	return Definition{
		Name:        "secret_scan",
		Description: "Scan files for leaked secrets, API keys, and credentials. Use after editing files that might contain sensitive data.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory path to scan (relative to workspace)",
				},
			},
		},
	}
}

func (t *SecretScanTool) Execute(ctx context.Context, call Call) (Result, error) {
	path, _ := call.Argument["path"].(string)
	if path == "" {
		return Result{Error: "path is required"}, nil
	}

	// Resolve path relative to workspace
	absPath := filepath.Join(t.workspace, path)
	info, err := os.Stat(absPath)
	if err != nil {
		return Result{Error: fmt.Sprintf("path not found: %s", path)}, nil
	}

	var findings []secrets.Finding

	if info.IsDir() {
		// Scan directory
		err = filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			// Skip binary files and common non-code files
			if shouldSkipFile(p) {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			fileFindings := secrets.Detect(string(data))
			// Adjust line numbers to be relative to file
			for i := range fileFindings {
				fileFindings[i].Kind = filepath.Base(p) + ": " + fileFindings[i].Kind
			}
			findings = append(findings, fileFindings...)
			return nil
		})
	} else {
		// Scan single file
		data, err := os.ReadFile(absPath)
		if err != nil {
			return Result{Error: fmt.Sprintf("failed to read file: %s", err)}, nil
		}
		findings = secrets.Detect(string(data))
	}

	if len(findings) == 0 {
		return Result{Content: "No secrets detected in " + path}, nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d secret(s) in %s:\n\n", len(findings), path))
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("%s at line %d:\n", f.SeverityEmoji(), f.Line))
		b.WriteString(fmt.Sprintf("  Type: %s\n", f.Kind))
		b.WriteString(fmt.Sprintf("  Match: %s\n", f.Match))
		b.WriteString(fmt.Sprintf("  Fix: %s\n\n", f.Suggestion))
	}

	return Result{Content: b.String()}, nil
}

func shouldSkipFile(path string) bool {
	skipExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".ico": true, ".svg": true, ".woff": true, ".woff2": true,
		".ttf": true, ".eot": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	if skipExts[ext] {
		return true
	}
	// Skip node_modules, .git, vendor
	parts := strings.Split(path, string(os.PathSeparator))
	for _, part := range parts {
		if part == "node_modules" || part == ".git" || part == "vendor" {
			return true
		}
	}
	return false
}
