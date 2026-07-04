package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// outputSeverity categorizes command output.
type outputSeverity string

const (
	outputInfo    outputSeverity = "info"
	outputSuccess outputSeverity = "success"
	outputWarning outputSeverity = "warning"
	outputError   outputSeverity = "error"
	outputDebug   outputSeverity = "debug"
)

// commandOutputEntry represents a single command execution result.
type commandOutputEntry struct {
	id        string
	command   string
	output    string
	exitCode  int
	severity  outputSeverity
	duration  time.Duration
	timestamp time.Time
	lines     int
}

// commandOutputView manages the command output display panel.
type commandOutputView struct {
	visible      bool
	entries      []commandOutputEntry
	cursor       int
	scrollOff    int
	expanded     map[int]bool
	filter       string // filter by command name
	showTimings  bool
	autoScroll   bool
}

// newCommandOutputView creates a new command output view.
func newCommandOutputView() *commandOutputView {
	return &commandOutputView{
		expanded:    make(map[int]bool),
		autoScroll:  true,
		showTimings: true,
	}
}

// show opens the command output panel.
func (cov *commandOutputView) show() {
	cov.visible = true
	cov.cursor = 0
	cov.scrollOff = 0
}

// hide closes the command output panel.
func (cov *commandOutputView) hide() {
	cov.visible = false
}

// isVisible returns true if the panel is visible.
func (cov *commandOutputView) isVisible() bool {
	return cov.visible
}

// addEntry adds a new command output entry.
func (cov *commandOutputView) addEntry(id, command, output string, exitCode int, duration time.Duration) {
	entry := commandOutputEntry{
		id:        id,
		command:   command,
		output:    output,
		exitCode:  exitCode,
		timestamp: time.Now(),
		duration:  duration,
		lines:     strings.Count(output, "\n") + 1,
	}

	if exitCode == 0 {
		entry.severity = outputSuccess
	} else {
		entry.severity = outputError
	}

	cov.entries = append(cov.entries, entry)

	if cov.autoScroll {
		cov.cursor = len(cov.entries) - 1
		cov.scrollToEnd()
	}
}

// handleKey processes key events for navigation and actions.
func (cov *commandOutputView) handleKey(key string) string {
	if !cov.visible || len(cov.entries) == 0 {
		return ""
	}

	switch key {
	case "up", "shift+tab":
		if cov.cursor > 0 {
			cov.cursor--
		}
		cov.adjustScroll()
	case "down", "tab":
		if cov.cursor < len(cov.entries)-1 {
			cov.cursor++
		}
		cov.adjustScroll()
	case "enter", " ":
		if cov.cursor >= 0 && cov.cursor < len(cov.entries) {
			cov.expanded[cov.cursor] = !cov.expanded[cov.cursor]
		}
	case "esc":
		cov.hide()
		return "close"
	case "c":
		// Copy selected output
		if cov.cursor >= 0 && cov.cursor < len(cov.entries) {
			entry := cov.entries[cov.cursor]
			text := fmt.Sprintf("$ %s\n%s", entry.command, entry.output)
			_ = clipboard.WriteAll(text)
			return "copied"
		}
	case "t":
		cov.showTimings = !cov.showTimings
	case "delete", "backspace":
		if cov.cursor >= 0 && cov.cursor < len(cov.entries) {
			cov.entries = append(cov.entries[:cov.cursor], cov.entries[cov.cursor+1:]...)
			if cov.cursor >= len(cov.entries) {
				cov.cursor = len(cov.entries) - 1
			}
		}
	case "home":
		cov.cursor = 0
		cov.scrollOff = 0
	case "end":
		cov.cursor = len(cov.entries) - 1
		cov.adjustScroll()
	}

	return ""
}

func (cov *commandOutputView) adjustScroll() {
	maxVisible := 8
	if cov.cursor < cov.scrollOff {
		cov.scrollOff = cov.cursor
	}
	if cov.cursor >= cov.scrollOff+maxVisible {
		cov.scrollOff = cov.cursor - maxVisible + 1
	}
}

func (cov *commandOutputView) scrollToEnd() {
	maxVisible := 8
	if len(cov.entries) > maxVisible {
		cov.scrollOff = len(cov.entries) - maxVisible
	} else {
		cov.scrollOff = 0
	}
}

// renderCommandOutput renders the command output panel.
func renderCommandOutput(cov *commandOutputView, styles appStyles, width int) string {
	if !cov.visible || len(cov.entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(fmt.Sprintf(" Command Output: %d entries ", len(cov.entries))))
	b.WriteString("\n\n")

	maxVisible := 8
	start := cov.scrollOff
	end := start + maxVisible
	if end > len(cov.entries) {
		end = len(cov.entries)
	}

	for i := start; i < end; i++ {
		entry := cov.entries[i]

		// Severity indicator
		var indicator string
		switch entry.severity {
		case outputSuccess:
			indicator = styles.success.Render("✓")
		case outputError:
			indicator = styles.error.Render("✗")
		case outputWarning:
			indicator = styles.accent.Render("⚠")
		default:
			indicator = styles.muted.Render("•")
		}

		// Expand indicator
		expandIcon := "▶"
		if cov.expanded[i] {
			expandIcon = "▼"
		}

		// Command line
		cmd := entry.command
		if len(cmd) > 40 {
			cmd = cmd[:40] + "..."
		}

		line := fmt.Sprintf("%s %s %s", indicator, expandIcon, cmd)

		if cov.showTimings && entry.duration > 0 {
			durationStr := fmt.Sprintf("(%s)", entry.duration.Round(time.Millisecond))
			line += " " + styles.muted.Render(durationStr)
		}

		if i == cov.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", line)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", line))
			b.WriteString("\n")
		}

		// Show output if expanded
		if cov.expanded[i] {
			output := entry.output
			const maxOutputLines = 15
			outputLines := strings.Split(output, "\n")
			if len(outputLines) > maxOutputLines {
				output = strings.Join(outputLines[:maxOutputLines], "\n")
				output += fmt.Sprintf("\n... %d more lines ...", len(outputLines)-maxOutputLines)
			}
			if output != "" {
				b.WriteString(styles.softPanel.Width(width-12).Render(
					styles.code.Render(output)),
				)
				b.WriteString("\n")
			}
			b.WriteString(styles.muted.Render(fmt.Sprintf("   exit: %d · %d lines", entry.exitCode, entry.lines)))
			b.WriteString("\n")
		}
	}

	if len(cov.entries) > maxVisible {
		b.WriteString("\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  ... %d more entries ...", len(cov.entries)-maxVisible)))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter expand · C copy · T timings · Del remove · Esc close"))
	b.WriteString("\n")
	return styles.panel.Width(width-4).Render(b.String())
}

// formatBashOutput formats raw bash output for display.
func formatBashOutput(output string, maxLines int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	// Show first 5 and last 5 lines
	head := lines[:5]
	tail := lines[len(lines)-5:]
	return strings.Join(head, "\n") +
		fmt.Sprintf("\n  ... %d lines omitted ...\n", len(lines)-10) +
		strings.Join(tail, "\n")
}

// severityIcon returns a styled icon for the severity level.
func severityIcon(severity outputSeverity) string {
	switch severity {
	case outputSuccess:
		return "✓"
	case outputError:
		return "✗"
	case outputWarning:
		return "⚠"
	default:
		return "•"
	}
}
