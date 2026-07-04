package ui

import (
	"fmt"
	"strings"
	"time"
)

// transcriptSelection manages row selection, expansion, and interaction in the transcript.
type transcriptSelection struct {
	selectedRow     int      // currently selected row index, -1 if none
	expandedRows    map[int]bool // row indices that are expanded
	hoveredRow      int      // row under mouse cursor, -1 if none
	dragging        bool
	lastClickedTime time.Time
}

// newTranscriptSelection creates a new transcript selection manager.
func newTranscriptSelection() *transcriptSelection {
	return &transcriptSelection{
		selectedRow:  -1,
		expandedRows: make(map[int]bool),
		hoveredRow:   -1,
	}
}

// selectRow selects a row by index.
func (ts *transcriptSelection) selectRow(idx int, maxRows int) {
	if idx >= 0 && idx < maxRows {
		ts.selectedRow = idx
	}
}

// toggleExpand toggles the expanded state of a row.
func (ts *transcriptSelection) toggleExpand(idx int) {
	if ts.expandedRows[idx] {
		delete(ts.expandedRows, idx)
	} else {
		ts.expandedRows[idx] = true
	}
}

// isExpanded returns true if the row is expanded.
func (ts *transcriptSelection) isExpanded(idx int) bool {
	return ts.expandedRows[idx]
}

// isRowVisible checks if a row should be visible based on filtering/collapsing rules.
func (ts *transcriptSelection) isRowVisible(row transcriptRow, allRows []transcriptRow, idx int) bool {
	// Plumbing tools are always hidden
	switch row.tool {
	case "update_plan", "tool_search", "enter_plan_mode", "exit_plan_mode",
		"discover_skills", "verify_plan_execution":
		return false
	}

	// Collapse repeated status cards
	if idx > 0 && row.kind == rowToolCall && allRows[idx-1].kind == rowToolCall {
		if row.tool == allRows[idx-1].tool && row.text == allRows[idx-1].text {
			return false
		}
	}

	return true
}

// handleClick handles a mouse click on the transcript.
func (ts *transcriptSelection) handleClick(rowIdx int, _ appStyles) string {
	if rowIdx < 0 {
		return ""
	}

	ts.selectedRow = rowIdx
	now := time.Now()

	// Double-click detection
	if now.Sub(ts.lastClickedTime) < 500*time.Millisecond && ts.selectedRow == rowIdx {
		ts.toggleExpand(rowIdx)
		ts.lastClickedTime = time.Time{}
		return "double_click"
	}

	ts.lastClickedTime = now
	return "single_click"
}

// getSelectedText returns the text of the selected row.
func (ts *transcriptSelection) getSelectedText(rows []transcriptRow) string {
	if ts.selectedRow < 0 || ts.selectedRow >= len(rows) {
		return ""
	}
	return rows[ts.selectedRow].text
}

// renderTranscriptSelection visual indicators for selection state.
func renderTranscriptSelection(ts *transcriptSelection, rowIdx int, styles appStyles) string {
	if ts.selectedRow == rowIdx {
		return styles.selected.Render("▸")
	}
	return " "
}

// collapseRepeatedToolCalls removes consecutive identical tool call/result pairs.
func collapseRepeatedToolCalls(rows []transcriptRow) []transcriptRow {
	if len(rows) < 4 {
		return rows
	}

	var result []transcriptRow
	i := 0
	for i < len(rows) {
		// Check for pattern: tool call + tool result repeated
		if i+3 < len(rows) &&
			rows[i].kind == rowToolCall &&
			rows[i+1].kind == rowToolResult &&
			rows[i+2].kind == rowToolCall &&
			rows[i+3].kind == rowToolResult &&
			rows[i].tool == rows[i+2].tool &&
			rows[i].text == rows[i+2].text {

			// Skip the second pair
			result = append(result, rows[i], rows[i+1])
			i += 4

			// Add a collapse indicator
			result = append(result, transcriptRow{
				kind:      rowSystem,
				text:      fmt.Sprintf("  ... %s repeated ...", rows[i].tool),
				timestamp: time.Now(),
			})
			continue
		}
		result = append(result, rows[i])
		i++
	}
	return result
}

