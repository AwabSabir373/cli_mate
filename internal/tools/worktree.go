package tools

import (
	"context"
	"fmt"
	"strings"

	"cli_mate/internal/worktrees"
)

// WorktreeCreateTool creates a new git worktree.
type WorktreeCreateTool struct {
	workspace string
}

func NewWorktreeCreateTool(workspace string) *WorktreeCreateTool {
	return &WorktreeCreateTool{workspace: workspace}
}

func (t *WorktreeCreateTool) Name() string {
	return "worktree_create"
}

func (t *WorktreeCreateTool) Definition() Definition {
	return Definition{
		Name:        "worktree_create",
		Description: "Create an isolated git worktree for safe experimentation.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the worktree (optional, auto-generated if empty)",
				},
			},
		},
	}
}

func (t *WorktreeCreateTool) Execute(ctx context.Context, call Call) (Result, error) {
	name, _ := call.Argument["name"].(string)

	wt, err := worktrees.Create(ctx, t.workspace, name)
	if err != nil {
		return Result{Error: err.Error()}, nil
	}

	return Result{
		Content: fmt.Sprintf("Created worktree: %s at %s", wt.Name, wt.Path),
	}, nil
}

// WorktreeListTool lists all cli_mate worktrees.
type WorktreeListTool struct {
	workspace string
}

func NewWorktreeListTool(workspace string) *WorktreeListTool {
	return &WorktreeListTool{workspace: workspace}
}

func (t *WorktreeListTool) Name() string {
	return "worktree_list"
}

func (t *WorktreeListTool) Definition() Definition {
	return Definition{
		Name:        "worktree_list",
		Description: "List all cli_mate worktrees in the repository.",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *WorktreeListTool) Execute(ctx context.Context, call Call) (Result, error) {
	wtList, err := worktrees.List(ctx, t.workspace)
	if err != nil {
		return Result{Error: err.Error()}, nil
	}

	if len(wtList) == 0 {
		return Result{Content: "No worktrees found."}, nil
	}

	var lines []string
	for _, wt := range wtList {
		lines = append(lines, worktrees.FormatWorktree(wt))
	}

	return Result{
		Content: strings.Join(lines, "\n"),
	}, nil
}

// WorktreeCleanupTool removes all cli_mate worktrees.
type WorktreeCleanupTool struct {
	workspace string
}

func NewWorktreeCleanupTool(workspace string) *WorktreeCleanupTool {
	return &WorktreeCleanupTool{workspace: workspace}
}

func (t *WorktreeCleanupTool) Name() string {
	return "worktree_cleanup"
}

func (t *WorktreeCleanupTool) Definition() Definition {
	return Definition{
		Name:        "worktree_cleanup",
		Description: "Remove all cli_mate worktrees to free disk space.",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *WorktreeCleanupTool) Execute(ctx context.Context, call Call) (Result, error) {
	removed, err := worktrees.Cleanup(ctx, t.workspace)
	if err != nil {
		return Result{Error: err.Error()}, nil
	}

	return Result{
		Content: fmt.Sprintf("Removed %d worktree(s).", removed),
	}, nil
}
