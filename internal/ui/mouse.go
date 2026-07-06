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
		mouse := m.Mouse()
		zone := a.mouseHitTest(mouse.X, mouse.Y)
		if zone == MZoneLog {
			a.scrollUp()
		}
	case tea.MouseClickMsg:
		mouse := m.Mouse()
		zone := a.mouseHitTest(mouse.X, mouse.Y)
		_ = zone
	}
}

func (a *App) mouseHitTest(_ int, y int) MouseZone {
	// Input area at bottom (last 2 lines)
	if y >= a.height-2 {
		return MZoneInput
	}

	// Suggestions area (above input)
	if y >= a.height-10 && y < a.height-2 {
		return MZoneSuggestions
	}

	// Default: log area
	return MZoneLog
}
