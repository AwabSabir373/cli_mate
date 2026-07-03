// Package specialist provides sub-agent delegation for the main agent loop.
// Specialists are focused agents with restricted tool access that handle
// specific types of work (exploration, code review, general tasks).
package specialist

import (
	"fmt"
	"strings"
)

// Location indicates where a specialist manifest was loaded from.
type Location string

const (
	LocationBuiltin Location = "builtin"
	LocationProject Location = "project"
	LocationUser    Location = "user"
)

// Metadata describes a specialist's identity and capabilities.
type Metadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"` // tool categories: "read-only", "edit", "execute", "plan"
}

// Manifest is a complete specialist definition.
type Manifest struct {
	Metadata     Metadata `json:"metadata"`
	SystemPrompt string   `json:"systemPrompt"`
	Location     Location `json:"location"`
	FilePath     string   `json:"filePath"`
}

// Validate checks that a manifest has the required fields.
func Validate(m *Manifest) error {
	if m.Metadata.Name == "" {
		return fmt.Errorf("specialist name is required")
	}
	if m.SystemPrompt == "" {
		return fmt.Errorf("specialist %q: system prompt is required", m.Metadata.Name)
	}
	return nil
}

// ToolCategories returns the set of tool categories this specialist can use.
func (m *Manifest) ToolCategories() map[string]bool {
	categories := make(map[string]bool)
	for _, tool := range m.Metadata.Tools {
		categories[strings.ToLower(tool)] = true
	}
	return categories
}

// CanUseTool reports whether the specialist is allowed to use a tool with the
// given category. Read-only specialists can only use "read-only" tools.
func (m *Manifest) CanUseTool(category string) bool {
	categories := m.ToolCategories()
	if categories["all"] {
		return true
	}
	return categories[category]
}
