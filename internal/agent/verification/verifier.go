package verification

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"cli_mate/internal/agent/agentloop"
)

type CommandRunner interface {
	Run(context.Context, agentloop.CommandSpec) agentloop.CheckResult
}

type Engine struct {
	WorkspaceRoot string
	Runner        CommandRunner
}

func New(workspaceRoot string) *Engine {
	return &Engine{
		WorkspaceRoot: workspaceRoot,
		Runner:        OSRunner{WorkspaceRoot: workspaceRoot},
	}
}

func (e *Engine) Verify(ctx context.Context, spec agentloop.VerificationSpec) agentloop.VerificationResult {
	started := time.Now().UTC()
	if len(spec.Commands) == 0 && len(spec.FileChecks) == 0 && len(spec.DiffChecks) == 0 {
		return agentloop.VerificationResult{
			Status:   agentloop.VerificationSkipped,
			Started:  started,
			Finished: time.Now().UTC(),
			Summary:  "no verification checks requested",
		}
	}
	runner := e.Runner
	if runner == nil {
		runner = OSRunner{WorkspaceRoot: e.WorkspaceRoot}
	}

	result := agentloop.VerificationResult{
		Status:  agentloop.VerificationPassed,
		Started: started,
	}
	for _, command := range spec.Commands {
		check := runner.Run(ctx, command)
		result.Checks = append(result.Checks, check)
		result.Evidence = append(result.Evidence, agentloop.EvidenceRef{
			Kind:    "command",
			Source:  command.Command,
			Summary: checkSummary(check),
		})
		if command.Required && !check.Passed {
			result.Status = agentloop.VerificationFailed
		}
	}
	result.Finished = time.Now().UTC()
	result.Summary = summarize(result)
	return result
}

type OSRunner struct {
	WorkspaceRoot string
}

func (r OSRunner) Run(ctx context.Context, spec agentloop.CommandSpec) agentloop.CheckResult {
	started := time.Now()
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name, args := shellCommand(spec.Command)
	cmd := exec.CommandContext(runCtx, name, args...)
	if strings.TrimSpace(spec.Workdir) != "" {
		cmd.Dir = spec.Workdir
	} else {
		cmd.Dir = r.WorkspaceRoot
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	check := agentloop.CheckResult{
		Name:     spec.Name,
		Command:  spec.Command,
		Passed:   err == nil,
		Output:   limit(output.String(), 12000),
		Duration: time.Since(started),
	}
	if err != nil {
		if runCtx.Err() != nil {
			check.Error = fmt.Sprintf("command timed out after %s", timeout)
		} else {
			check.Error = err.Error()
		}
	}
	return check
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		command = windowsCommand(command)
		return "powershell", []string{"-NoProfile", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

func windowsCommand(command string) string {
	if command == "gofmt -w $(git ls-files '*.go')" {
		return "$files = git ls-files '*.go'; if ($files) { gofmt -w $files }"
	}
	return command
}

func summarize(result agentloop.VerificationResult) string {
	if result.Status == agentloop.VerificationPassed {
		return "all required verification checks passed"
	}
	var failed []string
	for _, check := range result.Checks {
		if !check.Passed {
			failed = append(failed, check.Name+": "+strings.TrimSpace(check.Error))
		}
	}
	if len(failed) == 0 {
		return string(result.Status)
	}
	return strings.Join(failed, "; ")
}

func checkSummary(check agentloop.CheckResult) string {
	if check.Passed {
		return check.Name + " passed"
	}
	return check.Name + " failed: " + strings.TrimSpace(check.Error)
}

func limit(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	return text[:maxBytes] + "\n... truncated ..."
}
