package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// HookEvent represents the lifecycle events where hooks can fire.
type HookEvent string

const (
	HookBeforeTool   HookEvent = "beforeTool"
	HookAfterTool    HookEvent = "afterTool"
	HookBeforePrompt HookEvent = "beforePrompt"
	HookAfterPrompt  HookEvent = "afterPrompt"
	HookOnError      HookEvent = "onError"
	HookOnSession    HookEvent = "onSessionStart"
)

// Hook defines a single lifecycle hook configuration.
type Hook struct {
	Event    HookEvent `json:"event"`
	Command  string    `json:"command"`
	ToolName string    `json:"tool_name,omitempty"` // optional: filter to specific tool
	Timeout  int       `json:"timeout,omitempty"`   // seconds, default 30
}

// HookResult is the outcome of running a hook.
type HookResult struct {
	Hook     Hook
	Output   string
	ExitCode int
	Error    error
	Blocked  bool // true if beforeTool hook returned non-zero (blocks tool call)
}

// HookDispatcher manages and executes lifecycle hooks.
type HookDispatcher struct {
	hooks      []Hook
	enabled    bool
	auditStore *AuditStore
}

// NewHookDispatcher creates a dispatcher from hook configurations.
func NewHookDispatcher(hooks []Hook) *HookDispatcher {
	return &HookDispatcher{
		hooks:   hooks,
		enabled: len(hooks) > 0,
	}
}

// SetAuditStore configures the audit log store for hook executions.
func (hd *HookDispatcher) SetAuditStore(store *AuditStore) {
	hd.auditStore = store
}

// SetEnabled toggles the hook system on or off.
func (hd *HookDispatcher) SetEnabled(enabled bool) {
	hd.enabled = enabled
}

// Dispatch fires all matching hooks for the given event. Returns the combined
// results. For beforeTool events, if any hook returns non-zero, Blocked is true.
func (hd *HookDispatcher) Dispatch(ctx context.Context, event HookEvent, toolName string, extra map[string]string) []HookResult {
	if !hd.enabled {
		return nil
	}

	var results []HookResult
	for _, hook := range hd.hooks {
		if hook.Event != event {
			continue
		}
		// If tool_name filter is set, only fire for matching tools
		if hook.ToolName != "" && hook.ToolName != toolName {
			continue
		}

		timeout := 30
		if hook.Timeout > 0 {
			timeout = hook.Timeout
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)

		command := interpolateCommand(hook.Command, toolName, extra)
		name, args := shellExecCommand(command)
		cmd := exec.CommandContext(ctx, name, args...)
		output, err := cmd.CombinedOutput()
		cancel()

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		result := HookResult{
			Hook:     hook,
			Output:   string(output),
			ExitCode: exitCode,
			Error:    err,
			Blocked:  event == HookBeforeTool && exitCode != 0,
		}
		results = append(results, result)
	}

	// Record audit entries
	if hd.auditStore != nil && len(results) > 0 {
		hd.auditStore.RecordResults(event, toolName, results)
	}

	return results
}

// ShouldBlock checks if any beforeTool hook blocked the tool call.
func ShouldBlock(results []HookResult) bool {
	for _, r := range results {
		if r.Blocked {
			return true
		}
	}
	return false
}

// GetFeedback returns combined non-empty outputs from hooks (for afterTool feedback to model).
func GetFeedback(results []HookResult) string {
	var parts []string
	for _, r := range results {
		output := strings.TrimSpace(r.Output)
		if output != "" {
			parts = append(parts, output)
		}
	}
	return strings.Join(parts, "\n\n")
}

// interpolateCommand replaces {{tool}} and {{key}} placeholders in the command string.
func interpolateCommand(command, toolName string, extra map[string]string) string {
	command = strings.ReplaceAll(command, "{{tool}}", toolName)
	for key, value := range extra {
		command = strings.ReplaceAll(command, "{{"+key+"}}", value)
	}
	return command
}

// shellExecCommand determines how to execute a shell command based on OS.
func shellExecCommand(command string) (string, []string) {
	// Detect Windows
	if isWindows() {
		return "powershell", []string{"-NoProfile", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

func isWindows() bool {
	return strings.Contains(
		strings.ToLower(fmt.Sprintf("%s%s%s",
			execPath(), os.Getenv("OS"), os.Getenv("GOOS"))),
		"windows",
	)
}

func execPath() string {
	p, _ := exec.LookPath("powershell")
	return p
}
