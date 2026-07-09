package ui

import (
	tea "charm.land/bubbletea/v2"
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
	clickBuffer []tea.MouseClickMsg // buffer for click-and-drag detection
	lastClick   tea.MouseClickMsg
}

// newMouseHandler creates a new enhanced mouse handler.
func newMouseHandler() *mouseHandler {
	return &mouseHandler{}
}

// handleMouse processes a mouse message and returns an action string.
// Actions: "scroll_up", "scroll_down", "click", "drag_start", "drag_end", "drag_select"
func (mh *mouseHandler) handleMouse(msg tea.MouseMsg) string {
	switch m := msg.(type) {
	case tea.MouseWheelMsg:
		mouse := m.Mouse()
		if mouse.Button == tea.MouseWheelUp {
			return "scroll_up"
		}
		return "scroll_down"

	case tea.MouseClickMsg:
		mouse := m.Mouse()
		if !mh.drag.active {
			mh.drag.active = true
			mh.drag.startX = mouse.X
			mh.drag.startY = mouse.Y
			mh.drag.currentX = mouse.X
			mh.drag.currentY = mouse.Y
			return "click"
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
