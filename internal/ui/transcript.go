package ui

import (
	"fmt"
	"strings"
	"time"
)

// rowKind categorizes transcript entries.
type rowKind string

const (
	rowUser       rowKind = "user"
	rowAssistant  rowKind = "assistant"
	rowToolCall   rowKind = "tool"
	rowToolResult rowKind = "tool_result"
	rowSystem     rowKind = "system"
	rowError      rowKind = "error"
	rowReasoning  rowKind = "reasoning"
	rowWelcome    rowKind = "welcome"
	rowRecap      rowKind = "recap"
)

// transcriptRow represents a single row in the conversation transcript.
type status string

const (
	statusOK    status = "ok"
	statusError status = "error"
)

type transcriptRow struct {
	kind         rowKind
	id           string
	text         string
	detail       string
	tool         string
	arg          string
	status       status
	failed       bool
	runID        int
	final        bool
	turnTools    int
	turnElapsed  time.Duration
	changedFiles []string
	timestamp    time.Time
	expanded     bool
}

// startsTurn reports whether a row kind begins a new user or assistant turn.
func startsTurn(k rowKind) bool {
	return k == rowUser || k == rowAssistant
}

// transcriptAction is a sum type describing a mutation on a transcript slice.
// It is processed by reduceTranscript which appends, replaces, or inserts the
// action's row. The Redux-style reducer keeps the transcript update logic in
// one place – no scattered append/edit/replace calls across handlers.
type transcriptAction struct {
	kind transcriptActionKind
	row  transcriptRow
	text string
}

type transcriptActionKind int

const (
	actionAppendUser transcriptActionKind = iota
	actionAppendAssistant
	actionAppendToolCall
	actionAppendToolResult
	actionAppendSystem
	actionAppendError
	actionAppendReasoning
	actionReplaceLast
	actionInsertAfter
)

// reduceTranscript applies an action to a transcript slice and returns the
// updated slice.
func reduceTranscript(rows []transcriptRow, action transcriptAction) []transcriptRow {
	switch action.kind {
	case actionAppendUser, actionAppendAssistant, actionAppendToolCall, actionAppendToolResult, actionAppendSystem, actionAppendError, actionAppendReasoning:
		return append(rows, action.row)
	case actionReplaceLast:
		if len(rows) > 0 {
			rows[len(rows)-1] = action.row
		}
		return rows
	case actionInsertAfter:
		for i, r := range rows {
			if r.kind == action.row.kind && r.id != "" && r.id == action.row.id {
				rows = append(rows[:i+1], append([]transcriptRow{action.row}, rows[i+1:]...)...)
				return rows
			}
		}
		return append(rows, action.row)
	}
	return rows
}

// appendTranscriptRow is a convenience wrapper that creates an action and
// reduces it in one call.
func appendTranscriptRow(rows []transcriptRow, row transcriptRow) []transcriptRow {
	return reduceTranscript(rows, transcriptAction{kind: actionForKind(row.kind), row: row})
}

func actionForKind(k rowKind) transcriptActionKind {
	switch k {
	case rowUser:
		return actionAppendUser
	case rowAssistant:
		return actionAppendAssistant
	case rowToolCall:
		return actionAppendToolCall
	case rowToolResult:
		return actionAppendToolResult
	case rowSystem:
		return actionAppendSystem
	case rowError:
		return actionAppendError
	case rowReasoning:
		return actionAppendReasoning
	default:
		return actionAppendSystem
	}
}

// Agent message types for async communication between agent goroutine and UI.
type agentTextMsg struct {
	runID int
	delta string
}

type agentReasoningMsg struct {
	runID int
	delta string
}

type agentRowMsg struct {
	runID int
	row   transcriptRow
}

type agentResponseMsg struct {
	runID       int
	rows        []transcriptRow
	err         error
	turnTools   int
	turnElapsed time.Duration
}

type toolCallStreamStartMsg struct {
	runID int
	id    string
	name  string
}

type toolCallStreamDeltaMsg struct {
	runID    int
	id       string
	fragment string
}

type planUpdateMsg struct {
	runID int
	items []planItem
}

