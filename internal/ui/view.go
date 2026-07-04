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

	// Overlay views take full screen (priority ordering)
	if a.onboarding != nil && a.onboarding.isActive() {
		overlay := a.renderOverlay(a.onboarding, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.sessionPicker != nil && a.sessionPicker.isVisible() {
		overlay := a.renderSessionPickerOverlay(a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.mcpManager != nil && a.mcpManager.isVisible() {
		overlay := a.renderMCPOverlay(a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.specMode != nil && a.specMode.isVisible() {
		overlay := renderSpecMode(a.specMode, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.subchatManager != nil && a.subchatManager.isActive() {
		overlay := renderSubchat(a.subchatManager, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.prStatus != nil && a.prStatus.isVisible() {
		overlay := renderPRStatus(a.prStatus, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	// Additional overlay components
	if a.startup != nil && a.startup.isVisible() {
		overlay := renderStartup(a.startup, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.sessionCtrls != nil && a.sessionCtrls.isVisible() {
		overlay := renderSessionControls(a.sessionCtrls, a.styles, a.width, a.messages)
		if overlay != "" {
			return overlay
		}
	}

	if a.commandOutput != nil && a.commandOutput.isVisible() {
		overlay := renderCommandOutput(a.commandOutput, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.doctor != nil && a.doctor.isVisible() {
		overlay := renderDoctorView(a.doctor, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.imageAttach != nil && a.imageAttach.isVisible() {
		overlay := renderImageAttachments(a.imageAttach, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	if a.picker != nil && a.picker.isVisible() {
		overlay := renderPicker(a.picker, a.styles, a.width)
		if overlay != "" {
			return overlay
		}
	}

	layout := computeLayout(a.width, a.sidebar != nil && a.sidebar.hasContent(), a.planPanel != nil && a.planPanel.IsVisible())

	header := a.renderHeader(layout)

	if a.inputMode == "" && len(a.messages) == 0 && !a.loading {
		return a.renderSetup(header)
	}

	var b strings.Builder

	// Build main chat column
	chatContent := a.renderChatContent(layout)

	// Sidebar
	sidebarContent := ""
	if layout.ShowSidebar && a.sidebar != nil && a.sidebar.IsVisible() {
		sidebarContent = a.sidebar.Render(layout.SidebarWidth, a.height-6, a.styles)
	}

	if sidebarContent != "" {
		// Two-column layout
		b.WriteString(a.styles.panel.Width(a.width - 4).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				chatContent,
				stylesVerticalDivider(a.styles),
				sidebarContent,
			),
		))
	} else {
		// Single column
		b.WriteString(a.renderPanelContent(header, layout))
	}

	return b.String()
}

func stylesVerticalDivider(styles appStyles) string {
	return styles.muted.Render(" ┃ ")
}

func (a App) renderPanelContent(header string, layout layoutConfig) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	// Chat transcript
	if len(a.messages) > 0 || len(a.log) > 0 {
		b.WriteString(a.console())
		b.WriteString("\n")
	}

	// Streaming tool call preview
	if a.streamingTool != nil && !a.streamingTool.completed {
		toolView := streamingToolCallView(a.streamingTool, a.styles, layout.ChatWidth)
		if toolView != "" {
			b.WriteString(toolView)
			b.WriteString("\n")
		}
	}

	// Loading indicator
	if a.loading {
		b.WriteString(a.loadingText())
		b.WriteString("\n\n")
	} else if a.streamBuffer != "" && !a.loading {
		// Streaming response preview with fade
		preview := a.streamFade.render()
		if preview != "" {
			b.WriteString(preview)
			b.WriteString("\n\n")
		}
	}

	// Suggestions
	if len(a.currentSuggestions()) > 0 {
		b.WriteString(a.renderSuggestions())
		b.WriteString("\n")
	}

	// Permission prompt
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		b.WriteString(a.permissionPrompt.render(a.styles, layout.ChatWidth))
		b.WriteString("\n\n")
	}

	// Input prompt
	b.WriteString(a.renderPrompt())

	return a.styles.panel.Width(a.width - 4).Render(b.String())
}

func (a App) renderHeader(layout layoutConfig) string {
	logo := a.styles.logo.Render("cli_mate")
	bits := []string{logo}

	profile, err := a.cfg.Active()
	if err == nil {
		if layout.ShowHeaderPills {
			if profile.Provider != "" {
				bits = append(bits, a.styles.pill.Render(profile.Provider))
			}
			if profile.Model != "" {
				bits = append(bits, a.styles.pill.Render(profile.Model))
			}
		}
	}

	if a.workspaceName != "" && layout.ShowHeaderPills {
		bits = append(bits, a.styles.pill.Render(a.workspaceName))
	}

	if a.theme != "" && layout.ShowHeaderPills {
		bits = append(bits, a.styles.pill.Render(a.theme))
	}

	// Token/message count
	if a.tokensUsed > 0 {
		bits = append(bits, a.styles.tokenCount.Render(fmt.Sprintf("~%d tokens", a.tokensUsed)))
	} else if usage := a.tokenUsage(); usage != "" {
		bits = append(bits, usage)
	} else if len(a.messages) > 0 {
		bits = append(bits, a.styles.muted.Render(fmt.Sprintf("%d msgs", len(a.messages))))
	}

	return a.styles.pillRow.Render(strings.Join(bits, " "))
}

func (a App) renderSetup(header string) string {

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	// Render ASCII art logo based on terminal width
	if a.width >= 80 {
		b.WriteString(renderLogo(a.styles, a.width))
	} else {
		b.WriteString(renderLogoSmall(a.styles))
	}
	b.WriteString("\n\n")

	b.WriteString(a.styles.heroBorder.Width(a.width - 6).Render(
		lipgloss.JoinVertical(lipgloss.Left,
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

func (a App) renderChatContent(layout layoutConfig) string {
	renderWidth := max(40, layout.ChatWidth)
	var b strings.Builder

	// Show transcript messages
	if len(a.messages) > 0 || len(a.log) > 0 {
		b.WriteString(a.console())
		b.WriteString("\n")
	}

	// Show streaming tool call if active
	if a.streamingTool != nil && !a.streamingTool.completed {
		toolView := streamingToolCallView(a.streamingTool, a.styles, renderWidth)
		if toolView != "" {
			b.WriteString(toolView)
			b.WriteString("\n")
		}
	}

	// Show loading state
	if a.loading {
		b.WriteString(a.loadingText())
		b.WriteString("\n\n")
	} else if a.streamBuffer != "" {
		// Fade-rendered streaming preview
		preview := a.streamFade.render()
		if preview != "" {
			b.WriteString(preview)
			b.WriteString("\n\n")
		}
	}

	// Suggestions
	if len(a.currentSuggestions()) > 0 {
		b.WriteString(a.renderSuggestions())
		b.WriteString("\n")
	}

	// Permission prompt
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		b.WriteString(a.permissionPrompt.render(a.styles, renderWidth))
		b.WriteString("\n\n")
	}

	// Input prompt
	b.WriteString(a.renderPrompt())

	return b.String()
}

func (a App) console() string {
	entries := a.log

	if len(entries) == 0 {
		return ""
	}

	// Use viewport for scroll management
	vp := a.viewport
	visibleLines := 12
	vp.setVisibleLines(visibleLines)
	vp.setTotalLines(len(entries))

	start, end := vp.visibleRange()

	visible := entries[start:end]

	var b strings.Builder
	if start > 0 {
		b.WriteString(a.styles.scrollHint.Render(fmt.Sprintf("... %d older entries ...\n", start)))
	}

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

		entryIdx := start + i
		rendered := ""
		if entry.renderedText != "" && entry.renderWidth == renderWidth {
			rendered = entry.renderedText
		} else {
			// Use custom markdown renderer for assistant entries
			if entry.Kind == "assistant" {
				rendered = renderMarkdown(entry.Text, renderWidth, a.styles)
			} else if entry.Kind == "tool" {
				cardRendered := a.renderToolEntry(entry, renderWidth)
				if cardRendered != "" {
					rendered = cardRendered
				} else {
					rendered = a.renderer.Render(entry.Text)
				}
			} else {
				rendered = a.renderer.Render(entry.Text)
			}
			a.log[entryIdx].renderedText = rendered
			a.log[entryIdx].renderWidth = renderWidth
		}

		b.WriteString(fmt.Sprintf("%s %s %s", timeStr, marker, rendered))
		b.WriteString("\n")
	}

	if end < len(entries) {
		b.WriteString(a.styles.scrollHint.Render(fmt.Sprintf("... %d more entries ...\n", len(entries)-end)))
	}

	// Show scroll position indicator
	scrollIndicator := vp.renderScrollIndicator(renderWidth, a.styles)
	if scrollIndicator != "" {
		b.WriteString(scrollIndicator)
		b.WriteString("\n")
	}

	// Show scroll-to-bottom hint when scrolled up during active streaming
	if !vp.isAtBottom() && (a.loading || a.streamBuffer != "") {
		b.WriteString(a.styles.accent.Render("  ▼ jump to latest "))
		b.WriteString(a.styles.muted.Render("(scroll down)"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderToolEntry renders a tool log entry using the tool card registry.
func (a App) renderToolEntry(entry logEntry, width int) string {
	if a.toolCardRegistry == nil {
		return ""
	}

	req := toolBodyRequest{
		name:   parseToolName(entry.Text),
		arg:    parseToolArg(entry.Text),
		detail: entry.Text,
		path:   parseToolPath(entry.Text),
	}

	// Skip hidden plumbing tools
	if isHiddenPlumbingTool(req.name) {
		return ""
	}

	return a.toolCardRegistry.renderCard(req, a.styles, width)
}

// parseToolName extracts the tool name from a log entry text.
func parseToolName(text string) string {
	fields := strings.Fields(text)
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// parseToolArg extracts the tool arguments from a log entry text.
func parseToolArg(text string) string {
	fields := strings.Fields(text)
	if len(fields) > 1 {
		return strings.Join(fields[1:], " ")
	}
	return ""
}

// parseToolPath attempts to extract a file path from tool args.
func parseToolPath(text string) string {
	if strings.Contains(text, "path=") {
		parts := strings.Split(text, "path=")
		if len(parts) > 1 {
			path := strings.TrimSpace(parts[1])
			if idx := strings.IndexAny(path, " ,}\n"); idx >= 0 {
				path = path[:idx]
			}
			return strings.Trim(path, "\"'`")
		}
	}
	if strings.Contains(text, "path:") {
		parts := strings.Split(text, "path:")
		if len(parts) > 1 {
			path := strings.TrimSpace(parts[1])
			if idx := strings.IndexAny(path, " ,}\n"); idx >= 0 {
				path = path[:idx]
			}
			return strings.Trim(path, "\"'`")
		}
	}
	return ""
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
	case "ask_user":
		if a.askUserState != nil {
			return a.askUserState.render(a.styles, a.promptWidth())
		}
		return ""
	default:
		displayInput := a.input
		if strings.HasPrefix(displayInput, "/api-key ") && len(displayInput) > 9 {
			displayInput = "/api-key " + strings.Repeat("*", len(displayInput)-9)
		}
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

	currentStep := a.currentStepText
	if currentStep == "" && len(a.loadingSteps) > 0 {
		stepIndex := a.loadingFrame / 4
		if stepIndex >= len(a.loadingSteps) {
			stepIndex = len(a.loadingSteps) - 1
		}
		currentStep = a.loadingSteps[stepIndex]
	}

	var b strings.Builder

	if a.completedStepText != "" {
		b.WriteString(a.styles.success.Render("✓"))
		b.WriteString(" ")
		b.WriteString(a.styles.muted.Render(a.completedStepText))
		b.WriteString("\n")
	}

	b.WriteString(a.styles.spinner.Render(spinner))
	b.WriteString(" ")
	b.WriteString(a.styles.accent.Render(currentStep))

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
	cleaned := strings.TrimSpace(s)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	truncated := cleaned[len(cleaned)-maxLen:]
	if idx := strings.Index(truncated, " "); idx >= 0 && idx < 20 {
		truncated = truncated[idx+1:]
	}
	return "..." + truncated
}

func (a App) tokenUsage() string {
	total := 0
	for _, msg := range a.messages {
		total += len(msg.Content) / 4
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

// renderOverlay renders the onboarding wizard as a full-screen overlay.
func (a App) renderOverlay(os *onboardingState, width int) string {
	if os == nil || !os.isActive() {
		return ""
	}
	overlay := os.render(a.styles, width)
	if overlay == "" {
		return ""
	}
	return a.styles.panel.Width(width - 4).Render(overlay)
}

// renderSessionPickerOverlay renders the session picker as a full-screen overlay.
func (a App) renderSessionPickerOverlay(width int) string {
	if a.sessionPicker == nil || !a.sessionPicker.isVisible() {
		return ""
	}
	overlay := renderSessionPicker(a.sessionPicker, a.styles, width)
	if overlay == "" {
		return ""
	}
	return a.styles.panel.Width(width - 4).Render(overlay)
}

// renderMCPOverlay renders the MCP manager as a full-screen overlay.
func (a App) renderMCPOverlay(width int) string {
	if a.mcpManager == nil || !a.mcpManager.isVisible() {
		return ""
	}
	overlay := renderMCPManager(a.mcpManager, a.styles, width)
	if overlay == "" {
		return ""
	}
	return a.styles.panel.Width(width - 4).Render(overlay)
}

func defaultEntry(kind, text string) logEntry {
	return logEntry{Kind: kind, Text: text, Time: time.Now()}
}
