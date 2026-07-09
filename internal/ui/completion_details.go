package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"cli_mate/internal/providers"
)

const completionLogKind = "completion"

func (a App) completionDetails(messages []providers.Message, previousMessageCount int, err error) string {
	if err != nil {
		return ""
	}
	answer := latestAssistantAnswer(messages, previousMessageCount)
	if strings.TrimSpace(answer) == "" {
		return ""
	}

	runEntries := a.runEntries()
	files := completionFiles(runEntries)
	commands := completionCommands(runEntries)
	toolCounts := completionToolCounts(runEntries)

	var b strings.Builder
	b.WriteString("Summary\n")
	b.WriteString("- ")
	b.WriteString(completionSummary(answer))
	b.WriteString("\n")

	if len(files) > 0 {
		b.WriteString("\nFiles\n")
		for _, file := range limitStrings(files, 6) {
			b.WriteString("- ")
			b.WriteString(file)
			b.WriteString("\n")
		}
		if len(files) > 6 {
			b.WriteString(fmt.Sprintf("- ... %d more files\n", len(files)-6))
		}
	}

	if len(toolCounts) > 0 {
		b.WriteString("\nActions\n")
		b.WriteString("- Used ")
		b.WriteString(formatToolCounts(toolCounts))
		b.WriteString(".\n")
	}

	b.WriteString("\nVerification\n")
	if len(commands) > 0 {
		for _, cmd := range limitStrings(commands, 4) {
			b.WriteString("- ")
			b.WriteString(cmd)
			b.WriteString("\n")
		}
		if len(commands) > 4 {
			b.WriteString(fmt.Sprintf("- ... %d more commands\n", len(commands)-4))
		}
	} else {
		b.WriteString("- No verification command was reported in this run.\n")
	}

	if elapsed := time.Since(a.turnStartedAt).Round(time.Second); elapsed > 0 {
		b.WriteString("\nRun\n")
		b.WriteString("- Finished in ")
		b.WriteString(elapsed.String())
		b.WriteString(".\n")
	}

	return strings.TrimSpace(b.String())
}

func (a App) runEntries() []logEntry {
	if a.runLogStart < 0 {
		return a.log
	}
	if a.runLogStart >= len(a.log) {
		return nil
	}
	return a.log[a.runLogStart:]
}

func latestAssistantAnswer(messages []providers.Message, previousMessageCount int) string {
	if previousMessageCount < 0 {
		previousMessageCount = 0
	}
	if previousMessageCount > len(messages) {
		previousMessageCount = len(messages)
	}
	for i := len(messages) - 1; i >= previousMessageCount; i-- {
		if messages[i].Role == "assistant" && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i].Content
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i].Content
		}
	}
	return ""
}

func completionSummary(answer string) string {
	for _, line := range viewLines(answer) {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#-*0123456789. ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch strings.ToLower(line) {
		case "summary", "files", "files changed", "verification", "notes":
			continue
		}
		return truncateString(line, 180)
	}
	return "The assistant completed the task."
}

func completionFiles(entries []logEntry) []string {
	var files []string
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Kind != "tool" && entry.Kind != "file" {
			continue
		}
		path := parseToolPath(entry.Text)
		if path == "" {
			args := parseToolArgsJSON(parseToolArg(entry.Text))
			path = extractArgString(args, "path", "file_path", "file", "filename", "pattern")
		}
		if path == "" && entry.Kind == "file" {
			path = firstLine(strings.TrimPrefix(entry.Text, "Included @"))
		}
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, path)
	}
	return files
}

func completionCommands(entries []logEntry) []string {
	var commands []string
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Kind != "tool" {
			continue
		}
		name := parseToolName(entry.Text)
		if name != "shell" && name != "bash" && name != "run_terminal_command" {
			continue
		}
		cmd := toolCommand(entry.Text)
		if cmd == "" || seen[cmd] {
			continue
		}
		seen[cmd] = true
		commands = append(commands, cmd)
	}
	return commands
}

func toolCommand(text string) string {
	args := parseToolArgsJSON(parseToolArg(text))
	cmd := extractArgString(args, "command", "cmd", "script")
	if cmd != "" {
		return firstLine(cmd)
	}
	fields := strings.Fields(text)
	if len(fields) <= 1 {
		return ""
	}
	return firstLine(strings.Join(fields[1:], " "))
}

func completionToolCounts(entries []logEntry) map[string]int {
	counts := map[string]int{}
	for _, entry := range entries {
		if entry.Kind != "tool" {
			continue
		}
		name := parseToolName(entry.Text)
		if name == "" || isHiddenPlumbingTool(name) {
			continue
		}
		counts[name]++
	}
	return counts
}

func formatToolCounts(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		if counts[name] == 1 {
			parts = append(parts, name)
		} else {
			parts = append(parts, fmt.Sprintf("%s x%d", name, counts[name]))
		}
	}
	return strings.Join(parts, ", ")
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, "\n"); idx >= 0 {
		value = value[:idx]
	}
	return truncateString(strings.TrimSpace(value), 120)
}

func (a App) renderCompletionEntry(text string, width int, maxHeight int) string {
	contentWidth := max(20, width-8)
	body := fitStyledBlock(text, contentWidth, 0)
	title := a.styles.success.Render("Task complete")
	out := title + "\n" + body
	style := a.styles.softPanel.Width(contentWidth).MaxWidth(width)
	if maxHeight > 0 {
		style = style.MaxHeight(maxHeight)
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(style.Render(out))
}
