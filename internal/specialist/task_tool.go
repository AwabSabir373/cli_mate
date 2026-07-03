package specialist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cli_mate/internal/providers"
	"cli_mate/internal/tools"
)

// TaskTool allows the main agent to delegate work to a specialist sub-agent.
type TaskTool struct {
	registry *Registry
	provider providers.Provider
	tools    []tools.Tool
}

// NewTaskTool creates a task tool with the given specialist registry.
func NewTaskTool(registry *Registry, provider providers.Provider, availableTools []tools.Tool) *TaskTool {
	return &TaskTool{
		registry: registry,
		provider: provider,
		tools:    availableTools,
	}
}

func (t *TaskTool) Name() string {
	return "task"
}

func (t *TaskTool) Definition() tools.Definition {
	return tools.Definition{
		Name:        "task",
		Description: "Delegate a focused task to a specialist sub-agent. Use 'explorer' for read-only codebase search, 'code-review' for reviewing changes, or 'worker' for general coding tasks.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"specialist", "prompt"},
			"properties": map[string]any{
				"specialist": map[string]any{
					"type":        "string",
					"description": "Specialist name: 'worker', 'explorer', or 'code-review'",
					"enum":        []string{"worker", "explorer", "code-review"},
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "The task description for the specialist",
				},
			},
		},
	}
}

func (t *TaskTool) Execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	specialistName, _ := call.Argument["specialist"].(string)
	prompt, _ := call.Argument["prompt"].(string)

	if specialistName == "" || prompt == "" {
		return tools.Result{Error: "specialist and prompt are required"}, nil
	}

	manifest := t.registry.Get(specialistName)
	if manifest == nil {
		return tools.Result{Error: fmt.Sprintf("unknown specialist %q", specialistName)}, nil
	}

	// Filter tools based on specialist's allowed categories
	filteredTools := t.filterTools(manifest)

	// Run the specialist as a sub-agent
	result, err := t.runSpecialist(ctx, manifest, prompt, filteredTools)
	if err != nil {
		return tools.Result{Error: err.Error()}, nil
	}

	return tools.Result{Content: result}, nil
}

func (t *TaskTool) filterTools(manifest *Manifest) []tools.Tool {
	categories := manifest.ToolCategories()
	if categories["all"] {
		return t.tools
	}

	var filtered []tools.Tool
	for _, tool := range t.tools {
		// Map tool names to categories
		category := toolCategory(tool.Name())
		if categories[category] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func toolCategory(name string) string {
	switch {
	case name == "file_edit" || name == "file_write" || name == "apply_patch":
		return "edit"
	case name == "shell":
		return "execute"
	case name == "enter_plan_mode" || name == "exit_plan_mode":
		return "plan"
	case name == "file_read" || name == "glob" || name == "grep" || name == "read_subtree" || name == "file_list":
		return "read-only"
	default:
		return "read-only"
	}
}

func (t *TaskTool) runSpecialist(ctx context.Context, manifest *Manifest, prompt string, toolSet []tools.Tool) (string, error) {
	// Build the specialist's system prompt with available tools
	var systemPrompt strings.Builder
	systemPrompt.WriteString(manifest.SystemPrompt)
	systemPrompt.WriteString("\n\n## Available tools\n\n")
	for _, tool := range toolSet {
		def := tool.Definition()
		systemPrompt.WriteString("- " + def.Name + ": " + def.Description + "\n")
	}
	systemPrompt.WriteString("\n## Tool call format\n\n")
	systemPrompt.WriteString("When you need to use a tool, respond with exactly one fenced JSON block:\n\n")
	systemPrompt.WriteString("```cli_mate-tool\n{\"tool\":\"tool_name\",\"arguments\":{...}}\n```\n\n")
	systemPrompt.WriteString("After receiving a tool result, continue with another tool call or provide your final answer.\n")

	// Create messages for the specialist
	messages := []providers.Message{
		{Role: "system", Content: systemPrompt.String()},
		{Role: "user", Content: prompt},
	}

	// Build tool definitions
	var toolDefs []providers.ToolDefinition
	for _, tool := range toolSet {
		def := tool.Definition()
		toolDefs = append(toolDefs, providers.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			Schema:      def.Schema,
		})
	}

	// Run up to 10 iterations
	var answer string
	for i := 0; i < 10; i++ {
		stream, err := t.provider.StreamChat(ctx, providers.ChatRequest{
			Messages: messages,
			Tools:    toolDefs,
		})
		if err != nil {
			return "", fmt.Errorf("specialist stream error: %w", err)
		}

		// Consume the stream
		var text strings.Builder
		var toolCalls []providers.ToolCall
		for event := range stream {
			if event.Err != nil {
				return "", event.Err
			}
			text.WriteString(event.Delta)
			toolCalls = append(toolCalls, event.ToolCalls...)
		}

		answer = strings.TrimSpace(text.String())

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			break
		}

		// Add assistant message with tool calls
		messages = append(messages, providers.Message{
			Role:      "assistant",
			Content:   answer,
			ToolCalls: toolCalls,
		})

		// Execute tool calls
		for _, tc := range toolCalls {
			var args map[string]any
			if tc.Arguments != "" {
				json.Unmarshal([]byte(tc.Arguments), &args)
			}

			call := tools.Call{Name: tc.Name, Argument: args}
			result := executeTool(ctx, toolSet, call)

			messages = append(messages, providers.Message{
				Role:       "tool",
				Name:       tc.Name,
				ToolCallID: tc.ID,
				Content:    result.Content,
			})
		}
	}

	if answer == "" {
		answer = "(no response from specialist)"
	}
	return answer, nil
}

func executeTool(ctx context.Context, toolSet []tools.Tool, call tools.Call) tools.Result {
	for _, tool := range toolSet {
		if tool.Name() == call.Name {
			result, err := tool.Execute(ctx, call)
			if err != nil {
				return tools.Result{Error: err.Error()}
			}
			return result
		}
	}
	return tools.Result{Error: fmt.Sprintf("unknown tool %q", call.Name)}
}
