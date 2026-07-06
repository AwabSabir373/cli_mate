package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

type providerModelsDiscoveredMsg struct {
	providerID string
	token      int
	err        error
}

func (a *App) providerModelDiscoveryCmd() tea.Cmd {
	if a.provider == nil {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_ = ctx
		return providerModelsDiscoveredMsg{
			providerID: "current",
			token:      0,
			err:        nil,
		}
	}
}

func providerWizardUsesTypedModel(provider interface{}) bool {
	return false
}

func firstProviderDisplayValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
