package ui

import (
	"fmt"
	"strings"
)

// planStepWork represents work attributed to a plan step.
type planStepWork struct {
	tool    string // "write_file", "edit_file", "bash", "apply_patch", etc.
	summary string // One-line summary
	detail  string // Full output (diff, stdout, etc.)
	path    string // File path if applicable
}

// planStepDetail manages the detail view for a selected plan step.
type planStepDetail struct {
	stepIndex int
	stepTitle string
	works     []planStepWork
	visible   bool
	scrollPos int
}

// newPlanStepDetail creates a new plan step detail viewer.
func newPlanStepDetail() *planStepDetail {
	return &planStepDetail{}
}

// show displays the detail view for the given step and its work.
func (psd *planStepDetail) show(stepIndex int, title string, works []planStepWork) {
	psd.stepIndex = stepIndex
	psd.stepTitle = title
	psd.works = works
	psd.visible = true
	psd.scrollPos = 0
}

// hide hides the detail view.
func (psd *planStepDetail) hide() {
	psd.visible = false
	psd.works = nil
}

// isVisible returns true if the detail view is active.
func (psd *planStepDetail) isVisible() bool {
	return psd.visible
}

// scrollUp scrolls up in the detail view.
func (psd *planStepDetail) scrollUp() {
	if psd.scrollPos < len(psd.works)-1 {
		psd.scrollPos++
	}
}

// scrollDown scrolls down in the detail view.
func (psd *planStepDetail) scrollDown() {
	if psd.scrollPos > 0 {
		psd.scrollPos--
	}
}

// isWorkTool returns true if the tool represents implementation work worth showing.
func isWorkTool(toolName string) bool {
	switch toolName {
	case "write_file", "edit_file", "file_write", "file_edit",
		"apply_patch", "str_replace",
		"bash", "shell", "run_terminal_command",
		"exec_command":
		return true
	}
	return false
}

// captureWorkFromLog extracts plan step work from tool log entries.
func captureWorkFromLog(log []logEntry) []planStepWork {
	var works []planStepWork

	for _, entry := range log {
		if entry.Kind != "tool" {
			continue
		}

		toolName := parseToolName(entry.Text)
		if !isWorkTool(toolName) {
			continue
		}

		path := parseToolPath(entry.Text)
		summary := entry.Text
		if len(summary) > 80 {
			summary = summary[:80] + "..."
		}

		works = append(works, planStepWork{
			tool:    toolName,
			summary: summary,
			detail:  entry.Text,
			path:    path,
		})
	}

	return works
}

// renderPlanStepDetail renders the plan step detail view.
func renderPlanStepDetail(psd *planStepDetail, _ int, styles appStyles) string {
	if !psd.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(fmt.Sprintf("Step %d: %s", psd.stepIndex+1, truncateString(psd.stepTitle, 40))))
	b.WriteString("\n\n")

	if len(psd.works) == 0 {
		b.WriteString(styles.muted.Render("  No work recorded for this step."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Esc to close"))
		return b.String()
	}

	for i, work := range psd.works {
		icon := "  "
		switch work.tool {
		case "write_file", "edit_file", "file_write", "file_edit", "apply_patch":
			icon = "✎ "
		case "bash", "shell", "run_terminal_command":
			icon = "$ "
		default:
			icon = "▸ "
		}

		b.WriteString(styles.roleTool.Render(fmt.Sprintf("%s%s", icon, work.tool)))
		if work.path != "" {
			b.WriteString(" ")
			b.WriteString(styles.muted.Render(work.path))
		}
		b.WriteString("\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %s", work.summary)))
		b.WriteString("\n")

		if i < len(psd.works)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Esc to close"))

	return b.String()
}
