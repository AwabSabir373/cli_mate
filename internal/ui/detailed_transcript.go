package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// toggleDetailedTranscript toggles the detailed transcript overlay. When
// entering detailed mode, suggestions are cleared and pickers are dismissed.
func (a *App) toggleDetailedTranscript() {
	a.transcriptDetailed = !a.transcriptDetailed
	a.clearSuggestions()
	a.picker = nil
}

// detailedTranscriptView renders the full conversation transcript as a
// full-screen overlay suitable for review and copy.
func (a *App) detailedTranscriptView() string {
	width := chatWidth(a.width)

	var builder strings.Builder
	builder.WriteString(detailedTranscriptHeader(width))
	builder.WriteString("\n")
	builder.WriteString(zeroTheme.line.Render(strings.Repeat("─", width)))
	builder.WriteString("\n")

	shownAny := false
	var previousKind string

	// Build transcript rows from log entries, then render them
	rows := a.logToTranscriptRows()
	for _, row := range rows {
		if row.kind == rowWelcome {
			builder.WriteString(fitStyledLine(zeroTheme.faint.Render(row.text), width))
			builder.WriteString("\n")
			shownAny = true
			previousKind = string(row.kind)
			continue
		}
		rendered := a.renderRowDetailed(row, width)
		if rendered == "" {
			continue
		}
		if shownAny && startsTurn(row.kind) {
			builder.WriteString("\n")
		}
		if shownAny && previousKind == "user" && row.kind == rowReasoning {
			builder.WriteString("\n")
		}
		builder.WriteString(rendered)
		builder.WriteString("\n")
		shownAny = true
		previousKind = string(row.kind)
	}

	if !shownAny {
		builder.WriteString(zeroTheme.faint.Render("No transcript rows."))
		builder.WriteString("\n")
	}

	builder.WriteString(zeroTheme.line.Render(strings.Repeat("─", width)))
	builder.WriteString("\n")
	builder.WriteString(fitStyledLine(zeroTheme.faint.Render("Esc close | Ctrl+O toggle"), width))
	return builder.String()
}

// logToTranscriptRows converts log entries into transcriptRows.
func (a *App) logToTranscriptRows() []transcriptRow {
	var rows []transcriptRow
	for _, entry := range a.log {
		row := transcriptRow{
			text:      entry.Text,
			timestamp: entry.Time,
		}
		switch entry.Kind {
		case "user":
			row.kind = rowUser
		case "assistant":
			row.kind = rowAssistant
		case "tool":
			row.kind = rowToolCall
			row.tool = parseToolName(entry.Text)
			row.detail = entry.Text
		case "file":
			row.kind = rowSystem
		case "system":
			row.kind = rowSystem
		case "error":
			row.kind = rowError
		default:
			row.kind = rowSystem
		}
		row.final = true
		rows = append(rows, row)
	}
	// Add messages that aren't yet in the log
	for _, msg := range a.messages {
		kind := rowAssistant
		if msg.Role == "user" {
			kind = rowUser
		}
		// Deduplicate: skip if the last row has the same text
		if len(rows) > 0 && rows[len(rows)-1].text == msg.Content {
			continue
		}
		rows = append(rows, transcriptRow{
			kind:  kind,
			text:  msg.Content,
			final: true,
		})
	}
	return rows
}

// renderRowDetailed renders a single transcript row for the detailed view.
func (a *App) renderRowDetailed(row transcriptRow, width int) string {
	switch row.kind {
	case rowUser:
		label := a.styles.roleAssist.Render("You")
		text := a.renderer.Render(row.text)
		return label + "\n" + indentText(text, 2)
	case rowAssistant:
		label := a.styles.roleAssist.Render("Assistant")
		text := a.renderer.Render(row.text)
		return label + "\n" + indentText(text, 2)
	case rowToolCall:
		name := row.tool
		if name == "" {
			name = parseToolName(row.text)
		}
		label := a.styles.roleTool.Render("Tool: " + name)
		detail := row.detail
		if detail == "" {
			detail = row.text
		}
		if detail != "" && detail != name {
			return label + "\n" + indentText(truncateStyledLine(detail, width-4), 2)
		}
		return label
	case rowToolResult:
		label := a.styles.roleTool.Render("Result")
		text := truncateStyledLine(row.text, width-4)
		return label + "\n" + indentText(text, 2)
	case rowSystem:
		return a.styles.roleSystem.Render(row.text)
	case rowError:
		return a.styles.error.Render(row.text)
	case rowReasoning:
		label := a.styles.muted.Render("Reasoning")
		text := truncateStyledLine(row.text, width-4)
		return label + "\n" + indentText(text, 2)
	case rowRecap:
		return a.styles.muted.Render("※ " + row.text)
	default:
		return a.styles.muted.Render(row.text)
	}
}

func detailedTranscriptHeader(width int) string {
	title := zeroTheme.ink.Bold(true).Render("Transcript")
	hint := zeroTheme.faint.Render("detailed")
	return fitStyledLine(joinHeaderLine(title, hint, width), width)
}

func chatWidth(width int) int {
	if width <= 0 {
		return 88
	}
	if width < 24 {
		return 24
	}
	return width
}

func joinHeaderLine(left string, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

func indentText(text string, indent int) string {
	prefix := strings.Repeat(" ", indent)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// formatTimestamp formats a time for the detailed transcript.
func formatTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}
