package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cli_mate/internal/tools"
)

const (
	defaultMaxSelfCorrectAttempts = 3
	selfCorrectTimeout            = 5 * time.Minute
)

// SelfCorrector runs verification after mutating tool calls and feeds errors
// back to the model for automatic fix attempts.
type SelfCorrector struct {
	WorkspaceRoot string
	MaxAttempts   int
	enabled       bool
}

// NewSelfCorrector creates a self-corrector for the given workspace.
func NewSelfCorrector(workspaceRoot string) *SelfCorrector {
	return &SelfCorrector{
		WorkspaceRoot: workspaceRoot,
		MaxAttempts:   defaultMaxSelfCorrectAttempts,
		enabled:       true,
	}
}

// SetEnabled toggles self-correction on or off.
func (sc *SelfCorrector) SetEnabled(enabled bool) {
	sc.enabled = enabled
}

// IsEnabled returns whether self-correction is active.
func (sc *SelfCorrector) IsEnabled() bool {
	return sc.enabled
}

// isMutatingTool reports whether the tool call modifies files.
func isMutatingTool(call tools.Call) bool {
	switch call.Name {
	case "file_edit", "file_write", "apply_patch":
		return true
	}
	return false
}

// VerifyAfterMutation runs project-appropriate verification commands after a
// mutating tool call. Returns diagnostic output to feed back to the model, or
// empty string if everything passes.
func (sc *SelfCorrector) VerifyAfterMutation(ctx context.Context) (string, error) {
	if !sc.enabled || sc.WorkspaceRoot == "" {
		return "", nil
	}

	// Detect project type and run appropriate checks
	checks := sc.detectChecks()
	if len(checks) == 0 {
		return "", nil
	}

	var diagnostics []string
	for _, check := range checks {
		output, err := sc.runCheck(ctx, check)
		if err != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("%s failed: %s\n%s", check.Name, err.Error(), output))
		} else if output != "" {
			diagnostics = append(diagnostics, fmt.Sprintf("%s output:\n%s", check.Name, output))
		}
	}

	if len(diagnostics) == 0 {
		return "", nil
	}
	return strings.Join(diagnostics, "\n\n"), nil
}

type checkCommand struct {
	Name    string
	Command string
}

// detectChecks determines which verification commands to run based on project files.
func (sc *SelfCorrector) detectChecks() []checkCommand {
	root := sc.WorkspaceRoot
	var checks []checkCommand

	// Go project
	if fileExists(filepath.Join(root, "go.mod")) {
		checks = append(checks,
			checkCommand{Name: "gofmt", Command: gofmtSelfCorrectCommand()},
			checkCommand{Name: "go test", Command: "go test ./..."},
			checkCommand{Name: "go vet", Command: "go vet ./..."},
			checkCommand{Name: "go build", Command: "go build ./cmd/cli_mate"},
		)
	}

	// Node.js project
	if fileExists(filepath.Join(root, "package.json")) {
		if fileExists(filepath.Join(root, "node_modules")) {
			// Check if lint script exists
			if sc.hasNpmScript(root, "lint") {
				checks = append(checks, checkCommand{Name: "npm lint", Command: "npm run lint --if-present"})
			}
			if sc.hasNpmScript(root, "typecheck") || sc.hasNpmScript(root, "type-check") {
				checks = append(checks, checkCommand{Name: "typecheck", Command: "npm run typecheck --if-present || npm run type-check --if-present"})
			}
		}
	}

	// Python project
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "setup.py")) {
		checks = append(checks, checkCommand{Name: "python syntax", Command: "python -m py_compile $(find . -name '*.py' -not -path './node_modules/*' -not -path './.git/*' | head -20)"})
	}

	// Rust project
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		checks = append(checks,
			checkCommand{Name: "cargo check", Command: "cargo check"},
		)
	}

	return checks
}

// hasNpmScript checks if package.json has a specific script.
func (sc *SelfCorrector) hasNpmScript(root string, scriptName string) bool {
	packageJSON := filepath.Join(root, "package.json")
	data, err := os.ReadFile(packageJSON)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), fmt.Sprintf(`"%s"`, scriptName))
}

// runCheck executes a verification command and returns its output.
func (sc *SelfCorrector) runCheck(ctx context.Context, check checkCommand) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, selfCorrectTimeout)
	defer cancel()

	name, args := shellCommand(check.Command)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = sc.WorkspaceRoot

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// shellCommand determines how to run a shell command based on OS.
func shellCommand(command string) (string, []string) {
	if strings.Contains(os.Getenv("OS"), "Windows") || filepath.VolumeName(os.Getenv("SystemDrive")) != "" {
		return "powershell", []string{"-NoProfile", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func gofmtSelfCorrectCommand() string {
	if isWindowsShell() {
		return "$files = git ls-files '*.go'; if ($files) { gofmt -w $files }"
	}
	return "gofmt -w $(git ls-files '*.go')"
}

func isWindowsShell() bool {
	return strings.Contains(os.Getenv("OS"), "Windows") || filepath.VolumeName(os.Getenv("SystemDrive")) != ""
}
