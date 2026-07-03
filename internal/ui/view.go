package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (a App) View() string {
	if a.err != nil {
		return a.styles.error.Render(a.err.Error()) + "\n"
	}

	header := a.renderHeader()

	if a.inputMode == "" && len(a.messages) == 0 && !a.loading {
		return a.renderSetup(header)
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	if len(a.messages) > 0 {
		b.WriteString(a.console())
		b.WriteString("\n")
	}

	if a.loading {
		b.WriteString(a.loadingText())
		b.WriteString("\n\n")
	}

	if len(a.currentSuggestions()) > 0 {
		b.WriteString(a.renderSuggestions())
		b.WriteString("\n")
	}

	b.WriteString(a.renderPrompt())
	return a.styles.panel.Width(a.width - 4).Render(b.String())
}

func (a App) renderSetup(header string) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	b.WriteString(a.styles.heroBorder.Width(a.width - 6).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			a.styles.title.Render("cli_mate"),
			a.styles.subtitle.Render("Terminal-first AI coding agent"),
			"",
			a.styles.accent.Render("01")+"  "+a.styles.muted.Render("Choose a provider          /provider"),
			a.styles.accent.Render("02")+"  "+a.styles.muted.Render("Paste your API key         prompted automatically"),
			a.styles.accent.Render("03")+"  "+a.styles.muted.Render("Pick a model               /model"),
			a.styles.accent.Render("04")+"  "+a.styles.muted.Render("Start chatting             type anything"),
		),
	))

	b.WriteString("\n")

	b.WriteString(a.renderWorkspacePills())
	b.WriteString("\n\n")

	if len(a.log) > 0 {
		b.WriteString(a.console())
	}

	if len(a.currentSuggestions()) > 0 {
		b.WriteString("\n")
		b.WriteString(a.renderSuggestions())
	}

	b.WriteString("\n")
	b.WriteString(a.renderPrompt())

	return a.styles.panel.Width(a.width - 4).Render(b.String())
}

func (a App) renderHeader() string {
	logo := a.styles.logo.Render("cli_mate")
	bits := []string{logo}

	profile, err := a.cfg.Active()
	if err == nil {
		provider := profile.Provider
		if provider != "" {
			bits = append(bits, a.styles.pill.Render(provider))
		}
		if profile.Model != "" {
			bits = append(bits, a.styles.pill.Render(profile.Model))
		}
	}

	if a.workspaceName != "" {
		bits = append(bits, a.styles.pill.Render(a.workspaceName))
	}

	if a.theme != "" {
		bits = append(bits, a.styles.pill.Render(a.theme))
	}

	if a.tokensUsed > 0 {
		bits = append(bits, a.styles.tokenCount.Render(fmt.Sprintf("~%d tokens", a.tokensUsed)))
	} else if usage := a.tokenUsage(); usage != "" {
		bits = append(bits, usage)
	} else {
		bits = append(bits, a.styles.muted.Render(fmt.Sprintf("session %d msgs", len(a.messages))))
	}

	return a.styles.pillRow.Render(strings.Join(bits, " "))
}

func (a App) renderWorkspacePills() string {
	profile, err := a.cfg.Active()
	if err != nil {
		return ""
	}
	pill := func(label, value string) string {
		if value == "" {
			return a.styles.muted.Render(label + " not set")
		}
		return a.styles.pill.Render(value)
	}
	return lipgloss.JoinHorizontal(lipgloss.Center,
		pill("provider", profile.Provider),
		" ",
		pill("model", profile.Model),
		" ",
		pill("theme", a.theme),
	)
}

