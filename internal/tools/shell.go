package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ShellTool struct {
	Timeout time.Duration
	Root    string
}

func NewShellTool(root string, timeout time.Duration) *ShellTool {
	return &ShellTool{Root: root, Timeout: timeout}
}

func (t *ShellTool) Name() string {
	return "shell"
}

func (t *ShellTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Run a non-destructive shell command inside the workspace. Use for inspection, formatting, tests, and builds.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"command"},
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to run. Uses 'sh -c' on Linux/macOS and 'powershell -NoProfile -Command' on Windows. Use for tests, builds, formatting, and inspection. Destructive commands (rm -rf, shutdown, etc.) are blocked.",
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory relative to workspace root (default: workspace root)",
				},
			},
			"description": "Run a non-destructive shell command. Use for go fmt, go test, go build, go vet, and other inspection/verification commands. On Windows uses PowerShell, on Unix uses sh.",
		},
	}
}

func (t *ShellTool) Execute(ctx context.Context, call Call) (Result, error) {
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command, _ := call.Argument["command"].(string)
	workdir, _ := call.Argument["workdir"].(string)
	if strings.TrimSpace(command) == "" {
		err := fmt.Errorf("command is required")
		return Result{Error: err.Error()}, err
	}
	if err := rejectDangerousCommand(command); err != nil {
		return Result{Error: err.Error()}, err
	}
	if strings.TrimSpace(workdir) == "" {
		workdir = "."
	}
	resolvedWorkdir, err := resolveWorkspacePath(t.Root, workdir)
	if err != nil {
		return Result{Error: err.Error()}, err
	}
	if err := ensureExistingPathInWorkspace(t.Root, resolvedWorkdir); err != nil {
		return Result{Error: err.Error()}, err
	}

	name, args := shellCommand(command)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = resolvedWorkdir

	output := newLimitedBuffer(maxToolOutputBytes)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("command timed out after %s", timeout)
		}
		return Result{Content: output.String(), Error: err.Error()}, err
	}
	return Result{Content: output.String()}, nil
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

func rejectDangerousCommand(command string) error {
	lowered := strings.ToLower(command)
	blocked := []string{
		"rm -rf",
		"remove-item -recurse",
		"remove-item -r",
		"del /s",
		"rmdir /s",
		"format ",
		"mkfs",
		"diskpart",
		"shutdown",
		"restart-computer",
	}
	for _, pattern := range blocked {
		if strings.Contains(lowered, pattern) {
			return fmt.Errorf("refusing potentially destructive command containing %q", pattern)
		}
	}
	return nil
}
