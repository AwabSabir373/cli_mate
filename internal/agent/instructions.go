package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cli_mate/internal/repomap"
	"cli_mate/internal/workspaceseed"
)

var defaultInstructionFiles = []string{
	"AGENTS.md",
	filepath.ToSlash(filepath.Join("docs", "ai", "system.md")),
	filepath.ToSlash(filepath.Join("docs", "ai", "repository.md")),
	filepath.ToSlash(filepath.Join("docs", "ai", "tools.md")),
	filepath.ToSlash(filepath.Join("docs", "ai", "models.md")),
	filepath.ToSlash(filepath.Join("docs", "ai", "review.md")),
	filepath.ToSlash(filepath.Join("docs", "ai", "skills.md")),
	filepath.ToSlash(filepath.Join("docs", "ai", "prompts.md")),
}

func LoadInstructions(ctx context.Context, root string) (string, error) {
	var sections []string
	for _, name := range defaultInstructionFiles {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", err
		}

		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}
		sections = append(sections, "## "+name+"\n\n"+trimmed)
	}

	return strings.Join(sections, "\n\n---\n\n"), nil
}

// BuildWorkspaceContext creates a workspace seed and repo map, rendering them
// for injection into the system prompt. Returns empty string if root is empty.
func BuildWorkspaceContext(root string) string {
	if root == "" {
		return ""
	}

	var b strings.Builder

	// Get git branch if in a git repo
	gitBranch := detectGitBranch(root)

	// Check if working tree is dirty
	gitDirty := detectGitDirty(root)

	input := workspaceseed.Input{
		CWD:       root,
		GitBranch: gitBranch,
		GitDirty:  gitDirty,
	}

	seed := workspaceseed.BuildFromWorkspace(context.Background(), input)
	seedRendered := seed.Render(workspaceseed.RenderOptions{})
	if seedRendered != "" {
		b.WriteString(seedRendered)
	}

	// Add repo map
	repoMap := repomap.Build(root, 5)
	mapRendered := repoMap.Render(8)
	if mapRendered != "" {
		b.WriteString("\n")
		b.WriteString(mapRendered)
	}

	return b.String()
}

// detectGitBranch runs git rev-parse to get the current branch name.
func detectGitBranch(root string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectGitDirty checks if the working tree has uncommitted changes.
func detectGitDirty(root string) *bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	dirty := len(strings.TrimSpace(string(out))) > 0
	return &dirty
}
