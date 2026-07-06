package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (a *App) handleInitCommand() (tea.Model, tea.Cmd) {
	if a.exiting {
		return a, nil
	}
	a.appendLog("system", "Starting project initialization...")
	a.appendLog("system", fmt.Sprintf("Working directory: %s", a.workspaceRoot))
	a.appendLog("system", "Use /help to see available commands. Press / to open the command menu.")
	_ = time.Now
	return a, nil
}
