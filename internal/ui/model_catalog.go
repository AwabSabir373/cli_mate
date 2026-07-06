package ui

import (
	"fmt"
	"sort"
	"strings"
)

func (a *App) modelListText() string {
	if a.cfg == nil || len(a.cfg.Profiles) == 0 {
		return "Models\nNo models configured.\nUse /provider to set up a provider."
	}

	activeProvider := ""
	activeModel := ""
	if p, err := a.cfg.Active(); err == nil {
		activeProvider = p.Provider
		activeModel = p.Model
	}

	var lines []string
	lines = append(lines, "Active model: "+displayValue(activeModel, "none"))
	lines = append(lines, "Provider: "+displayValue(activeProvider, "none"))
	lines = append(lines, "")

	var modelLines []string
	for _, profile := range a.cfg.Profiles {
		if profile.Model != "" {
			marker := " "
			if profile.Provider == activeProvider && profile.Model == activeModel {
				marker = "*"
			}
			modelLines = append(modelLines, fmt.Sprintf("%s %s/%s", marker, profile.Provider, profile.Model))
		}
	}
	sort.Strings(modelLines)
	lines = append(lines, modelLines...)

	return strings.Join(lines, "\n")
}

func displayValue(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
