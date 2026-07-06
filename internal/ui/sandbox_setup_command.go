package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (a *App) startSandboxSetupCommand(args string) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(args) != "" {
		a.appendLog("system", "Usage: /sandbox-setup")
		return a, nil
	}
	if a.pending {
		a.appendLog("system", "Cannot run sandbox setup while a run is active.")
		return a, nil
	}
	a.appendLog("system", "Sandbox setup is not available in this environment.")
	return a, nil
}
