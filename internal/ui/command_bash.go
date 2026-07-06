package ui

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const bashEscapeTimeout = 30 * time.Second

type bashResultMsg struct {
	command string
	output  string
}

func runBashEscape(cwd, command string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), bashEscapeTimeout)
		defer cancel()

		name, shellArgs := escapeShell(command)
		cmd := exec.CommandContext(ctx, name, shellArgs...)
		if strings.TrimSpace(cwd) != "" {
			cmd.Dir = cwd
		}
		out, err := cmd.CombinedOutput()
		text := strings.TrimRight(string(out), "\n")

		switch {
		case ctx.Err() == context.DeadlineExceeded:
			text = appendNote(text, "[timed out after "+bashEscapeTimeout.String()+"]")
		case err != nil:
			text = appendNote(text, "[exit error: "+err.Error()+"]")
		}
		if strings.TrimSpace(text) == "" {
			text = "(no output)"
		}
		return bashResultMsg{command: command, output: text}
	}
}

func escapeShell(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", []string{"/d", "/s", "/c", command}
	}
	return "/bin/sh", []string{"-c", command}
}

func appendNote(text, note string) string {
	if text == "" {
		return note
	}
	return text + "\n" + note
}
