package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (a App) View() tea.View {
	if a.err != nil {
		return tea.View{Content: a.styles.error.Render(a.err.Error()) + "\n"}
	}

	// Overlay views take full screen (priority ordering)
	wrap := func(s string) tea.View {
		return tea.View{Content: s, AltScreen: true, MouseMode: tea.MouseModeCellMotion}
	}
	if a.onboarding != nil && a.onboarding.isActive() {
		if o := a.renderOverlay(a.onboarding, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.sessionPicker != nil && a.sessionPicker.isVisible() {
		if o := a.renderSessionPickerOverlay(a.width); o != "" {
			return wrap(o)
		}
	}
	if a.mcpManager != nil && a.mcpManager.isVisible() {
		if o := a.renderMCPOverlay(a.width); o != "" {
			return wrap(o)
		}
	}
	if a.specMode != nil && a.specMode.isVisible() {
		if o := renderSpecMode(a.specMode, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.subchatManager != nil && a.subchatManager.isActive() {
		if o := renderSubchat(a.subchatManager, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.prStatus != nil && a.prStatus.isVisible() {
		if o := renderPRStatus(a.prStatus, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.startup != nil && a.startup.isVisible() {
		if o := renderStartup(a.startup, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.sessionCtrls != nil && a.sessionCtrls.isVisible() {
		if o := renderSessionControls(a.sessionCtrls, a.styles, a.width, a.messages); o != "" {
			return wrap(o)
		}
	}
	if a.commandOutput != nil && a.commandOutput.isVisible() {
		if o := renderCommandOutput(a.commandOutput, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.doctor != nil && a.doctor.isVisible() {
		if o := renderDoctorView(a.doctor, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.imageAttach != nil && a.imageAttach.isVisible() {
		if o := renderImageAttachments(a.imageAttach, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}
	if a.picker != nil && a.picker.isVisible() {
		if o := renderPicker(a.picker, a.styles, a.width); o != "" {
			return wrap(o)
		}
	}

	// Detailed transcript overlay
	if a.transcriptDetailed {
		return wrap(a.detailedTranscriptView())
	}

	layout := computeLayout(a.width, a.sidebar != nil && a.sidebar.hasContent(), a.planPanel != nil && a.planPanel.IsVisible())

	header := a.renderHeader(layout)

	if a.inputMode == "" && len(a.messages) == 0 && !a.loading {
		return wrap(a.renderSetup(header))
	}

	var b strings.Builder

	sidebarContent := ""
	if layout.ShowSidebar && a.sidebar != nil && a.sidebar.IsVisible() {
		sidebarContent = a.sidebar.Render(layout.SidebarWidth, a.height-6, a.styles)
	}

	if sidebarContent != "" {
		chatContent := a.renderChatColumn(header, layout)
		b.WriteString(a.styles.panel.Width(a.width - 4).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				chatContent,
				stylesVerticalDivider(a.styles),
				sidebarContent,
			),
		))
	} else {
		b.WriteString(a.renderPanelContent(header, layout))
	}

	return wrap(b.String())
}

func stylesVerticalDivider(styles appStyles) string {
	return styles.muted.Render(" | ")
}

func (a App) renderPanelContent(header string, layout layoutConfig) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	availableHeight := a.consoleHeightBudget(layout.ChatWidth, true)

	// Chat transcript — flexible region above fixed chrome
	if len(a.messages) > 0 || len(a.log) > 0 {
		b.WriteString(a.consoleFor(layout.ChatWidth, layout.ConsoleLines, availableHeight))
		b.WriteString("\n")
	}

	// Suggestions
	if len(a.currentSuggestions()) > 0 {
		b.WriteString(a.renderSuggestionsFor(layout.ChatWidth))
		b.WriteString("\n")
	}

	// Permission prompt
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		b.WriteString(a.permissionPrompt.render(a.styles, layout.ChatWidth))
		b.WriteString("\n\n")
	}

	// Activity (tools / loading) stays compact above the input
	b.WriteString(a.renderActivityStrip(layout.ChatWidth))

	// Input stays a small fixed footer portion
	b.WriteString(a.renderPromptFor(layout.ChatWidth))

	return a.styles.panel.Width(a.width - 4).Render(b.String())
}

func (a App) renderHeader(layout layoutConfig) string {
	logo := a.styles.logo.Render("cli_mate")
	bits := []string{logo}

	if a.cfg != nil && layout.ShowHeaderPills {
		profile, err := a.cfg.Active()
		if err == nil {
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
	if a.cfg == nil {
		return ""
	}
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

	availableHeight := a.consoleHeightBudget(renderWidth, false)

	// Show transcript messages
	if len(a.messages) > 0 || len(a.log) > 0 {
		b.WriteString(a.consoleFor(renderWidth, layout.ConsoleLines, availableHeight))
		b.WriteString("\n")
	}

	// Suggestions
	if len(a.currentSuggestions()) > 0 {
		b.WriteString(a.renderSuggestionsFor(renderWidth))
		b.WriteString("\n")
	}

	// Permission prompt
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		b.WriteString(a.permissionPrompt.render(a.styles, renderWidth))
		b.WriteString("\n\n")
	}

	b.WriteString(a.renderActivityStrip(renderWidth))

	// Input prompt — always a small portion at the bottom
	b.WriteString(a.renderPromptFor(renderWidth))

	return b.String()
}

func (a App) renderChatColumn(header string, layout layoutConfig) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(a.renderChatContent(layout))
	return lipgloss.NewStyle().Width(max(40, layout.ChatWidth)).Render(b.String())
}

func (a App) renderActivityStrip(width int) string {
	var b strings.Builder

	if a.streamingTool != nil && !a.streamingTool.completed {
		toolView := streamingToolCallView(a.streamingTool, a.styles, width)
		if toolView != "" {
			b.WriteString(toolView)
			b.WriteString("\n")
		}
	}

	if a.loading {
		b.WriteString(a.loadingText())
		b.WriteString("\n")
	} else if a.streamBuffer != "" && a.streamFade != nil {
		preview := a.streamFade.render()
		if preview != "" {
			// Keep stream preview compact so the input stays visible
			b.WriteString(takeLastLines(preview, maxActivityPreviewLines))
			b.WriteString("\n")
		}
	}

	out := b.String()
	if out == "" {
		return ""
	}
	// Hard-cap activity chrome so tools never push the input off-screen
	if visualHeight(out) > maxActivityStripLines {
		out = takeLastLines(out, maxActivityStripLines)
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
	}
	return out
}

const (
	// panelChromeLines accounts for outer panel border (2) + padding (2).
	panelChromeLines = 4
	// minConsoleHeight keeps at least a few transcript lines visible.
	minConsoleHeight = 3
	// maxPromptContentLines keeps the composer a small bottom portion.
	maxPromptContentLines = 3
	// maxActivityStripLines caps live tool/loading chrome above the input.
	maxActivityStripLines = 8
	// maxActivityPreviewLines caps streaming text previews.
	maxActivityPreviewLines = 4
	// mouseWheelScrollStep moves several log entries per wheel notch.
	mouseWheelScrollStep = 3
)

func visualHeight(s string) int {
	if s == "" {
		return 0
	}
	return lipgloss.Height(s)
}

func takeLastLines(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return "…" + "\n" + strings.Join(lines[len(lines)-n+1:], "\n")
}

func (a App) promptContentLineCount() int {
	lines := strings.Count(a.input, "\n") + 1
	if lines < 1 {
		lines = 1
	}
	if lines > maxPromptContentLines {
		lines = maxPromptContentLines
	}
	return lines
}

// promptChromeLines is content lines + inputPanel top/bottom border.
func (a App) promptChromeLines() int {
	return a.promptContentLineCount() + 2
}

// consoleHeightBudget reserves space for fixed chrome so the input never scrolls off.
func (a App) consoleHeightBudget(width int, includeHeader bool) int {
	h := a.height
	if h <= 0 {
		h = 30
	}

	reserved := panelChromeLines
	if includeHeader {
		reserved += 2 // header row + blank
	}
	reserved += a.promptChromeLines()

	if n := len(a.currentSuggestions()); n > 0 {
		reserved += n + 1
	}
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		// Estimate permission block without depending on full render side effects.
		reserved += 4
	}

	// Activity strip is rendered after the console; reserve a compact slice.
	activityReserve := 0
	if a.loading || (a.streamingTool != nil && !a.streamingTool.completed) || a.streamBuffer != "" {
		activityReserve = maxActivityStripLines
	}
	reserved += activityReserve
	reserved += 2 // section spacing / scroll hints

	available := h - reserved
	if available < minConsoleHeight {
		available = minConsoleHeight
	}
	return available
}

func (a App) console() string {
	return a.consoleFor(a.width-8, 12, 0)
}

func (a App) consoleFor(width int, visibleLines int, heightBudget int) string {
	entries := a.log

	if len(entries) == 0 {
		return ""
	}

	vp := a.viewport
	if vp == nil {
		vp = newViewport()
	}
	if visibleLines <= 0 {
		visibleLines = 12
	}
	// Prefer a window large enough for the height budget so bottom-up packing
	// can include more short entries before tall tool cards cap the view.
	if heightBudget > visibleLines {
		visibleLines = heightBudget
	}
	vp.setVisibleLines(visibleLines)
	vp.setTotalLines(len(entries))

	renderWidth := max(32, width-4)
	hintReserve := 0
	if heightBudget > 0 {
		hintReserve = 2
	}
	bodyBudget := heightBudget
	if bodyBudget > hintReserve {
		bodyBudget -= hintReserve
	}

	// Build candidate rows (newest first after scrollPos), then pack bottom-up
	// so the latest tool/read output stays visible and the input stays put.
	end := vp.packWindowEnd()
	type packedRow struct {
		idx  int
		text string
	}
	var packed []packedRow
	linesUsed := 0

	for i := end - 1; i >= 0; i-- {
		row := a.renderLogEntry(entries[i], i, renderWidth)
		entryLines := visualHeight(row)
		if entryLines < 1 {
			entryLines = 1
		}

		if bodyBudget > 0 && linesUsed+entryLines > bodyBudget {
			remaining := bodyBudget - linesUsed
			if remaining <= 0 {
				break
			}
			// Keep the newest slice of a tall entry so tool/read tails stay useful.
			row = takeLastLines(row, remaining)
			entryLines = visualHeight(row)
			if entryLines < 1 {
				break
			}
			packed = append(packed, packedRow{idx: i, text: row})
			linesUsed += entryLines
			break
		}

		packed = append(packed, packedRow{idx: i, text: row})
		linesUsed += entryLines
	}

	// packed is newest→oldest; reverse for chronological display
	for i, j := 0, len(packed)-1; i < j; i, j = i+1, j-1 {
		packed[i], packed[j] = packed[j], packed[i]
	}

	var b strings.Builder
	startIdx := 0
	endIdx := end
	if len(packed) > 0 {
		startIdx = packed[0].idx
		endIdx = packed[len(packed)-1].idx + 1
	}

	if startIdx > 0 {
		b.WriteString(a.styles.scrollHint.Render(fmt.Sprintf("... %d older entries ...", startIdx)))
		b.WriteString("\n")
	}

	for _, row := range packed {
		b.WriteString(row.text)
		if !strings.HasSuffix(row.text, "\n") {
			b.WriteString("\n")
		}
	}

	vp.setRenderedEntries(len(packed))

	if endIdx < len(entries) {
		b.WriteString(a.styles.scrollHint.Render(fmt.Sprintf("... %d more entries ...", len(entries)-endIdx)))
		b.WriteString("\n")
	}

	if scrollIndicator := vp.renderScrollIndicator(renderWidth, a.styles); scrollIndicator != "" {
		b.WriteString(scrollIndicator)
		b.WriteString("\n")
	}

	if !vp.isAtBottom() && (a.loading || a.streamBuffer != "") {
		b.WriteString(a.styles.accent.Render("  jump to latest "))
		b.WriteString(a.styles.muted.Render("(scroll down)"))
		b.WriteString("\n")
	}

	out := b.String()
	if heightBudget > 0 && visualHeight(out) > heightBudget {
		out = takeLastLines(out, heightBudget)
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
	}
	return out
}

// renderLogEntry renders a single log entry with timestamp and role marker.
func (a App) renderLogEntry(entry logEntry, entryIdx int, renderWidth int) string {
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

	entryRendered := ""
	if entry.renderedText != "" && entry.renderWidth == renderWidth {
		entryRendered = entry.renderedText
	} else {
		switch entry.Kind {
		case "assistant":
			entryRendered = renderMarkdown(entry.Text, renderWidth, a.styles)
		case "tool":
			cardRendered := a.renderToolEntry(entry, renderWidth)
			if cardRendered != "" {
				entryRendered = cardRendered
			} else if a.renderer != nil {
				entryRendered = a.renderer.Render(entry.Text)
			} else {
				entryRendered = entry.Text
			}
		default:
			if a.renderer != nil {
				entryRendered = a.renderer.Render(entry.Text)
			} else {
				entryRendered = entry.Text
			}
		}
		if entryIdx >= 0 && entryIdx < len(a.log) {
			a.log[entryIdx].renderedText = entryRendered
			a.log[entryIdx].renderWidth = renderWidth
		}
	}

	return fmt.Sprintf("%s %s %s", timeStr, marker, entryRendered)
}

// renderToolEntry renders a tool log entry using the tool card registry.
func (a App) renderToolEntry(entry logEntry, width int) string {
	if a.toolCardRegistry == nil {
		return ""
	}

	req := toolBodyRequest{
		name:     parseToolName(entry.Text),
		arg:      parseToolArg(entry.Text),
		detail:   entry.Text,
		path:     parseToolPath(entry.Text),
		argsJSON: parseToolArgsJSON(parseToolArg(entry.Text)),
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
	return a.renderPromptFor(a.width - 4)
}

func (a App) renderPromptFor(width int) string {
	promptWidth := a.promptWidthFor(width)
	return a.styles.inputPanel.Width(promptWidth).Render(a.renderPromptContent(promptWidth))
}

func (a App) promptWidth() int {
	return a.promptWidthFor(a.width - 4)
}

func (a App) promptWidthFor(width int) int {
	width = width - 6
	if width < 32 {
		return 32
	}
	return width
}

func (a App) renderPromptContent(width int) string {
	emptyHint := "Describe changes, mention @files, or type /"
	if width < 58 {
		emptyHint = "Message, @file, or /command"
	}
	emptyHint = a.styles.muted.Render(emptyHint)
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
			return a.askUserState.render(a.styles, width)
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

		// Keep multi-line composer small: show a window around the cursor line.
		cursorLineIdx := 0
		cursorLocal := a.cursorPos
		for i, line := range lines {
			if cursorLocal <= len(line) {
				cursorLineIdx = i
				break
			}
			cursorLocal -= len(line) + 1 // +1 for newline
			if i == len(lines)-1 {
				cursorLineIdx = i
			}
		}
		startLine := 0
		if len(lines) > maxPromptContentLines {
			startLine = cursorLineIdx - maxPromptContentLines + 1
			if startLine < 0 {
				startLine = 0
			}
			if startLine+maxPromptContentLines > len(lines) {
				startLine = len(lines) - maxPromptContentLines
			}
		}
		endLine := startLine + maxPromptContentLines
		if endLine > len(lines) {
			endLine = len(lines)
		}

		var b strings.Builder
		for i := startLine; i < endLine; i++ {
			line := lines[i]
			prefix := a.styles.prompt.Render("... ")
			if i == 0 {
				prefix = a.styles.prompt.Render(">>> ")
			}
			if i == cursorLineIdx {
				left := line
				right := ""
				if cursorLocal <= len(line) {
					left = line[:cursorLocal]
					right = line[cursorLocal:]
				}
				b.WriteString(prefix)
				b.WriteString(a.styles.input.Render(left))
				b.WriteString(cursor)
				b.WriteString(a.styles.input.Render(right))
			} else {
				b.WriteString(prefix)
				b.WriteString(a.styles.input.Render(line))
			}
			if i < endLine-1 {
				b.WriteString("\n")
			}
		}
		return b.String()
	}
}

func (a App) renderSuggestions() string {
	return a.renderSuggestionsFor(a.width - 4)
}

func (a App) renderSuggestionsFor(width int) string {
	items := a.currentSuggestions()
	if len(items) == 0 {
		return ""
	}

	var b strings.Builder
	for i, item := range items {
		prefix := "  "
		if i == a.selected {
			prefix = "> "
		}

		labelWidth := clamp(width/2, 14, 32)
		descWidth := max(10, width-labelWidth-8)
		label := truncateString(item.Label, labelWidth)
		desc := truncateString(item.Description, descWidth)
		if desc == "" {
			desc = "command"
		}
		line := fmt.Sprintf("%s%-*s  %s", prefix, labelWidth, label, desc)

		if i == a.selected {
			b.WriteString(a.styles.selected.Render(line))
		} else {
			b.WriteString(a.styles.muted.Render(line))
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
	if a.streamBuffer != "" && !a.reducedMotion {
		palette := ripplePalette()
		rippled := rippleText(currentStep, palette, a.loadingFrame, 8)
		b.WriteString(rippled)
	} else {
		b.WriteString(a.styles.accent.Render(currentStep))
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
