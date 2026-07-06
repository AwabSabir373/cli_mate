package ui

import (
	"fmt"
	"strings"
)

func (a *App) planText() string {
	if a.planPanel == nil || len(a.planPanel.steps) == 0 {
		return "No plan is active."
	}
	items := a.planPanel.steps
	if len(items) == 0 {
		return "No plan is active."
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "Current Plan")
	for i, item := range items {
		status := item.Status
		if status == "" {
			status = "pending"
		}
		line := fmt.Sprintf("%d. [%s] %s", i+1, status, item.Title)
		if item.Description != "" {
			line += "\n   " + item.Description
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
