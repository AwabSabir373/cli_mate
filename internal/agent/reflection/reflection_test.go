package reflection

import (
	"context"
	"testing"

	"cli_mate/internal/agent/agentloop"
)

func TestReflectionStopsAfterTaskRetryBudget(t *testing.T) {
	state := agentloop.NewRunState("run-1", "fix tests", 10)
	task := &agentloop.TaskNode{ID: "task-1", Attempts: 3}
	state.Plan = &agentloop.TaskGraph{RootID: "task-1", Nodes: map[string]*agentloop.TaskNode{"task-1": task}, Order: []string{"task-1"}}
	state.ActiveTaskID = "task-1"

	report := New().Reflect(context.Background(), state, agentloop.VerificationResult{
		Status:  agentloop.VerificationFailed,
		Summary: "go test failed",
	})
	if !report.StopRecommended {
		t.Fatal("expected stop recommendation after retry budget")
	}
}
