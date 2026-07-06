package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"cli_mate/internal/usercommands"
)

func (a *App) handleUserCommand(raw string) (tea.Model, tea.Cmd, bool) {
	name, args := splitUserCommand(raw)
	if name == "" {
		return a, nil, false
	}
	cmd, ok := a.lookupUserCommand(name)
	if !ok {
		return a, nil, false
	}
	prompt := usercommands.Expand(cmd.Template, args)
	if strings.TrimSpace(prompt) == "" {
		return a, nil, false
	}
	a.setInput(prompt)
	return a, a.submit(), true
}

func (a *App) lookupUserCommand(name string) (usercommands.Command, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, cmd := range a.userCommands {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return usercommands.Command{}, false
}

func splitUserCommand(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "/") {
		return "", ""
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return "", ""
	}
	name := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	args := strings.TrimSpace(raw[len(fields[0]):])
	return name, args
}