func (a App) console() string {
	entries := a.log
	scrollOffset := a.scrollOffset

	if len(entries) == 0 {
		return ""
	}

	// Calculate visible window
	visibleLines := 12
	start := len(entries) - visibleLines - scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(entries) {
		end = len(entries)
	}

	visible := entries[start:end]

	var b strings.Builder
	if start > 0 {
		b.WriteString(a.styles.muted.Render(fmt.Sprintf("... %d older entries (scroll with Alt+Up/Down) ...\n", start)))
	}

	// Calculate render width once for all entries
	renderWidth := max(40, a.width-8)

	for i, entry := range visible {
		ts := entry.Time.Format("15:04:05")
		timeStr := a.styles.logTime.Render(ts)

		var marker string
		switch entry.Kind {
		case "tool":
			marker = a.styles.roleTool.Render("tool")
		case "file":
			marker = a.styles.roleFile.Render("file")
		case "system":
			marker = a.styles.roleSystem.Render("system")
		case "assistant":
			marker = a.styles.roleAssist.Render("assistant")
		case "error":
			marker = a.styles.error.Render("error")
		default:
			marker = a.styles.muted.Render(entry.Kind)
		}
		marker = a.styles.logPrefix.Render(marker)

		// Use cached render if available and width hasn't changed
		entryIdx := start + i
		rendered := ""
		if entry.renderedText != "" && entry.renderWidth == renderWidth {
			rendered = entry.renderedText
		} else {
			rendered = a.renderer.Render(entry.Text)
			// Cache the rendered text in the entry
			a.log[entryIdx].renderedText = rendered
			a.log[entryIdx].renderWidth = renderWidth
		}

		b.WriteString(fmt.Sprintf("%s %s %s", timeStr, marker, rendered))
		b.WriteString("\n")
	}

	if end < len(entries) {
		b.WriteString(a.styles.muted.Render(fmt.Sprintf("... %d more entries ...\n", len(entries)-end)))
	}

	return b.String()
}

func (a App) renderPrompt() string {
	return a.styles.inputPanel.Width(a.promptWidth()).Render(a.renderPromptContent())
}

func (a App) promptWidth() int {
	width := a.width - 12
	if width < 32 {
		return 32
	}
	return width
}

func (a App) renderPromptContent() string {
	emptyHint := a.styles.muted.Render("Describe changes, mention @files, or type /")
	cursor := a.styles.cursor.Render("█")

	switch a.inputMode {
	case "api_key":
		masked := strings.Repeat("*", len(a.input))
		return a.styles.prompt.Render("api key: ") + a.styles.input.Render(masked) + a.styles.cursor.Render(" ")
	case "custom_model":
		if a.cursorPos < len(a.input) {
			left := a.input[:a.cursorPos]
			right := a.input[a.cursorPos:]
			return a.styles.prompt.Render("model id: ") + a.styles.input.Render(left) + cursor + a.styles.input.Render(right)
		}
		return a.styles.prompt.Render("model id: ") + a.styles.input.Render(a.input) + cursor
	case "finder":
		return a.renderFinder()
	default:
		// Mask API key if user types /api-key inline
		displayInput := a.input
		if strings.HasPrefix(displayInput, "/api-key ") && len(displayInput) > 9 {
			displayInput = "/api-key " + strings.Repeat("*", len(displayInput)-9)
		}
		// Multiline input: show cursor on the correct line
		lines := strings.Split(displayInput, "\n")
		if len(lines) <= 1 {
			if displayInput == "" {
				return a.styles.prompt.Render(">>> ") + cursor + " " + emptyHint
			}
			if a.cursorPos < len(displayInput) {
				left := displayInput[:a.cursorPos]
				right := displayInput[a.cursorPos:]
				return a.styles.prompt.Render(">>> ") + a.styles.input.Render(left) + cursor + a.styles.input.Render(right)
			}
			return a.styles.prompt.Render(">>> ") + a.styles.input.Render(displayInput) + cursor
		}

		var b strings.Builder
		cursorLine := -1
		cursorLocal := a.cursorPos
		for i, line := range lines {
			prefix := a.styles.prompt.Render(">>> ")
			if i > 0 {
				prefix = a.styles.prompt.Render(" · ")
			}
			if cursorLine == -1 && cursorLocal <= len(line) {
				cursorLine = i
				left := line[:cursorLocal]
				right := line[cursorLocal:]
				b.WriteString(prefix)
				b.WriteString(a.styles.input.Render(left))
				b.WriteString(cursor)
				b.WriteString(a.styles.input.Render(right))
			} else {
				b.WriteString(prefix)
				b.WriteString(a.styles.input.Render(line))
			}
			if i < len(lines)-1 {
				b.WriteString("\n")
			}
		}

		// Cursor at very end
		if cursorLine == -1 {
			b.WriteString(a.styles.prompt.Render(" · "))
			b.WriteString(cursor)
		}

		return b.String()
	}
}

