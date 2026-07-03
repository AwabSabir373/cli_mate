package tools

import (
	"context"
	"fmt"
)

// EscalateModelTool allows the agent to switch to a stronger model mid-run
// when the current model is struggling with a task.
type EscalateModelTool struct {
	callback func(model string)
}

// NewEscalateModelTool creates an escalate model tool with the given callback.
func NewEscalateModelTool(callback func(model string)) *EscalateModelTool {
	return &EscalateModelTool{callback: callback}
}

func (t *EscalateModelTool) Name() string {
	return "escalate_model"
}

func (t *EscalateModelTool) Definition() Definition {
	return Definition{
		Name:        "escalate_model",
		Description: "Switch to a stronger model when the current one is struggling. Use this when you're stuck or making repeated errors.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"model", "reason"},
			"properties": map[string]any{
				"model": map[string]any{
					"type":        "string",
					"description": "The stronger model to switch to (e.g., 'gpt-4', 'claude-sonnet-4-20250514')",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Why you need to escalate (e.g., 'repeated failures', 'complex reasoning needed')",
				},
			},
		},
	}
}

func (t *EscalateModelTool) Execute(ctx context.Context, call Call) (Result, error) {
	model, _ := call.Argument["model"].(string)
	reason, _ := call.Argument["reason"].(string)

	if model == "" {
		return Result{Error: "model is required"}, nil
	}
	if reason == "" {
		return Result{Error: "reason is required"}, nil
	}

	// Call the callback to switch models
	if t.callback != nil {
		t.callback(model)
	}

	return Result{
		Content:        fmt.Sprintf("Model escalated to %s. Reason: %s. The next response will use the new model.", model, reason),
		RequestedModel: model,
	}, nil
}
