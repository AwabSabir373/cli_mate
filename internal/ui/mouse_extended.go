package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// dragState tracks mouse drag operations.
type dragState struct {
	active      bool
	startX      int
	startY      int
	currentX    int
	currentY    int
	scrollAccum int // accumulated scroll delta
}

// mouseHandler provides enhanced mouse interaction handling.
type mouseHandler struct {
	drag        dragState
	clickBuffer []tea.MouseMsg // buffer for click-and-drag detection
	lastClick   tea.MouseMsg
}

// newMouseHandler creates a new enhanced mouse handler.
func newMouseHandler() *mouseHandler {
	return &mouseHandler{}
}

// handleMouse processes a mouse message and returns an action string.
// Actions: "scroll_up", "scroll_down", "click", "drag_start", "drag_end", "drag_select"
func (mh *mouseHandler) handleMouse(msg tea.MouseMsg) string {
	switch msg.Type {
	case tea.MouseWheelUp:
		return "scroll_up"

	case tea.MouseWheelDown:
		return "scroll_down"

	case tea.MouseLeft:
		// Left button down - start potential drag
		if !mh.drag.active {
			mh.drag.active = true
			mh.drag.startX = msg.X
			mh.drag.startY = msg.Y
			mh.drag.currentX = msg.X
			mh.drag.currentY = msg.Y
			return "click"
		}

	case tea.MouseMotion:
		// Mouse movement while button is pressed = drag
		if mh.drag.active {
			dx := msg.X - mh.drag.currentX
			dy := msg.Y - mh.drag.currentY
			mh.drag.currentX = msg.X
			mh.drag.currentY = msg.Y

			// Only trigger drag if we've moved enough
			if abs(dx) > 3 || abs(dy) > 3 {
				if abs(dy) > abs(dx) {
					// Vertical drag = scroll
					mh.drag.scrollAccum += dy
					if abs(mh.drag.scrollAccum) >= 2 {
						dir := "scroll_down"
						if mh.drag.scrollAccum < 0 {
							dir = "scroll_up"
						}
						mh.drag.scrollAccum = 0
						return dir
					}
				}
				return "drag"
			}
			return "drag_start"
		}

	case tea.MouseRelease:
		// Button release
		if mh.drag.active {
			mh.drag.active = false
			mh.drag.scrollAccum = 0
			return "drag_end"
		}
	}

	return ""
}

// isDragActive returns true if a drag operation is in progress.
func (mh *mouseHandler) isDragActive() bool {
	return mh.drag.active
}

// dragDelta returns the horizontal and vertical distance dragged.
func (mh *mouseHandler) dragDelta() (int, int) {
	return mh.drag.currentX - mh.drag.startX,
		mh.drag.currentY - mh.drag.startY
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