// renderTranscriptRowWithSelection renders a row with selection and expansion state.
func renderTranscriptRowWithSelection(row transcriptRow, ts *transcriptSelection, idx int, styles appStyles, width int, showDetail bool) string {
	var b strings.Builder

	// Selection indicator
	if ts.selectedRow == idx {
		b.WriteString(styles.selected.Render(" ▸ "))
	} else {
		b.WriteString("   ")
	}

	// Expansion toggle for tool calls
	if row.kind == rowToolCall || row.kind == rowToolResult {
		if ts.isExpanded(idx) {
			b.WriteString(styles.muted.Render("▼ "))
		} else {
			b.WriteString(styles.muted.Render("▶ "))
		}
	}

	// Render the row content
	switch row.kind {
	case rowUser:
		b.WriteString(styles.accent.Render("You: "))
		b.WriteString(truncateString(row.text, width-20))
	case rowAssistant:
		b.WriteString(styles.roleAssist.Render(row.text))
	case rowToolCall:
		if showDetail || ts.isExpanded(idx) {
			b.WriteString(styles.roleTool.Render(fmt.Sprintf("🔧 %s", row.tool)))
			if row.detail != "" && ts.isExpanded(idx) {
				b.WriteString("\n")
				b.WriteString(styles.softPanel.Width(width - 12).Render(row.detail))
			}
		} else {
			b.WriteString(styles.roleTool.Render(fmt.Sprintf("🔧 %s %s", row.tool, truncateString(row.text, width-25))))
		}
	case rowToolResult:
		if ts.isExpanded(idx) && row.detail != "" {
			b.WriteString(styles.softPanel.Width(width - 12).Render(row.detail))
		} else {
			b.WriteString(styles.muted.Render(truncateString(row.text, width-15)))
		}
	case rowSystem:
		b.WriteString(styles.roleSystem.Render(row.text))
	case rowError:
		b.WriteString(styles.error.Render(row.text))
	case rowReasoning:
		b.WriteString(styles.muted.Render(fmt.Sprintf("🤔 %s", row.text)))
	}

	b.WriteString("\n")
	return b.String()
}

// transcriptView handles viewport-aware rendering of the transcript.
type transcriptView struct {
	rows        []transcriptRow
	visibleRows int
	scrollPos   int
	pinBottom   bool
}

// newTranscriptView creates a new transcript view.
func newTranscriptView() *transcriptView {
	return &transcriptView{
		pinBottom: true,
	}
}

// append adds a row and auto-scrolls if pinned.
func (tv *transcriptView) append(row transcriptRow) {
	tv.rows = append(tv.rows, row)

	// Collapse repeated entries
	tv.rows = collapseRepeatedToolCalls(tv.rows)

	if tv.pinBottom {
		tv.scrollToBottom()
	}
}

// visibleRange returns the start and end indices of visible rows.
func (tv *transcriptView) visibleRange() (int, int) {
	if len(tv.rows) == 0 {
		return 0, 0
	}

	start := len(tv.rows) - tv.visibleRows - tv.scrollPos
	if start < 0 {
		start = 0
	}
	end := start + tv.visibleRows
	if end > len(tv.rows) {
		end = len(tv.rows)
	}
	return start, end
}

// scrollUp scrolls up by one row.
func (tv *transcriptView) scrollUp() {
	maxScroll := len(tv.rows) - tv.visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if tv.scrollPos < maxScroll {
		tv.scrollPos++
		tv.pinBottom = false
	}
}

// scrollDown scrolls down by one row.
func (tv *transcriptView) scrollDown() {
	if tv.scrollPos > 0 {
		tv.scrollPos--
	}
	if tv.scrollPos == 0 {
		tv.pinBottom = true
	}
}

// scrollToBottom scrolls to the bottom.
func (tv *transcriptView) scrollToBottom() {
	tv.scrollPos = 0
	tv.pinBottom = true
}

// isAtBottom returns true if viewing the latest content.
func (tv *transcriptView) isAtBottom() bool {
	return tv.scrollPos == 0
}

// setVisibleRows updates the visible row count.
func (tv *transcriptView) setVisibleRows(n int) {
	tv.visibleRows = n
}
