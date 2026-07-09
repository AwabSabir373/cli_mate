package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// safeCommands is the whitelist of approved safe commands.
// Any command not matching this whitelist is rejected immediately.
//
// SECURITY NOTE: `make` is included because it is a common development tool,
// but it can execute arbitrary shell commands defined in Makefile rules.
// Only add `make` if you trust the workspace's Makefile contents.
var safeCommands = []string{
	"git",
	"go",
	"gofmt",
	"gopls",
	"make",
	"findstr",
	"dir",
	"echo",
	"cat",
	"head",
	"tail",
	"wc",
	"sort",
	"uniq",
	"which",
	"pwd",
	"ls",
	"ps",
	"date",
	"env",
}

// SandboxProvider is the interface for sandboxed command execution.
// When CLI_MATE_ENV=production is set, commands are routed through this.
type SandboxProvider interface {
	// Execute runs a command in a sandboxed environment and returns the output.
	Execute(ctx context.Context, name string, args []string, workdir string) ([]byte, error)
}

// defaultSandbox is a no-op sandbox that executes commands directly.
// It is used when no sandbox provider is configured.
type defaultSandbox struct{}

func (d *defaultSandbox) Execute(ctx context.Context, name string, args []string, workdir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workdir
	return cmd.CombinedOutput()
}

var globalSandbox SandboxProvider = &defaultSandbox{}

// SetSandboxProvider sets the global sandbox provider for command execution.
// When set, all shell tool commands are routed through this provider.
func SetSandboxProvider(s SandboxProvider) {
	globalSandbox = s
}

// GetSandboxProvider returns the current sandbox provider.
func GetSandboxProvider() SandboxProvider {
	return globalSandbox
}

// isProductionEnv returns true when CLI_MATE_ENV=production is set.
func isProductionEnv() bool {
	return os.Getenv("CLI_MATE_ENV") == "production"
}

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
					"description": "Shell command to run. Must be a safe, whitelisted command (e.g., go test, go build, gofmt, git status). Destructive and non-whitelisted commands are blocked.",
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory relative to workspace root (default: workspace root)",
				},
			},
			"description": "Run a safe, non-destructive command. Only whitelisted commands are allowed: go, git, gofmt, make, and basic inspection utilities. On Windows uses direct execution, on Unix uses direct execution (no shell interpreter).",
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

	// Parse the command into structured tokens
	name, args, err := parseCommand(command)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	// Validate against the whitelist
	if err := validateCommand(name); err != nil {
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

	// In production mode, route all execution through the sandbox provider.
	// Otherwise, execute directly with output capped by limitedBuffer.
	if isProductionEnv() {
		sandbox := GetSandboxProvider()
		sandboxOutput, sandboxErr := sandbox.Execute(ctx, name, args, resolvedWorkdir)
		if sandboxErr != nil {
			if ctx.Err() != nil {
				return Result{Content: string(sandboxOutput), Error: fmt.Sprintf("command timed out after %s", timeout)}, fmt.Errorf("command timed out after %s", timeout)
			}
			return Result{Content: string(sandboxOutput), Error: sandboxErr.Error()}, sandboxErr
		}
		return Result{Content: string(sandboxOutput)}, nil
	}

	output := newLimitedBuffer(maxToolOutputBytes)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = resolvedWorkdir
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

// parseCommand splits a command string into the executable name and its
// arguments, respecting basic shell quoting. This avoids passing shell
// interpreter metacharacters like pipes, redirects, and variable expansions.
func parseCommand(command string) (string, []string, error) {
	tokens, err := tokenize(command)
	if err != nil {
		return "", nil, fmt.Errorf("parse command: %w", err)
	}
	if len(tokens) == 0 {
		return "", nil, fmt.Errorf("empty command")
	}

	name := tokens[0]
	args := tokens[1:]

	// Block dangerous shell metacharacters — pipes, redirects, subshells.
	// Glob/brace characters (* ? [ ] { }) are allowed because they have no
	// special meaning when passed as literal arguments to exec.CommandContext.
	for i, token := range tokens {
		if containsShellMeta(token) {
			return "", nil, fmt.Errorf("command %q contains shell metacharacters at position %d: %q — use structured arguments instead of shell operators", name, i, token)
		}
	}

	return name, args, nil
}

// tokenize splits a command string into tokens, respecting single and double quotes.
func tokenize(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inDouble {
			// In double quotes, only certain escapes are meaningful.
			// We'll pass through for now and strip later.
			next := i + 1
			if next < len(s) && (s[next] == '"' || s[next] == '\\' || s[next] == '$' || s[next] == '`') {
				escaped = true
				continue
			}
			current.WriteByte(ch)
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if (ch == ' ' || ch == '\t') && !inSingle && !inDouble {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote in command")
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

// blockedShellMetas are characters that have dangerous shell meaning and are
// always rejected. Under exec.CommandContext these would just be literal
// characters, but blocking them provides defense-in-depth against accidental
// misuse or future execution path changes.
var blockedShellMetas = []string{
	"|",   // pipe
	";",   // command separator
	"&",   // background / command separator
	"`",   // command substitution (backtick)
	"$(",  // command substitution
	">",   // redirect output
	"<",   // redirect input
	"~",   // tilde expansion
}

// containsShellMeta checks if a token contains shell metacharacters that could
// be exploited if passed through a shell interpreter. Since we now use
// exec.CommandContext directly (no shell), these would normally be just
// regular arguments. However, blocking them provides defense-in-depth.
//
// Glob/brace characters (* ? [ ] { }) are intentionally NOT blocked here
// because they have no special meaning when passed as literal arguments
// to exec.CommandContext — they just become literal characters in argv.
func containsShellMeta(token string) bool {
	for _, meta := range blockedShellMetas {
		if strings.Contains(token, meta) {
			return true
		}
	}
	return false
}

// validateCommand checks that the command name is in the safe whitelist.
func validateCommand(name string) error {
	base := strings.TrimSuffix(name, ".exe")
	lowered := strings.ToLower(base)

	for _, safe := range safeCommands {
		if lowered == safe {
			return nil
		}
	}

	return fmt.Errorf("command %q is not in the safe command whitelist. Allowed commands: %s. Use only these simple commands with structured arguments (no pipes, redirects, or shell operators).", name, strings.Join(safeCommands, ", "))
}


