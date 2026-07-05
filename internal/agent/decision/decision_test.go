package decision

import (
	"context"
	"testing"

	"cli_mate/internal/agent/agentloop"
	"cli_mate/internal/agent/toolrouter"
	"cli_mate/internal/tools"
)

type fakeTool struct {
	name string
	desc string
}

func (t fakeTool) Name() string { return t.name }
func (t fakeTool) Definition() tools.Definition {
	return tools.Definition{Name: t.name, Description: t.desc}
}
func (t fakeTool) Execute(context.Context, tools.Call) (tools.Result, error) {
	return tools.Result{Content: "ok"}, nil
}

func TestDecisionRequestsContextWhenEvidenceMissing(t *testing.T) {
	router := toolrouter.New()
	router.Register(fakeTool{name: "grep", desc: "search file contents"})
	task := &agentloop.TaskNode{
		ID:     "task-1",
		Goal:   "Inspect relevant files",
		Status: agentloop.TaskRunning,
		Risk:   agentloop.RiskReadOnly,
	}
	state := agentloop.NewRunState("run-1", "find the bug", 10)
	state.Plan = &agentloop.TaskGraph{RootID: "task-1", Nodes: map[string]*agentloop.TaskNode{"task-1": task}, Order: []string{"task-1"}}
	state.ActiveTaskID = "task-1"

	decision, err := New(router).Decide(context.Background(), state, agentloop.ContextBundle{UserRequest: state.UserRequest, ActiveTask: task})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.NeedMoreContext {
		t.Fatal("expected decision to request more context")
	}
	if decision.SelectedAction.Tool.Name != "grep" {
		t.Fatalf("expected grep selection, got %q", decision.SelectedAction.Tool.Name)
	}
}
