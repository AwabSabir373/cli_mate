package planner

import (
	"context"
	"strings"
	"time"

	"cli_mate/internal/agent/agentloop"
)

type Planner interface {
	Plan(context.Context, agentloop.ContextBundle) (*agentloop.TaskGraph, error)
	Replan(context.Context, *agentloop.RunState, agentloop.ReflectionReport) (*agentloop.TaskGraph, error)
}

type HeuristicPlanner struct {
	MaxAttempts int
}

func NewHeuristicPlanner() *HeuristicPlanner {
	return &HeuristicPlanner{MaxAttempts: 3}
}

func (p *HeuristicPlanner) Plan(_ context.Context, bundle agentloop.ContextBundle) (*agentloop.TaskGraph, error) {
	now := time.Now().UTC()
	maxAttempts := p.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	root := &agentloop.TaskNode{
		ID:          agentloop.NewID("task"),
		Goal:        strings.TrimSpace(bundle.UserRequest),
		Status:      agentloop.TaskPending,
		Priority:    100,
		Risk:        agentloop.RiskReadOnly,
		Confidence:  0.3,
		MaxAttempts: maxAttempts,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if root.Goal == "" {
		root.Goal = "Complete the user's coding task"
	}

	inspect := child(root.ID, "Inspect relevant files, commands, diagnostics, and project conventions.", 90, agentloop.RiskReadOnly, maxAttempts, now)
	plan := child(root.ID, "Create or update the task graph with explicit verification before edits.", 80, agentloop.RiskReadOnly, maxAttempts, now)
	implement := child(root.ID, "Apply the smallest correct code or configuration changes.", 70, agentloop.RiskLocalEdit, maxAttempts, now)
	verify := child(root.ID, "Run required formatters, tests, vet, build, and integrity checks.", 60, agentloop.RiskCommand, maxAttempts, now)
	summarize := child(root.ID, "Summarize completed work, verification evidence, and residual risk.", 50, agentloop.RiskReadOnly, maxAttempts, now)

	plan.Dependencies = []string{inspect.ID}
	implement.Dependencies = []string{plan.ID}
	verify.Dependencies = []string{implement.ID}
	summarize.Dependencies = []string{verify.ID}
	verify.Verification = DefaultGoVerificationSpec()

	nodes := map[string]*agentloop.TaskNode{
		root.ID:      root,
		inspect.ID:   inspect,
		plan.ID:      plan,
		implement.ID: implement,
		verify.ID:    verify,
		summarize.ID: summarize,
	}
	return &agentloop.TaskGraph{
		RootID: root.ID,
		Nodes:  nodes,
		Order:  []string{inspect.ID, plan.ID, implement.ID, verify.ID, summarize.ID, root.ID},
	}, nil
}

func (p *HeuristicPlanner) Replan(ctx context.Context, state *agentloop.RunState, report agentloop.ReflectionReport) (*agentloop.TaskGraph, error) {
	bundle := agentloop.ContextBundle{UserRequest: state.UserRequest, ActiveTask: state.ActiveTask(), RecentEvents: state.Events}
	graph, err := p.Plan(ctx, bundle)
	if err != nil {
		return nil, err
	}
	if report.FailureReason != "" {
		recovery := child(graph.RootID, "Recover from failure: "+report.FailureReason, 95, agentloop.RiskReadOnly, p.MaxAttempts, time.Now().UTC())
		recovery.Verification = DefaultGoVerificationSpec()
		graph.Nodes[recovery.ID] = recovery
		graph.Order = append([]string{recovery.ID}, graph.Order...)
	}
	return graph, nil
}

func child(parentID, goal string, priority int, risk agentloop.RiskLevel, maxAttempts int, now time.Time) *agentloop.TaskNode {
	return &agentloop.TaskNode{
		ID:          agentloop.NewID("task"),
		ParentID:    parentID,
		Goal:        goal,
		Status:      agentloop.TaskPending,
		Priority:    priority,
		Risk:        risk,
		Confidence:  0.4,
		MaxAttempts: maxAttempts,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func DefaultGoVerificationSpec() agentloop.VerificationSpec {
	return agentloop.VerificationSpec{
		Commands: []agentloop.CommandSpec{
			{
				Name:        "gofmt",
				Command:     gofmtCommand(),
				Timeout:     60 * time.Second,
				Required:    true,
				Mutates:     true,
				Retryable:   true,
				FailureHint: "Go formatting failed; inspect malformed Go files before retrying.",
			},
			{
				Name:        "go test",
				Command:     "go test ./...",
				Timeout:     5 * time.Minute,
				Required:    true,
				Retryable:   true,
				FailureHint: "Tests failed; prefer targeted package fixes before rerunning all tests.",
			},
			{
				Name:        "go vet",
				Command:     "go vet ./...",
				Timeout:     3 * time.Minute,
				Required:    true,
				Retryable:   true,
				FailureHint: "Vet found suspicious code; inspect diagnostics and fix the smallest cause.",
			},
			{
				Name:        "go build",
				Command:     "go build ./cmd/cli_mate",
				Timeout:     3 * time.Minute,
				Required:    true,
				Retryable:   true,
				FailureHint: "Build failed; use compiler diagnostics as primary evidence.",
			},
		},
		DiffChecks: []agentloop.DiffCheck{
			{Description: "Ensure the final diff is scoped to the requested task."},
			{Description: "Ensure no unrelated user changes were reverted."},
		},
		RequiredPasses: []string{"gofmt", "go test", "go vet", "go build"},
	}
}

func gofmtCommand() string {
	return "gofmt -w $(git ls-files '*.go')"
}
