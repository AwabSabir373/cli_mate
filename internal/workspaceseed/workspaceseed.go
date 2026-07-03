// Package workspaceseed builds a compact, deterministic workspace context seed
// for the agent's system prompt. It provides a directory tree summary with git
// metadata so the model knows the repo structure upfront.
package workspaceseed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultMaxLayoutEntries = 16
	defaultMaxRenderLines   = 12
	defaultRenderWidth      = 100
)

var detectedProjectFiles = []string{
	"README.md",
	"go.mod",
	"package.json",
	"AGENTS.md",
	"Cargo.toml",
	"pyproject.toml",
	"Makefile",
	"Dockerfile",
}

// Input is the pure builder input. Callers provide paths and git metadata.
type Input struct {
	CWD              string
	GitBranch        string
	GitDirty         *bool
	Paths            []string
	MaxLayoutEntries int
}

// GitInfo is metadata supplied by integration code that already knows git state.
type GitInfo struct {
	Branch string
	Dirty  *bool
}

// Seed is the compact workspace context model ready for rendering.
type Seed struct {
	CWD          string
	GitBranch    string
	GitSummary   string
	Layout       []string
	ProjectFiles []string
	Truncated    bool
}

// BuildFromWorkspace builds a Seed from the given input without shelling out.
func BuildFromWorkspace(ctx context.Context, input Input) Seed {
	seed := Seed{
		CWD:       input.CWD,
		GitBranch: input.GitBranch,
	}

	if input.GitDirty != nil {
		if *input.GitDirty {
			seed.GitSummary = input.GitBranch + "*"
		} else {
			seed.GitSummary = input.GitBranch
		}
	} else if input.GitBranch != "" {
		seed.GitSummary = input.GitBranch
	}

	maxEntries := input.MaxLayoutEntries
	if maxEntries <= 0 {
		maxEntries = defaultMaxLayoutEntries
	}

	// Build layout from paths
	if len(input.Paths) > 0 {
		seed.Layout = buildLayout(input.Paths, maxEntries)
		if len(input.Paths) > maxEntries {
			seed.Truncated = true
		}
	} else if input.CWD != "" {
		paths := listTopLevel(input.CWD)
		seed.Layout = buildLayout(paths, maxEntries)
		if len(paths) > maxEntries {
			seed.Truncated = true
		}
	}

	// Detect project files
	if input.CWD != "" {
		seed.ProjectFiles = detectProjectFiles(input.CWD)
	}

	return seed
}

// RenderOptions controls text output budgets.
type RenderOptions struct {
	MaxLines int
	Width    int
}

// Render produces a compact text representation of the seed for injection
// into the system prompt.
func (s Seed) Render(opts RenderOptions) string {
	maxLines := opts.MaxLines
	if maxLines <= 0 {
		maxLines = defaultMaxRenderLines
	}
	width := opts.Width
	if width <= 0 {
		width = defaultRenderWidth
	}

	var b strings.Builder

	// Header with cwd and git info
	if s.GitSummary != "" {
		b.WriteString(fmt.Sprintf("Workspace: %s (branch: %s)\n", s.CWD, s.GitSummary))
	} else {
		b.WriteString(fmt.Sprintf("Workspace: %s\n", s.CWD))
	}

	// Layout
	if len(s.Layout) > 0 {
		shown := s.Layout
		if len(shown) > maxLines {
			shown = shown[:maxLines]
		}
		b.WriteString("Layout:\n")
		for _, line := range shown {
			if len(line) > width {
				line = line[:width-3] + "..."
			}
			b.WriteString("  " + line + "\n")
		}
		if s.Truncated {
			b.WriteString("  ... (more files)\n")
		}
	}

	// Project files
	if len(s.ProjectFiles) > 0 {
		b.WriteString("Project: " + strings.Join(s.ProjectFiles, ", ") + "\n")
	}

	return b.String()
}

// buildLayout creates a tree-like layout from file paths.
func buildLayout(paths []string, maxEntries int) []string {
	// Sort for determinism
	sorted := make([]string, len(paths))
	copy(sorted, paths)
	sort.Strings(sorted)

	var layout []string
	for _, p := range sorted {
		if len(layout) >= maxEntries {
			break
		}
		layout = append(layout, p)
	}
	return layout
}

// listTopLevel lists files and directories at the top level of a directory.
func listTopLevel(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var paths []string
	skipped := map[string]bool{
		".git":          true,
		".idea":         true,
		".vscode":       true,
		"node_modules":  true,
		"vendor":        true,
		".openclaude":   true,
		".cli_mate":     true,
		".mimocode":     true,
		".claude":       true,
		".codex":        true,
		".opencode":     true,
		".agents":       true,
	}

	for _, entry := range entries {
		name := entry.Name()
		if skipped[name] {
			continue
		}
		if entry.IsDir() {
			paths = append(paths, name+"/")
		} else {
			paths = append(paths, name)
		}
	}
	return paths
}

// detectProjectFiles checks which common project files exist.
func detectProjectFiles(dir string) []string {
	var found []string
	for _, name := range detectedProjectFiles {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, name)
		}
	}
	return found
}
