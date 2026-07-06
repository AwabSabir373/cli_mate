package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

type clipboardReadMsg struct {
	content string
	err     error
}

func pasteFromClipboardCmd() tea.Cmd {
	return func() tea.Msg {
		content, err := clipboard.ReadAll()
		return clipboardReadMsg{content: content, err: err}
	}
}

func (a *App) routePaste(content string) (tea.Model, tea.Cmd) {
	if content == "" {
		return a, nil
	}
	a.insertText(content)
	a.selected = 0
	return a, nil
}
