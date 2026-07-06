package ui

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
)

func (a *App) providerText() string {
	var lines []string
	profile, err := a.cfg.Active()
	if err != nil {
		lines = append(lines, "No active provider configured.")
		lines = append(lines, "Use /provider to set up a provider.")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, fmt.Sprintf("Provider: %s", profile.Provider))
	lines = append(lines, fmt.Sprintf("Model: %s", profile.Model))
	lines = append(lines, fmt.Sprintf("Base URL: %s", profile.BaseURL))
	if profile.APIKey != "" {
		lines = append(lines, "API Key: configured")
	} else {
		lines = append(lines, "API Key: not configured")
	}
	return strings.Join(lines, "\n")
}

func (a *App) modelText(args string) string {
	return a.modelListText()
}

func (a *App) configText() string {
	var lines []string
	lines = append(lines, "Runtime Configuration")
	lines = append(lines, "")
	if a.cfg == nil {
		lines = append(lines, "No configuration loaded.")
		return strings.Join(lines, "\n")
	}
	profile, err := a.cfg.Active()
	if err == nil {
		lines = append(lines, fmt.Sprintf("Provider: %s", profile.Provider))
		lines = append(lines, fmt.Sprintf("Model: %s", profile.Model))
	}
	lines = append(lines, fmt.Sprintf("Response Style: %s", a.responseStyle))
	if a.loading {
		lines = append(lines, "Status: processing")
	} else {
		lines = append(lines, "Status: idle")
	}
	return strings.Join(lines, "\n")
}

func (a *App) contextText() string {
	var lines []string
	lines = append(lines, "Context")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Working directory: %s", a.workspaceRoot))
	lines = append(lines, fmt.Sprintf("Files indexed: %d", len(a.files)))
	lines = append(lines, fmt.Sprintf("Messages: %d", len(a.messages)))
	return strings.Join(lines, "\n")
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func apiKeyState(configured bool) string {
	if configured {
		return "configured"
	}
	return "not configured"
}

func providerProfileHasCredential(profile config.Profile) bool {
	return profile.APIKey != ""
}
