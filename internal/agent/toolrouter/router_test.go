package toolrouter

import (
	"context"
	"testing"

	"cli_mate/internal/agent/agentloop"
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
	return tools.Result{Content: t.name + " ok"}, nil
}

func TestRouterSelectsMatchingCapability(t *testing.T) {
	router := New()
	router.Register(fakeTool{name: "grep", desc: "search file contents"})
	router.Register(fakeTool{name: "shell", desc: "run commands"})

	candidates, err := router.Select(context.Background(), agentloop.ToolIntent{
		Kind:       agentloop.ToolFilesystem,
		Capability: "search",
		MaxRisk:    agentloop.RiskCommand,
	}, agentloop.RouteOptions{MaxCandidates: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected one candidate, got %d", len(candidates))
	}
	if candidates[0].Tool.Name != "grep" {
		t.Fatalf("expected grep, got %s", candidates[0].Tool.Name)
	}
}
