package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ReviewTool struct {
	WorkspaceRoot string
}

func NewReviewTool(workspaceRoot string) *ReviewTool {
	return &ReviewTool{WorkspaceRoot: workspaceRoot}
}

func (t *ReviewTool) Name() string {
	return "review"
}

func (t *ReviewTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Review code changes. Shows git diff and provides a code review summary.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"files": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Specific files to review (optional, defaults to all staged changes)",
				},
				"staged": map[string]any{
					"type":        "boolean",
					"description": "Review staged changes only (default: true)",
				},
			},
			"description": "Review code changes by showing git diff and analyzing the changes.",
		},
	}
}

func (t *ReviewTool) Execute(_ context.Context, call Call) (Result, error) {
	staged, _ := call.Argument["staged"].(bool)
	if !staged {
		// Default to reviewing all changes when not explicitly set
	}

	var cmd *exec.Cmd
	if staged {
		cmd = exec.Command("git", "diff", "--cached")
	} else {
		cmd = exec.Command("git", "diff")
	}
	cmd.Dir = t.WorkspaceRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Error: fmt.Sprintf("git diff failed: %v", err)}, err
	}

	if strings.TrimSpace(string(output)) == "" {
		return Result{Content: "No changes to review."}, nil
	}

	var fileCmd *exec.Cmd
	if staged {
		fileCmd = exec.Command("git", "diff", "--cached", "--name-only")
	} else {
		fileCmd = exec.Command("git", "diff", "--name-only")
	}
	fileCmd.Dir = t.WorkspaceRoot

	fileOutput, _ := fileCmd.CombinedOutput()
	files := strings.Split(strings.TrimSpace(string(fileOutput)), "\n")

	var b strings.Builder
	b.WriteString("## Code Review\n\n")
	b.WriteString("### Files Changed\n")
	for _, f := range files {
		if strings.TrimSpace(f) != "" {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("\n### Changes\n")
	b.WriteString("```diff\n")
	b.WriteString(string(output))
	b.WriteString("\n```\n")

	return Result{Content: b.String()}, nil
}

type DiffTool struct {
	WorkspaceRoot string
}

func NewDiffTool(workspaceRoot string) *DiffTool {
	return &DiffTool{WorkspaceRoot: workspaceRoot}
}

func (t *DiffTool) Name() string {
	return "diff"
}

func (t *DiffTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Show the difference between files or git states.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":        "string",
					"description": "Specific file to diff (optional)",
				},
				"compare_to": map[string]any{
					"type":        "string",
					"description": "Compare to a specific ref (default: HEAD)",
				},
			},
			"description": "Show differences between files or git states.",
		},
	}
}

func (t *DiffTool) Execute(_ context.Context, call Call) (Result, error) {
	file, _ := call.Argument["file"].(string)
	compareTo, _ := call.Argument["compare_to"].(string)

	args := []string{"diff"}
	if compareTo != "" {
		args = append(args, compareTo)
	}
	if file != "" {
		args = append(args, "--", file)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = t.WorkspaceRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Error: fmt.Sprintf("git diff failed: %v", err)}, err
	}

	if strings.TrimSpace(string(output)) == "" {
		return Result{Content: "No differences found."}, nil
	}

	return Result{Content: "```diff\n" + string(output) + "\n```"}, nil
}

type CommitTool struct {
	WorkspaceRoot string
}

func NewCommitTool(workspaceRoot string) *CommitTool {
	return &CommitTool{WorkspaceRoot: workspaceRoot}
}

func (t *CommitTool) Name() string {
	return "commit"
}

func (t *CommitTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Create a git commit with the staged changes.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"message"},
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "Commit message",
				},
				"files": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Specific files to stage and commit (optional, defaults to all changes)",
				},
			},
			"description": "Create a git commit with staged changes.",
		},
	}
}

func (t *CommitTool) Execute(_ context.Context, call Call) (Result, error) {
	message, _ := call.Argument["message"].(string)
	if strings.TrimSpace(message) == "" {
		return Result{Error: "message is required"}, fmt.Errorf("message is required")
	}

	files, _ := call.Argument["files"].([]interface{})

	if len(files) > 0 {
		var filePaths []string
		for _, f := range files {
			if s, ok := f.(string); ok {
				filePaths = append(filePaths, s)
			}
		}
		args := append([]string{"add"}, filePaths...)
		cmd := exec.Command("git", args...)
		cmd.Dir = t.WorkspaceRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			return Result{Error: fmt.Sprintf("git add failed: %v\n%s", err, string(output))}, err
		}
	}

	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = t.WorkspaceRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{Error: fmt.Sprintf("git commit failed: %v\n%s", err, string(output))}, err
	}

	return Result{Content: string(output)}, nil
}

type CompactTool struct {
	WorkspaceRoot string
}

func NewCompactTool(workspaceRoot string) *CompactTool {
	return &CompactTool{WorkspaceRoot: workspaceRoot}
}

func (t *CompactTool) Name() string {
	return "compact"
}

func (t *CompactTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Compact/clean the workspace by removing temporary files, build artifacts, etc.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "If true, show what would be deleted without actually deleting (default: true)",
				},
			},
			"description": "Clean temporary files and build artifacts from the workspace.",
		},
	}
}

func (t *CompactTool) Execute(_ context.Context, call Call) (Result, error) {
	dryRun := true // default to dry run
	if dr, ok := call.Argument["dry_run"].(bool); ok {
		dryRun = dr
	}

	cleanPatterns := []string{
		// JavaScript/Node
		"node_modules/.cache",
		".next/cache",
		".nuxt",
		".cache",
		"dist",
		"build",
		".turbo",
		// Python
		"__pycache__",
		".pytest_cache",
		"*.pyc",
		".mypy_cache",
		".ruff_cache",
		"*.egg-info",
		// Go
		".mimocode",
		// Rust
		"target",
		// General
		".DS_Store",
		"Thumbs.db",
		"*.swp",
		"*.swo",
		"*~",
		// IDE
		".idea/workspace.xml",
		".vscode/settings.json",
		// Project-specific
		".cli_mate/worktrees",
	}

	var found []string
	filepath.Walk(t.WorkspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(t.WorkspaceRoot, path)
		for _, pattern := range cleanPatterns {
			matched, _ := filepath.Match(pattern, filepath.Base(path))
			if matched || strings.Contains(rel, pattern) {
				found = append(found, rel)
				if !dryRun {
					os.RemoveAll(path)
				}
				break
			}
		}
		return nil
	})

	if len(found) == 0 {
		return Result{Content: "Workspace is already clean."}, nil
	}

	var b strings.Builder
	if dryRun {
		b.WriteString("Dry run - would remove:\n")
	} else {
		b.WriteString("Removed:\n")
	}
	for _, f := range found {
		fmt.Fprintf(&b, "- %s\n", f)
	}

	return Result{Content: b.String()}, nil
}
