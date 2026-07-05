package planner

import (
	"context"
	"testing"

	"cli_mate/internal/agent/agentloop"
)

func TestHeuristicPlannerCreatesVerificationTask(t *testing.T) {
	graph, err := NewHeuristicPlanner().Plan(context.Background(), agentloop.ContextBundle{
		UserRequest: "add an agent loop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if graph.Empty() {
		t.Fatal("expected non-empty task graph")
	}
	var verify *agentloop.TaskNode
	for _, node := range graph.Nodes {
		if len(node.Verification.Commands) > 0 {
			verify = node
			break
		}
	}
	if verify == nil {
		t.Fatal("expected verification task")
	}
	if got := len(verify.Verification.Commands); got != 4 {
		t.Fatalf("expected 4 verification commands, got %d", got)
	}
	if verify.Verification.RequiredPasses[0] != "gofmt" {
		t.Fatalf("expected gofmt to be required first, got %q", verify.Verification.RequiredPasses[0])
	}
}
