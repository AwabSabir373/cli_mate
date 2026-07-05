package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// hoverKind identifies the type of element under the mouse cursor.
type hoverKind int

const (
	hoverNone hoverKind = iota
	hoverTranscript
	hoverSidebarAgent
	hoverPlanStep
	hoverFileRow
)

// hoverTarget stores metadata about the element currently under the mouse.
type hoverTarget struct {
	kind      hoverKind
	stepIndex int // For plan steps
	filePath  string
	label     string
}

// hoverManager manages mouse hover state and click interactions.
type hoverManager struct {
	target    hoverTarget
	prevClick hoverTarget // Track last click for double-click detection
	enabled   bool
}

// newHoverManager creates a new hover manager.
func newHoverManager() *hoverManager {
	return &hoverManager{
		enabled: true,
	}
}

// isMouseHover returns true if the message is a mouse motion (not a click).
func isMouseHover(msg tea.MouseMsg) bool {
	return msg.Button == tea.MouseButtonNone
}

// isMouseClick returns true if the message is a mouse click.
func isMouseClick(msg tea.MouseMsg) bool {
	return msg.Button != tea.MouseButtonNone &&
		msg.Button != tea.MouseButtonWheelUp &&
		msg.Button != tea.MouseButtonWheelDown
}

// updateHover updates the hover target based on current mouse position.
// hit tests against transcript rows, plan steps, and sidebar files.
func (hm *hoverManager) updateHover(
	y int,
	transcriptRows int,
	planSteps int,
	touchedFiles int,
	sidebarVisible bool,
	planVisible bool,
) {
	hm.target = hoverTarget{kind: hoverNone}

	currentY := y

	// Hit test plan steps (shown in sidebar)
	if planVisible && currentY >= 2 && currentY < 2+planSteps {
		stepIdx := currentY - 2
		if stepIdx >= 0 && stepIdx < planSteps {
			hm.target = hoverTarget{
				kind:      hoverPlanStep,
				stepIndex: stepIdx,
				label:     fmt.Sprintf("Step %d", stepIdx+1),
			}
			return
		}
	}

	// Hit test touched files (shown in sidebar)
	if sidebarVisible && currentY >= 4+planSteps && currentY < 4+planSteps+touchedFiles {
		fileIdx := currentY - 4 - planSteps
		if fileIdx >= 0 && fileIdx < touchedFiles {
			hm.target = hoverTarget{
				kind:  hoverFileRow,
				label: fmt.Sprintf("File %d", fileIdx+1),
			}
			return
		}
	}

	// Hit test transcript rows
	if currentY >= 0 && currentY < transcriptRows {
		hm.target = hoverTarget{
			kind:  hoverTranscript,
			label: fmt.Sprintf("Row %d", currentY),
		}
	}
}

// handleClick processes a mouse click and returns an action string.
func (hm *hoverManager) handleClick() string {
	if hm.target.kind == hoverNone {
		return ""
	}

	switch hm.target.kind {
	case hoverPlanStep:
		return fmt.Sprintf("plan_step_click:%d", hm.target.stepIndex)
	case hoverFileRow:
		return fmt.Sprintf("file_click:%s", hm.target.label)
	case hoverTranscript:
		return "transcript_click"
	}

	return ""
}

// renderHoverIndicator renders a visual indicator for the hovered element.
func renderHoverIndicator(target hoverTarget, styles appStyles) string {
	if target.kind == hoverNone {
		return ""
	}

	switch target.kind {
	case hoverPlanStep:
		return styles.hover.Render(fmt.Sprintf("  Click to view details for %s", target.label))
	case hoverFileRow:
		return styles.hover.Render("  Click to open file")
	case hoverTranscript:
		return styles.hover.Render("  Click to select")
	}

	return ""
}

// renderHoverStyle applies hover styling to text if the element is hovered.
func (hm *hoverManager) renderHoverStyle(text string, kind hoverKind, index int, styles appStyles) string {
	if hm.target.kind != kind {
		return text
	}
	switch kind {
	case hoverPlanStep:
		if hm.target.stepIndex == index {
			return styles.hover.Render(text)
		}
	case hoverFileRow:
		if hm.target.label == text {
			return styles.hover.Render(text)
		}
	}
	return text
}

// formatHoverHint returns a formatted hint string for the hovered element.
func formatHoverHint(target hoverTarget, styles appStyles) string {
	if target.kind == hoverNone {
		return ""
	}

	switch target.kind {
	case hoverPlanStep:
		return strings.TrimSpace(fmt.Sprintf("%s %s",
			styles.muted.Render("Click to view step details"),
			styles.scrollHint.Render("(click)"),
		))
	case hoverFileRow:
		return strings.TrimSpace(fmt.Sprintf("%s %s",
			styles.muted.Render("Click to open file"),
			styles.scrollHint.Render("(click)"),
		))
	}
	return ""
}
