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

func TestHeuristicPlannerReplanCreatesDiagnosticRecoveryTask(t *testing.T) {
	state := agentloop.NewRunState("run-1", "fix failing verification", 10)
	graph, err := NewHeuristicPlanner().Replan(context.Background(), state, agentloop.ReflectionReport{
		FailureReason: "go test: exit status 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if graph.Empty() {
		t.Fatal("expected non-empty task graph")
	}
	recovery := graph.Nodes[graph.Order[0]]
	if recovery == nil {
		t.Fatal("expected first task to be recovery task")
	}
	if recovery.Risk != agentloop.RiskReadOnly {
		t.Fatalf("expected readonly recovery task, got %s", recovery.Risk.String())
	}
	if len(recovery.Verification.Commands) != 0 {
		t.Fatalf("expected recovery task to inspect before rerunning verification, got %d commands", len(recovery.Verification.Commands))
	}
	if recovery.SuccessCriteria != "relevant files, commands, or diagnostics read and understood" {
		t.Fatalf("expected diagnostic success criteria, got %q", recovery.SuccessCriteria)
	}
}
