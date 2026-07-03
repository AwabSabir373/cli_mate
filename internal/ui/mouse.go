package ui

import (
	tea "github.com/charmbracelet/bubbletea"
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
	x, y := msg.X, msg.Y
	zone := a.mouseHitTest(x, y)

	switch msg.Type {
	case tea.MouseWheelUp:
		if zone == MZoneLog {
			a.scrollUp()
		}
	case tea.MouseWheelDown:
		if zone == MZoneLog {
			a.scrollDown()
		}
	}
}

func (a *App) mouseHitTest(x, y int) MouseZone {
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