func (a App) renderSuggestions() string {
	items := a.currentSuggestions()
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	for i, item := range items {
		if i == a.selected {
			b.WriteString(a.styles.selected.Render(item.Label))
		} else {
			b.WriteString(a.styles.muted.Render(item.Label))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (a App) loadingText() string {
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := spinnerChars[a.loadingFrame%len(spinnerChars)]

	// Show the real-time step from tool calls if available
	currentStep := a.currentStepText
	if currentStep == "" && len(a.loadingSteps) > 0 {
		stepIndex := a.loadingFrame / 4
		if stepIndex >= len(a.loadingSteps) {
			stepIndex = len(a.loadingSteps) - 1
		}
		currentStep = a.loadingSteps[stepIndex]
	}

	var b strings.Builder

	// Show completed step with checkmark
	if a.completedStepText != "" {
		b.WriteString(a.styles.success.Render("✓"))
		b.WriteString(" ")
		b.WriteString(a.styles.muted.Render(a.completedStepText))
		b.WriteString("\n")
	}

	// Show current step with spinner
	b.WriteString(a.styles.accent.Render(spinner))
	b.WriteString(" ")
	b.WriteString(a.styles.accent.Render(currentStep))

	// Show a truncated preview of streaming tokens if any
	if a.streamBuffer != "" {
		preview := truncateStreamPreview(a.streamBuffer, 80)
		if preview != "" {
			b.WriteString("\n")
			b.WriteString(a.styles.muted.Render("  " + preview))
		}
	}

	return b.String()
}

func truncateStreamPreview(s string, maxLen int) string {
	// Take the last meaningful portion of the stream
	cleaned := strings.TrimSpace(s)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	// Try to find a good break point
	truncated := cleaned[len(cleaned)-maxLen:]
	if idx := strings.Index(truncated, " "); idx >= 0 && idx < 20 {
		truncated = truncated[idx+1:]
	}
	return "..." + truncated
}

func (a App) tokenUsage() string {
	total := 0
	for _, msg := range a.messages {
		total += len(msg.Content) / 4 // rough estimate
	}
	if total == 0 {
		return ""
	}
	return a.styles.tokenCount.Render(fmt.Sprintf("~%d tokens", total))
}

func (a App) renderFinder() string {
	query := strings.ToLower(a.input)
	var matches []string
	for _, f := range a.files {
		if query == "" || strings.Contains(strings.ToLower(f), query) {
			matches = append(matches, f)
		}
		if len(matches) >= 20 {
			break
		}
	}

	var b strings.Builder
	b.WriteString(a.styles.accent.Render(" 🔍 Find File "))
	b.WriteString("\n\n")

	cursor := a.styles.cursor.Render("█")
	b.WriteString(a.styles.prompt.Render("   "))
	if a.cursorPos < len(a.input) {
		left := a.input[:a.cursorPos]
		right := a.input[a.cursorPos:]
		b.WriteString(a.styles.input.Render(left))
		b.WriteString(cursor)
		b.WriteString(a.styles.input.Render(right))
	} else {
		b.WriteString(a.styles.input.Render(a.input))
		b.WriteString(cursor)
	}
	b.WriteString("\n\n")

	if len(matches) == 0 {
		b.WriteString(a.styles.muted.Render("  No matches found"))
	} else {
		for i, m := range matches {
			if i == a.selected%len(matches) {
				b.WriteString(a.styles.selected.Render(" " + m + " "))
				b.WriteString("\n")
			} else {
				kind := "📄"
				if strings.HasSuffix(m, "/") {
					kind = "📁"
				}
				b.WriteString(fmt.Sprintf("  %s %s\n", kind, m))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(a.styles.muted.Render("  Type to filter · Enter to open · Esc to close"))
	return a.styles.panel.Width(a.width - 4).Render(b.String())
}

func defaultEntry(kind, text string) logEntry {
	return logEntry{Kind: kind, Text: text, Time: time.Now()}
}
