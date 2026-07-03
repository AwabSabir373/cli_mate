// Package worktrees manages isolated git worktrees for safe experimentation.
package worktrees

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Worktree represents a git worktree.
type Worktree struct {
	Name      string
	Path      string
	Branch    string
	GitStatus string // "clean", "dirty", ""
}

// GetStatus returns the git status of a worktree.
func GetStatus(ctx context.Context, repoRoot string, name string) (string, error) {
	worktreesDir := filepath.Join(repoRoot, ".cli_mate", "worktrees")
	worktreePath := filepath.Join(worktreesDir, name)

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		return "clean", nil
	}
	return "dirty", nil
}

// Create creates a new git worktree for experimentation.
func Create(ctx context.Context, repoRoot string, name string) (*Worktree, error) {
	if name == "" {
		name = fmt.Sprintf("cli_mate_%d", time.Now().UnixNano())
	}

	worktreesDir := filepath.Join(repoRoot, ".cli_mate", "worktrees")
	if err := os.MkdirAll(worktreesDir, 0700); err != nil {
		return nil, fmt.Errorf("create worktrees dir: %w", err)
	}

	worktreePath := filepath.Join(worktreesDir, name)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return &Worktree{Name: name, Path: worktreePath}, nil
	}

	// Create the worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return &Worktree{Name: name, Path: worktreePath}, nil
}

// List returns all cli_mate worktrees in the repository.
func List(ctx context.Context, repoRoot string) ([]Worktree, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var worktrees []Worktree
	lines := strings.Split(string(output), "\n")
	var current Worktree
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			current = Worktree{Path: path}
		} else if strings.HasPrefix(line, "HEAD ") {
			// Extract commit hash from HEAD
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			// Extract branch name
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			current.Branch = branch
		} else if line == "" && current.Path != "" {
			// Only include cli_mate worktrees
			if strings.Contains(current.Path, ".cli_mate/worktrees") {
				current.Name = filepath.Base(current.Path)
				worktrees = append(worktrees, current)
			}
			current = Worktree{}
		}
	}
	// Don't forget the last worktree
	if current.Path != "" && strings.Contains(current.Path, ".cli_mate/worktrees") {
		current.Name = filepath.Base(current.Path)
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// Remove deletes a git worktree.
func Remove(ctx context.Context, repoRoot string, name string) error {
	worktreesDir := filepath.Join(repoRoot, ".cli_mate", "worktrees")
	worktreePath := filepath.Join(worktreesDir, name)

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// Cleanup removes all cli_mate worktrees.
func Cleanup(ctx context.Context, repoRoot string) (int, error) {
	worktrees, err := List(ctx, repoRoot)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, wt := range worktrees {
		if err := Remove(ctx, repoRoot, wt.Name); err == nil {
			removed++
		}
	}

	return removed, nil
}

// FormatWorktree returns a human-readable summary.
func FormatWorktree(wt Worktree) string {
	status := ""
	if wt.GitStatus == "dirty" {
		status = " (modified)"
	}
	branch := ""
	if wt.Branch != "" {
		branch = " [" + wt.Branch + "]"
	}
	return fmt.Sprintf("%s%s -> %s%s", wt.Name, branch, wt.Path, status)
}
