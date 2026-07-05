package verification

import (
	"context"
	"testing"

	"cli_mate/internal/agent/agentloop"
)

type recordingRunner struct {
	checks []agentloop.CommandSpec
}

func (r *recordingRunner) Run(_ context.Context, spec agentloop.CommandSpec) agentloop.CheckResult {
	r.checks = append(r.checks, spec)
	return agentloop.CheckResult{Name: spec.Name, Command: spec.Command, Passed: true}
}

func TestVerifierRunsRequiredCommands(t *testing.T) {
	runner := &recordingRunner{}
	engine := &Engine{Runner: runner}
	result := engine.Verify(context.Background(), agentloop.VerificationSpec{
		Commands: []agentloop.CommandSpec{
			{Name: "go test", Command: "go test ./...", Required: true},
			{Name: "go vet", Command: "go vet ./...", Required: true},
		},
	})
	if result.Status != agentloop.VerificationPassed {
		t.Fatalf("expected pass, got %s", result.Status)
	}
	if len(runner.checks) != 2 {
		t.Fatalf("expected two commands, got %d", len(runner.checks))
	}
}
