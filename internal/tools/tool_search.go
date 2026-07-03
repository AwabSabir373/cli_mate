package tools

import (
	"context"
	"fmt"
	"strings"
)

// ToolSearchTool allows the model to discover tools by keyword when
// too many tools are registered and full schemas are withheld.
type ToolSearchTool struct {
	registry *ToolRegistry
}

func NewToolSearchTool(registry *ToolRegistry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t *ToolSearchTool) Name() string {
	return "tool_search"
}

func (t *ToolSearchTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Search for available tools by keyword. Use when you need to find a tool but don't see it in your available tools list.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search keyword to find matching tools (e.g. 'file', 'edit', 'search', 'web')",
				},
			},
			"description": "Search for tools by keyword. Returns matching tool names and descriptions.",
		},
	}
}

func (t *ToolSearchTool) Execute(_ context.Context, call Call) (Result, error) {
	query, _ := call.Argument["query"].(string)
	if strings.TrimSpace(query) == "" {
		return Result{Error: "query is required"}, fmt.Errorf("query is required")
	}

	query = strings.ToLower(strings.TrimSpace(query))
	matches := t.registry.Search(query)

	if len(matches) == 0 {
		return Result{Content: fmt.Sprintf("No tools found matching %q. Available tools: %s", query, t.registry.ListNames())}, nil
	}

	var loaded []string
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d tool(s) matching %q:\n\n", len(matches), query))
	for _, tool := range matches {
		def := tool.Definition()
		b.WriteString(fmt.Sprintf("- **%s**: %s\n", def.Name, def.Description))
		loaded = append(loaded, def.Name)
	}

	return Result{Content: b.String(), LoadedTools: loaded}, nil
}
