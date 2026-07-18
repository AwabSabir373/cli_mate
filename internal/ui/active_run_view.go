package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (a App) renderConversationEntry(entry logEntry, width, maxHeight int) string {
	width = max(20, width)
	var rendered string
	switch entry.Kind {
	case "user":
		rendered = a.styles.prompt.Render("> ") + a.styles.input.Render(strings.TrimSpace(entry.Text))
	case "assistant":
		rendered = a.renderAssistantCard(entry.Text, width, false)
	case liveAssistantLogKind:
		rendered = a.renderAssistantCard(entry.Text, width, true)
	case thinkingLogKind:
		rendered = a.renderThinkingStatus(width)
	case "tool":
		rendered = a.renderCompletedToolStatus(entry, width)
	case liveToolLogKind:
		rendered = a.renderLiveToolStatus(width)
	case "file":
		path := strings.TrimSpace(firstLine(strings.TrimPrefix(entry.Text, "Included @")))
		rendered = a.styles.roleTool.Render("● Reading ") + a.styles.input.Render(displayPath(path))
	case completionLogKind:
		rendered = a.renderCompletionEntry(entry.Text, width, maxHeight)
	case "error":
		rendered = a.styles.error.Render("✗ " + entry.Text)
	case "system":
		rendered = a.styles.muted.Render("● " + entry.Text)
	default:
		rendered = a.styles.muted.Render(entry.Text)
	}
	if maxHeight > 0 && visualHeight(rendered) > maxHeight {
		rendered = takeLastLines(rendered, maxHeight)
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(rendered)
}

func (a App) renderAssistantCard(text string, width int, streaming bool) string {
	width = max(24, width)
	innerWidth := max(16, width-4)
	body := strings.TrimSpace(text)
	if body == "" {
		body = "Thinking..."
	}
	lines := wrapConversationText(body, innerWidth)
	if streaming && len(lines) > 0 {
		lines[len(lines)-1] += a.styles.cursor.Render("█")
	}
	content := renderMarkdown(strings.Join(lines, "\n"), innerWidth, a.styles)
	title := a.styles.accent.Render("Assistant")
	return a.styles.card.
		Width(innerWidth).
		MaxWidth(width).
		Render(title + "\n" + content)
}

func wrapConversationText(text string, width int) []string {
	if width <= 1 {
		return []string{text}
	}
	var result []string
	for _, sourceLine := range strings.Split(text, "\n") {
		if strings.TrimSpace(sourceLine) == "" {
			result = append(result, "")
			continue
		}
		var line strings.Builder
		for _, word := range strings.Fields(sourceLine) {
			wordWidth := lipgloss.Width(word)
			if line.Len() > 0 && lipgloss.Width(line.String())+1+wordWidth > width {
				result = append(result, line.String())
				line.Reset()
			}
			if line.Len() > 0 {
				line.WriteByte(' ')
			}
			line.WriteString(word)
		}
		if line.Len() > 0 {
			result = append(result, line.String())
		}
	}
	return result
}

func (a App) renderLiveToolStatus(width int) string {
	if a.streamingTool == nil {
		return ""
	}
	name := a.streamingTool.name
	target := displayPath(a.streamingTool.path)
	if target == "" {
		target = name
	}
	label := "Running"
	if isReadTool(name) {
		label = "Reading"
	} else if isFileWritingTool(name) {
		label = "Editing"
	}
	line := a.styles.roleTool.Render("● "+label+" ") + a.styles.input.Render(target)
	if label == "Running" {
		line += a.styles.muted.Render("...")
	}
	return fitStyledLine(line, width)
}

func (a App) renderCompletedToolStatus(entry logEntry, width int) string {
	name := parseToolName(entry.Text)
	target := toolDisplayTarget(entry.Text)
	if target == "" {
		target = name
	}
	if isFileWritingTool(name) {
		status := a.styles.success.Render("✓ Edited ") + a.styles.input.Render(displayPath(target))
		if diff := extractDiffBlock(entry.Text); diff != "" {
			status += "\n\n" + a.renderInlineDiffCard(target, diff, width)
		}
		return status
	}
	if isReadTool(name) {
		return a.styles.roleTool.Render("● Reading ") + a.styles.input.Render(displayPath(target))
	}
	if isVerificationTool(name, entry.Text) {
		return a.styles.roleTool.Render("● Running ") + a.styles.input.Render(truncateString(toolCommand(entry.Text), 72)) + a.styles.muted.Render("...")
	}
	return a.styles.success.Render("✓ ") + a.styles.muted.Render(humanToolName(name)+" "+displayPath(target))
}

func (a App) renderInlineDiffCard(path, diff string, width int) string {
	innerWidth := max(18, width-6)
	var lines []string
	for _, line := range viewLines(diff) {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			line = a.styles.diffAdd.Render(line)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			line = a.styles.diffRemove.Render(line)
		case strings.HasPrefix(line, "@@"):
			line = a.styles.muted.Render(line)
		}
		lines = append(lines, fitStyledLine(line, innerWidth))
		if len(lines) >= 10 {
			lines = append(lines, a.styles.muted.Render("... diff truncated"))
			break
		}
	}
	title := a.styles.accent.Render(displayPath(path))
	return a.styles.card.Width(innerWidth).MaxWidth(width).Render(title + "\n" + strings.Join(lines, "\n"))
}

func extractDiffBlock(text string) string {
	start := strings.Index(text, "```diff")
	if start < 0 {
		return ""
	}
	value := text[start+len("```diff"):]
	if end := strings.Index(value, "```"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func toolDisplayTarget(text string) string {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return ""
	}
	target := strings.Trim(fields[1], " :\"'`")
	if strings.HasPrefix(target, "{") || target == "completed" || target == "failed" {
		return ""
	}
	return target
}

func isReadTool(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "read") || strings.Contains(lower, "grep") || strings.Contains(lower, "glob") || strings.Contains(lower, "search") || strings.Contains(lower, "symbol") || strings.Contains(lower, "reference")
}

func isVerificationTool(name, text string) bool {
	if name != "shell" && name != "bash" && name != "run_terminal_command" {
		return false
	}
	lower := strings.ToLower(text)
	return strings.Contains(lower, "test") || strings.Contains(lower, "build") || strings.Contains(lower, "vet") || strings.Contains(lower, "lint")
}

func humanToolName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return "Tool"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func (a App) renderRunFooter(width int) string {
	width = max(10, width)
	rule := a.styles.divider.Render(strings.Repeat("─", width))
	hint := a.styles.muted.Render("esc to interrupt")
	if a.cancelConfirmActive {
		hint = a.styles.error.Render("esc again to interrupt")
	}
	return rule + "\n" + hint
}

func (a App) renderThinkingStatus(width int) string {
	spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := spinnerChars[a.loadingFrame%len(spinnerChars)]
	return fitStyledLine(a.styles.spinner.Render(spinner)+a.styles.muted.Render(" Thinking..."), width)
}