type planItem struct {
	Status  string
	Content string
	Notes   string
}

// transcript manages the structured conversation history.
type transcript struct {
	rows       []transcriptRow
	scrollPos  int
	showDetail bool // Toggle between compact/detailed views
}

// newTranscript creates a new empty transcript.
func newTranscript() *transcript {
	return &transcript{
		showDetail: false,
	}
}

// append adds a new row to the transcript.
func (t *transcript) append(kind rowKind, text string) {
	t.rows = append(t.rows, transcriptRow{
		kind:      kind,
		text:      text,
		timestamp: time.Now(),
		final:     true,
	})
}

// appendTool adds a tool call row to the transcript.
func (t *transcript) appendTool(name string, args string, result string) {
	t.rows = append(t.rows, transcriptRow{
		kind:      rowToolCall,
		tool:      name,
		detail:    result,
		text:      fmt.Sprintf("%s %s", name, args),
		timestamp: time.Now(),
		final:     true,
	})
}

// appendStreaming adds a streaming assistant row.
func (t *transcript) appendStreaming(text string, final bool) {
	if len(t.rows) > 0 && t.rows[len(t.rows)-1].kind == rowAssistant {
		// Append to the last assistant row
		last := &t.rows[len(t.rows)-1]
		last.text += text
		last.final = final
		last.timestamp = time.Now()
		return
	}
	t.rows = append(t.rows, transcriptRow{
		kind:      rowAssistant,
		text:      text,
		timestamp: time.Now(),
		final:     final,
	})
}

// clear removes all rows.
func (t *transcript) clear() {
	t.rows = nil
	t.scrollPos = 0
}

// visibleRows returns the slice of visible rows based on scroll position.
func (t *transcript) visibleRows(maxLines int) []transcriptRow {
	if len(t.rows) == 0 {
		return nil
	}

	start := len(t.rows) - maxLines - t.scrollPos
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(t.rows) {
		end = len(t.rows)
	}

	return t.rows[start:end]
}

// hasOlderEntries returns true if there are older entries before the visible window.
func (t *transcript) hasOlderEntries(maxLines int) (int, bool) {
	start := len(t.rows) - maxLines - t.scrollPos
	if start <= 0 {
		return 0, false
	}
	return start, true
}

// hasNewerEntries returns true if there are newer entries after the visible window.
func (t *transcript) hasNewerEntries(_ int) (int, bool) {
	end := len(t.rows) - t.scrollPos
	if end >= len(t.rows) {
		return 0, false
	}
	return len(t.rows) - end, true
}

// scrollUp scrolls the transcript up by one line.
func (t *transcript) scrollUp() {
	maxScroll := len(t.rows) - 1
	if t.scrollPos < maxScroll {
		t.scrollPos++
	}
}

// scrollDown scrolls the transcript down by one line.
func (t *transcript) scrollDown() {
	if t.scrollPos > 0 {
		t.scrollPos--
	}
}

// toggleDetail toggles between compact and detailed transcript views.
func (t *transcript) toggleDetail() {
	t.showDetail = !t.showDetail
}

// renderRow renders a single transcript row with appropriate styling.
func renderTranscriptRow(row transcriptRow, styles appStyles, width int, showDetail bool) string {
	var b strings.Builder

	switch row.kind {
	case rowUser:
		b.WriteString(styles.roleAssist.Render(fmt.Sprintf("You: %s", truncateString(row.text, width-20))))
	case rowAssistant:
		b.WriteString(styles.roleAssist.Render(row.text))
	case rowToolCall:
		// Show tool name only in compact mode
		if showDetail {
			b.WriteString(styles.roleTool.Render(fmt.Sprintf("🔧 %s", row.tool)))
			if row.detail != "" {
				b.WriteString("\n")
				b.WriteString(styles.muted.Render(truncateString(row.detail, width-10)))
			}
		} else {
			b.WriteString(styles.roleTool.Render(fmt.Sprintf("🔧 %s %s", row.tool, truncateString(row.text, width-20))))
		}
	case rowToolResult:
		b.WriteString(styles.muted.Render(truncateString(row.text, width-10)))
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
