package ui

import (
	tea "charm.land/bubbletea/v2"
)

// MouseZone identifies a clickable region in the UI.
type MouseZone string

const (
	MZoneNone        MouseZone = ""
	MZoneInput       MouseZone = "input"
	MZoneSuggestions MouseZone = "suggestions"
	MZoneLog         MouseZone = "log"
)

// MouseEvent represents a mouse interaction.
type MouseEvent struct {
	Zone   MouseZone
	Action string // "click", "scroll_up", "scroll_down"
	X      int
	Y      int
}

// HandleMouse processes a mouse message and performs the corresponding action.
func (a *App) HandleMouse(msg tea.MouseMsg) {
	switch m := msg.(type) {
	case tea.MouseWheelMsg:
		if a.viewport == nil {
			a.viewport = newViewport()
			a.viewport.setTotalLines(len(a.log))
		}
		mouse := m.Mouse()
		switch mouse.Button {
		case tea.MouseWheelUp:
			a.viewport.scrollBy(mouseWheelScrollStep)
		case tea.MouseWheelDown:
			a.viewport.scrollBy(-mouseWheelScrollStep)
		}
	case tea.MouseClickMsg:
		mouse := m.Mouse()
		_ = a.mouseHitTest(mouse.X, mouse.Y)
	}
}

func (a *App) mouseHitTest(_ int, y int) MouseZone {
	// Input area at bottom (prompt panel)
	promptLines := a.promptChromeLines()
	if promptLines < 2 {
		promptLines = 2
	}
	if y >= a.height-promptLines {
		return MZoneInput
	}

	// Suggestions / activity just above input
	if y >= a.height-promptLines-maxActivityStripLines && y < a.height-promptLines {
		return MZoneSuggestions
	}

	// Default: log / transcript area
	return MZoneLog
}
