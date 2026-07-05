package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"cli_mate/internal/specmode"
)

type PlanMode struct {
	Active    bool
	Plan      []PlanStep
	ReadOnly  bool
	SpecStore *specmode.Store
}

type PlanStep struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func NewPlanMode() *PlanMode {
	return &PlanMode{}
}

func (pm *PlanMode) SetSpecStore(store *specmode.Store) {
	pm.SpecStore = store
}

type EnterPlanModeTool struct {
	PlanMode *PlanMode
}

func NewEnterPlanModeTool(pm *PlanMode) *EnterPlanModeTool {
	return &EnterPlanModeTool{PlanMode: pm}
}

func (t *EnterPlanModeTool) Name() string {
	return "enter_plan_mode"
}

func (t *EnterPlanModeTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Enter plan mode to plan changes before executing them. In plan mode, you can only read files and analyze the codebase.",
		Schema: map[string]any{
			"type":        "object",
			"properties":  map[string]any{},
			"description": "Enter plan mode. You will be in read-only mode until you call exit_plan_mode.",
		},
	}
}

func (t *EnterPlanModeTool) Execute(_ context.Context, call Call) (Result, error) {
	if t.PlanMode.Active {
		return Result{Content: "Already in plan mode."}, nil
	}
	t.PlanMode.Active = true
	t.PlanMode.ReadOnly = true
	t.PlanMode.Plan = nil
	return Result{Content: "Entered plan mode. You can now read files and analyze the codebase. Use exit_plan_mode when ready to execute."}, nil
}

type ExitPlanModeTool struct {
	PlanMode *PlanMode
}

func NewExitPlanModeTool(pm *PlanMode) *ExitPlanModeTool {
	return &ExitPlanModeTool{PlanMode: pm}
}

func (t *ExitPlanModeTool) Name() string {
	return "exit_plan_mode"
}

func (t *ExitPlanModeTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Exit plan mode and return to normal execution mode.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"description": map[string]any{
								"type":        "string",
								"description": "Description of the step",
							},
						},
						"required": []string{"description"},
					},
					"description": "The plan steps to save before exiting",
				},
			},
			"description": "Exit plan mode and return to normal execution.",
		},
	}
}

func (t *ExitPlanModeTool) Execute(_ context.Context, call Call) (Result, error) {
	if !t.PlanMode.Active {
		return Result{Content: "Not in plan mode."}, nil
	}

	if planRaw, ok := call.Argument["plan"]; ok {
		planJSON, err := json.Marshal(planRaw)
		if err == nil {
			var steps []PlanStep
			if err := json.Unmarshal(planJSON, &steps); err == nil {
				t.PlanMode.Plan = steps
			}
		}
	}

	t.PlanMode.Active = false
	t.PlanMode.ReadOnly = false

	var b strings.Builder
	b.WriteString("Exited plan mode.\n\n")

	if len(t.PlanMode.Plan) > 0 {
		b.WriteString("Plan:\n")
		for i, step := range t.PlanMode.Plan {
			fmt.Fprintf(&b, "%d. %s\n", i+1, step.Description)
		}

		// Save as spec if spec store is available
		if t.PlanMode.SpecStore != nil {
			spec := specmode.Spec{
				ID:    fmt.Sprintf("spec_%d", len(t.PlanMode.Plan)),
				Title: "Plan from session",
				Steps: make([]specmode.Step, len(t.PlanMode.Plan)),
			}
			for i, step := range t.PlanMode.Plan {
				spec.Steps[i] = specmode.Step{
					Title:       step.Description,
					Description: step.Description,
					Status:      specmode.StatusPending,
				}
			}
			if err := t.PlanMode.SpecStore.Save(spec); err == nil {
				b.WriteString(fmt.Sprintf("\nSpec saved: %s\n", spec.ID))
			}
		}
	}

	return Result{Content: b.String()}, nil
}

type VerifyPlanExecutionTool struct {
	PlanMode *PlanMode
}

func NewVerifyPlanExecutionTool(pm *PlanMode) *VerifyPlanExecutionTool {
	return &VerifyPlanExecutionTool{PlanMode: pm}
}

func (t *VerifyPlanExecutionTool) Name() string {
	return "verify_plan_execution"
}

func (t *VerifyPlanExecutionTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Verify that the plan steps have been executed correctly.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"results": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"step_id": map[string]any{
								"type":        "integer",
								"description": "ID of the plan step",
							},
							"status": map[string]any{
								"type":        "string",
								"description": "Status: completed, failed, or skipped",
							},
						},
						"required": []string{"step_id", "status"},
					},
					"description": "Results for each plan step",
				},
			},
			"description": "Verify plan execution results",
		},
	}
}

func (t *VerifyPlanExecutionTool) Execute(_ context.Context, call Call) (Result, error) {
	if len(t.PlanMode.Plan) == 0 {
		return Result{Content: "No plan to verify."}, nil
	}

	var b strings.Builder
	b.WriteString("Plan Verification:\n\n")
	for _, step := range t.PlanMode.Plan {
		status := "[pending]"
		switch step.Status {
		case "completed":
			status = "[done]"
		case "failed":
			status = "[FAILED]"
		case "skipped":
			status = "[skipped]"
		}
		fmt.Fprintf(&b, "%s Step %d: %s\n", status, step.ID, step.Description)
	}

	return Result{Content: b.String()}, nil
}
