package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Path        string `json:"path"`
}

type SkillTool struct {
	WorkspaceRoot string
	SkillsDir     string
}

func NewSkillTool(workspaceRoot string) *SkillTool {
	skillsDir := filepath.Join(workspaceRoot, ".cli_mate", "skills")
	return &SkillTool{
		WorkspaceRoot: workspaceRoot,
		SkillsDir:     skillsDir,
	}
}

func (t *SkillTool) Name() string {
	return "skill"
}

func (t *SkillTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Load a skill by name to get its full instructions. Skills provide reusable behaviors and domain-specific knowledge that you can use to complete tasks.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the skill to load",
				},
			},
			"description": "Load a skill by name to get its full instructions.",
		},
	}
}

func (t *SkillTool) Execute(_ context.Context, call Call) (Result, error) {
	name, _ := call.Argument["name"].(string)
	if strings.TrimSpace(name) == "" {
		return Result{Error: "name is required"}, fmt.Errorf("name is required")
	}

	// Look for skill file
	skillPath := filepath.Join(t.SkillsDir, name+".md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		// Try .txt extension
		skillPath = filepath.Join(t.SkillsDir, name+".txt")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			return Result{Error: fmt.Sprintf("skill %q not found", name)}, fmt.Errorf("skill %q not found", name)
		}
	}

	content, err := os.ReadFile(skillPath)
	if err != nil {
		return Result{Error: fmt.Sprintf("failed to read skill: %v", err)}, err
	}

	return Result{Content: string(content)}, nil
}

type DiscoverSkillsTool struct {
	WorkspaceRoot string
	SkillsDir     string
}

func NewDiscoverSkillsTool(workspaceRoot string) *DiscoverSkillsTool {
	skillsDir := filepath.Join(workspaceRoot, ".cli_mate", "skills")
	return &DiscoverSkillsTool{
		WorkspaceRoot: workspaceRoot,
		SkillsDir:     skillsDir,
	}
}

func (t *DiscoverSkillsTool) Name() string {
	return "discover_skills"
}

func (t *DiscoverSkillsTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "List available skills in the workspace. Skills are stored in .cli_mate/skills/ directory.",
		Schema: map[string]any{
			"type":        "object",
			"properties":  map[string]any{},
			"description": "Discover available skills in the workspace.",
		},
	}
}

func (t *DiscoverSkillsTool) Execute(_ context.Context, call Call) (Result, error) {
	if _, err := os.Stat(t.SkillsDir); os.IsNotExist(err) {
		return Result{Content: "No skills directory found. Create .cli_mate/skills/ to add skills."}, nil
	}

	entries, err := os.ReadDir(t.SkillsDir)
	if err != nil {
		return Result{Error: fmt.Sprintf("failed to read skills directory: %v", err)}, err
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Remove extension
		if idx := strings.LastIndex(name, "."); idx > 0 {
			name = name[:idx]
		}

		content, err := os.ReadFile(filepath.Join(t.SkillsDir, entry.Name()))
		if err != nil {
			continue
		}

		// Extract description from first line or first paragraph
		description := ""
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				description = trimmed
				if len(description) > 100 {
					description = description[:100] + "..."
				}
				break
			}
		}

		skills = append(skills, Skill{
			Name:        name,
			Description: description,
			Path:        filepath.Join(t.SkillsDir, entry.Name()),
		})
	}

	if len(skills) == 0 {
		return Result{Content: "No skills found in .cli_mate/skills/"}, nil
	}

	var b strings.Builder
	b.WriteString("Available Skills:\n\n")
	for _, skill := range skills {
		fmt.Fprintf(&b, "- %s: %s\n", skill.Name, skill.Description)
	}
	b.WriteString("\nUse the skill tool to load a skill by name.")

	return Result{Content: b.String()}, nil
}
