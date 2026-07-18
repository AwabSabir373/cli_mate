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
		if o := renderPicker(a.picker, a.styles, a.width, a.height); o != "" {
			return wrap(o)
		}
	}

	// Detailed transcript overlay
	if a.transcriptDetailed {
		return wrap(a.detailedTranscriptView())
	}

	// Setup view (no messages, not loading)
	if a.inputMode == "" && len(a.messages) == 0 && !a.loading {
		header := a.renderHeader(computeLayout(a.width, false, false))
		return wrap(a.renderSetup(header))
	}

	// ── Rigid 4-zone grid layout (top-down sizing) ──
	// Header=2, Input=3, ToolStream=2(active)/0(idle), Body=flex
	grid := a.computeCurrentGrid()

	// Zone 1: Header (fixed 2 lines)
	header := a.renderHeaderForGrid(grid)

	// Zone 2: Body — sidebar + main transcript viewport side by side
	body := a.renderBodyZone(grid)

	// Zone 3: Tool stream strip (fixed 0 or 2 lines, above input)
	toolPane := a.renderToolExecutionPane(grid)

	// Zone 4: User input box (fixed 3 lines, permanently at bottom)
	input := a.renderInputZone(grid)

	// Assemble the rigid grid with JoinVertical
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		toolPane,
		input,
	)

	return wrap(a.renderMainPanel(content))
}

func (a App) renderMainPanel(content string) string {
	width := max(1, a.width-4)
	height := max(1, a.height-2)
	return a.styles.panel.Width(width).Height(height).Render(content)
}

func stylesVerticalDivider(styles appStyles) string {
	return styles.divider.Render(" │ ")
}

// computeCurrentGrid computes the rigid grid layout for the current frame.
// All dimensions are strictly derived from the terminal size downward.
func (a App) computeCurrentGrid() gridLayout {
	hasSidebar := a.sidebar != nil && a.sidebar.IsVisible() && a.sidebar.hasContent()
	hasPlan := a.planPanel != nil && a.planPanel.IsVisible()

	suggestions := a.suggestionLinesForGrid()
	permissions := a.permissionLinesForGrid()
	toolActive := a.loading || a.hasActiveStreamingTool()

	return computeGridLayout(
		a.width, max(1, a.height-2),
		hasSidebar, hasPlan,
		suggestions, permissions,
		toolActive,
	)
}

// suggestionLinesForGrid returns the number of lines the suggestion list
// occupies when visible, 0 otherwise.
func (a App) suggestionLinesForGrid() int {
	// Suggestions are rendered as a centered dialog inside the body instead of
	// consuming transcript rows above it.
	return 0
}

// permissionLinesForGrid returns the number of lines the permission prompt
// occupies when active, 0 otherwise.
func (a App) permissionLinesForGrid() int {
	if a.permissionPrompt == nil || !a.permissionPrompt.active {
		return 0
	}
	return permissionHeaderHeight
}

// renderHeaderForGrid renders the single-line header row for the grid.
func (a App) renderHeaderForGrid(grid gridLayout) string {
	header := a.renderHeader(layoutConfig{
		Tier:            grid.Tier,
		ShowHeaderPills: grid.ShowHeaderPills,
	})
	return fitStyledLine(header, grid.BodyWidth)
}

// renderBodyZone renders Zone 2: sidebar + main transcript viewport side by side.
func (a App) renderBodyZone(grid gridLayout) string {
	sidebarContent := ""
	if grid.ShowSidebar && a.sidebar != nil && a.sidebar.IsVisible() {
		sidebarContent = a.sidebar.Render(grid.SidebarWidth, grid.BodyHeight, a.styles)
	}

	if sidebarContent != "" {
		chatContent := a.renderChatColumnForGrid(grid)
		return lipgloss.JoinHorizontal(lipgloss.Top,
			chatContent,
			stylesVerticalDivider(a.styles),
			sidebarContent,
		)
	}

	return a.renderMainTranscriptForGrid(grid)
}

// renderChatColumnForGrid renders the main chat column for the sidebar layout.
func (a App) renderChatColumnForGrid(grid gridLayout) string {
	if len(a.currentSuggestions()) > 0 {
		return a.renderSuggestionDialog(grid.ChatWidth, grid.BodyHeight)
	}

	var b strings.Builder

	// Permission prompt (if any)
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		b.WriteString(a.permissionPrompt.render(a.styles, grid.ChatWidth))
		b.WriteString("\n")
	}

	// Transcript viewport — fills the remaining grid height
	b.WriteString(a.consoleFor(grid.ChatWidth, grid.TranscriptHeight, grid.TranscriptHeight))

	return lipgloss.NewStyle().Width(max(40, grid.ChatWidth)).Render(b.String())
}

// renderMainTranscriptForGrid renders the main transcript area without sidebar.
func (a App) renderMainTranscriptForGrid(grid gridLayout) string {
	if len(a.currentSuggestions()) > 0 {
		return a.renderSuggestionDialog(grid.ChatWidth, grid.BodyHeight)
	}

	var b strings.Builder

	// Permission prompt (if any)
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		b.WriteString(a.permissionPrompt.render(a.styles, grid.ChatWidth))
		b.WriteString("\n")
	}

	// Transcript viewport — fills the remaining grid height
	b.WriteString(a.consoleFor(grid.ChatWidth, grid.TranscriptHeight, grid.TranscriptHeight))

	// Render the complete body zone so the footer remains at the bottom and
	// blank rows overwrite any text left behind by the previous frame.
	return lipgloss.NewStyle().
		Width(max(1, grid.ChatWidth)).
		Height(max(1, grid.BodyHeight)).
		Render(b.String())
}

// renderToolExecutionPane renders Zone 3: the tool stream strip.
// This is a compact 2-line status bar statically placed above the input box.
// The main tool output flows inline in the transcript viewport.
func (a App) renderToolExecutionPane(grid gridLayout) string {
	if grid.ToolPaneHeight == 0 || a.loading || a.pending {
		return ""
	}

	var b strings.Builder

	if a.loading {
		// Line 1: spinner + current step text
		b.WriteString(a.loadingText())
		b.WriteString("\n")
		// Line 2: completed step (if any) or muted fill
		if a.completedStepText != "" {
			b.WriteString(a.styles.success.Render("✓"))
			b.WriteString(" ")
			b.WriteString(a.styles.muted.Render(a.completedStepText))
		} else {
			b.WriteString(a.styles.muted.Render(strings.Repeat("─", min(40, grid.ChatWidth))))
		}
	} else if a.streamBuffer != "" && a.streamFade != nil {
		preview := a.streamFade.render()
		if preview != "" {
			// Line 1: stream preview tail (max 1 line)
			lines := strings.Split(preview, "\n")
			lastLine := ""
			for i := len(lines) - 1; i >= 0; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					lastLine = lines[i]
					break
				}
			}
			b.WriteString(a.styles.accent.Render("▸ "))
			b.WriteString(fitStyledLine(a.styles.muted.Render(lastLine), grid.ChatWidth-2))
			b.WriteString("\n")
			// Line 2: status
			b.WriteString(a.styles.muted.Render(strings.Repeat("─", min(40, grid.ChatWidth))))
		}
	}

	out := b.String()
	if out == "" {
		return ""
	}

	// Cap to the fixed pane height (2 lines)
	if visualHeight(out) > grid.ToolPaneHeight {
		out = takeLastLines(out, grid.ToolPaneHeight)
	}

	return out
}

// renderInputZone renders Zone 4: the user input box, permanently locked at the bottom.
func (a App) renderInputZone(grid gridLayout) string {
	if a.loading || a.pending {
		return a.renderRunFooter(grid.ChatWidth)
	}
	return a.renderPromptFor(grid.ChatWidth)
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

	// Keep short terminals useful: the composer and setup action must remain
	// visible even when there is not enough room for decorative chrome.
	ultraCompact := a.height > 0 && a.height < 16
	compact := a.height > 0 && a.height < 24
	if compact {
		b.WriteString(header)
		b.WriteString("\n\n")
	}
	switch {
	case ultraCompact:
		// The header and composer are sufficient at this size. The placeholder
		// still points users to commands when horizontal space allows it.
	case compact:
		b.WriteString(a.styles.title.Render("Connect an AI provider"))
		b.WriteString("\n")
		b.WriteString(a.styles.muted.Render("Use /provider, then /model to get started."))
		b.WriteString("\n\n")
	case a.width >= 80:
		b.WriteString(renderLogo(a.styles, a.width))
	default:
		b.WriteString(renderLogoSmall(a.styles))
	}
	if !compact {
		contentWidth := max(20, a.width-10)
		rule := a.styles.divider.Render(strings.Repeat("─", min(56, contentWidth)))
		provider, model := a.welcomeConnectionDetails()

		b.WriteString("\n\n")
		b.WriteString(a.styles.subtitle.Render("          Your AI Coding Agent — " + BuildVersion))
		b.WriteString("\n\n")
		b.WriteString(rule)
		b.WriteString("\n\n")
		b.WriteString(a.styles.muted.Render("Model:      ") + a.styles.input.Render(model))
		b.WriteString("\n")
		b.WriteString(a.styles.muted.Render("Provider:   ") + a.styles.input.Render(provider))
		b.WriteString("\n")
		b.WriteString(a.styles.muted.Render("Workspace:  ") + a.styles.input.Render(a.workspaceRoot))
		b.WriteString("\n\n")
		b.WriteString(rule)
		b.WriteString("\n\n")
		b.WriteString(a.styles.accent.Render("Type a message to start"))
		b.WriteString("\n")
		b.WriteString(a.styles.accent.Render("/help") + a.styles.muted.Render("     show commands"))
		b.WriteString("\n")
		b.WriteString(a.styles.accent.Render("/model") + a.styles.muted.Render("    switch model"))
		b.WriteString("\n")
		b.WriteString(a.styles.accent.Render("/exit") + a.styles.muted.Render("     quit"))
		b.WriteString("\n")
	}

	if len(a.currentSuggestions()) > 0 {
		b.WriteString("\n")
		b.WriteString(a.renderSuggestions())
	}

	b.WriteString("\n")
	b.WriteString(a.renderPrompt())

	return a.renderMainPanel(b.String())
}

func (a App) welcomeConnectionDetails() (provider, model string) {
	provider, model = "Not connected", "Not selected"
	if a.cfg == nil {
		return provider, model
	}
	profile, err := a.cfg.Active()
	if err != nil {
		return provider, model
	}
	if profile.Provider != "" {
		provider = profile.Provider
	}
	if profile.Model != "" {
		model = profile.Model
	}
	return provider, model
}

func (a App) renderActivityStrip(width int) string {
	var b strings.Builder

	if a.streamBuffer != "" {
		b.WriteString(a.renderAssistantCard(a.streamBuffer, width, true))
		b.WriteString("\n")
	} else if a.loading {
		b.WriteString(a.renderThinkingStatus(width))
		b.WriteString("\n")
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

func (a App) activityStripBudgetLines() int {
	if a.loading {
		if a.completedStepText != "" {
			return 2
		}
		return 1
	}
	if a.streamBuffer != "" {
		return maxActivityPreviewLines
	}
	return 0
}

// consoleHeightBudget reserves space for fixed chrome so the input never scrolls off.
// In the rigid grid layout, this returns the pre-computed transcript height
// from the grid, ensuring the viewport never shifts during streaming.
func (a App) consoleHeightBudget(_ int, includeHeader bool) int {
	// Use the grid layout when available for consistent height calculation.
	if a.height > 0 {
		grid := a.computeCurrentGrid()
		return grid.TranscriptHeight
	}

	// Fallback for edge cases (zero-size terminal).
	return minTranscriptHeight
}

func (a App) console() string {
	return a.consoleFor(a.width-8, 12, 0)
}

const (
	liveToolLogKind      = "tool_live"
	liveAssistantLogKind = "assistant_live"
	thinkingLogKind      = "thinking_live"
)

func (a App) hasActiveStreamingTool() bool {
	return a.streamingTool != nil && !a.streamingTool.completed
}

func (a App) hasTranscriptContent() bool {
	return len(a.messages) > 0 || len(a.log) > 0 || a.hasActiveStreamingTool()
}

func (a App) transcriptEntries() []logEntry {
	if !a.hasActiveStreamingTool() && a.streamBuffer == "" && !a.loading {
		return a.log
	}
	entries := make([]logEntry, 0, len(a.log)+2)
	entries = append(entries, a.log...)
	if a.streamBuffer != "" {
		entries = append(entries, logEntry{Kind: liveAssistantLogKind, Text: a.streamBuffer, Time: time.Now()})
	} else if a.loading {
		entries = append(entries, logEntry{Kind: thinkingLogKind, Text: "Thinking...", Time: time.Now()})
	}
	if a.hasActiveStreamingTool() {
		entries = append(entries, logEntry{Kind: liveToolLogKind, Text: a.streamingTool.name, Time: time.Now()})
	}
	return entries
}

func (a App) consoleFor(width int, visibleLines int, heightBudget int) string {
	entries := a.transcriptEntries()

	if len(entries) == 0 {
		return ""
	}
	return a.consoleForVisualLines(entries, width, visibleLines, heightBudget)
	/*

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
			if bodyBudget > 0 && linesUsed >= bodyBudget {
				break
			}
			heightLimit := 0
			if bodyBudget > 0 {
				heightLimit = bodyBudget - linesUsed
			}
			row := a.renderLogEntryWithHeightLimit(entries[i], i, renderWidth, heightLimit)
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
		// Width is already constrained by individual entry rendering functions
		// (renderLogEntryWithHeightLimit, streamingToolCallView, etc.).
		return out
	*/
}

// consoleForVisualLines scrolls the rendered transcript by terminal rows.
// Treating each log entry as one scroll unit made multi-line cards impossible
// to inspect because a wheel tick skipped the whole entry.
func (a App) consoleForVisualLines(entries []logEntry, width, visibleLines, heightBudget int) string {
	vp := a.viewport
	if vp == nil {
		vp = newViewport()
	}
	renderWidth := max(32, width-4)

	var content strings.Builder
	for i, entry := range entries {
		row := a.renderLogEntryWithHeightLimit(entry, i, renderWidth, 0)
		content.WriteString(strings.TrimSuffix(row, "\n"))
		content.WriteString("\n")
	}
	contentLines := strings.Split(strings.TrimSuffix(content.String(), "\n"), "\n")

	if heightBudget <= 0 {
		heightBudget = visibleLines
	}
	if heightBudget <= 0 {
		heightBudget = 12
	}
	contentHeight := max(1, heightBudget-3)
	vp.setRenderedEntries(0)
	vp.setVisibleLines(contentHeight)
	vp.setTotalLines(len(contentLines))

	end := clamp(len(contentLines)-vp.scrollPos, 0, len(contentLines))
	start := max(0, end-contentHeight)

	var b strings.Builder
	if start > 0 {
		b.WriteString(a.styles.scrollHint.Render(fmt.Sprintf("... %d older lines ...", start)))
		b.WriteString("\n")
	}
	for _, line := range contentLines[start:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if end < len(contentLines) {
		b.WriteString(a.styles.scrollHint.Render(fmt.Sprintf("... %d newer lines ...", len(contentLines)-end)))
		b.WriteString("\n")
	}
	if !vp.isAtBottom() && (a.loading || a.streamBuffer != "") {
		b.WriteString(a.styles.accent.Render("  jump to latest "))
		b.WriteString(a.styles.muted.Render("(scroll down)"))
		b.WriteString("\n")
	}
	return b.String()
}

// renderLogEntry renders a single log entry with timestamp and role marker.
func (a App) renderLogEntry(entry logEntry, entryIdx int, renderWidth int) string {
	return a.renderLogEntryWithHeightLimit(entry, entryIdx, renderWidth, 0)
}

func (a App) renderLogEntryWithHeightLimit(entry logEntry, entryIdx int, renderWidth int, heightLimit int) string {
	return a.renderConversationEntry(entry, renderWidth, heightLimit)
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
	return max(1, width-6)
}

func (a App) renderPromptContent(width int) string {
	emptyHint := "Describe changes, mention @files, or type /"
	if width < 58 {
		emptyHint = "Message, @file, or /command"
	}
	if width < 26 {
		emptyHint = ""
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
		lines := strings.Split(displayInput, "\n")
		if len(lines) <= 1 {
			if displayInput == "" {
				content := a.styles.prompt.Render(">>> ") + cursor
				if emptyHint != "" {
					content += " " + emptyHint
				}
				return content
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
	isMention := false
	if _, ok := activeMentionToken(a.input); ok {
		isMention = true
	}
	title := "Commands"
	footer := "Up/Down navigate  Enter select  Esc close"
	if isMention {
		title = "Mention a file"
		footer = "Type a path with /  Up/Down navigate  Enter mention"
	}
	b.WriteString(a.styles.cardHeader.Render(title))
	b.WriteString("\n")
	b.WriteString(a.styles.muted.Render(strings.Repeat("-", max(1, width-8))))
	b.WriteString("\n")

	start := 0
	if a.selected >= maxVisibleSuggestions {
		start = a.selected - maxVisibleSuggestions + 1
	}
	end := min(len(items), start+maxVisibleSuggestions)

	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		if i == a.selected {
			prefix = "> "
		}

		// File mentions get nearly the entire dialog width. The old shared
		// 32-column cap hid the filename and most of its workspace path.
		labelWidth := clamp(width/2, 14, 32)
		if isMention {
			labelWidth = max(10, width-12)
		}
		descWidth := max(4, width-labelWidth-7)
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

	if len(items) > maxVisibleSuggestions {
		b.WriteString(a.styles.muted.Render(fmt.Sprintf("  %d-%d of %d", start+1, end, len(items))))
		b.WriteString("\n")
	}
	b.WriteString(a.styles.cardFooter.Render(footer))

	return a.styles.card.Width(max(20, width-4)).Render(b.String())
}

// renderSuggestionDialog keeps completion choices visible near the visual
// center while leaving the composer anchored at the bottom of the terminal.
func (a App) renderSuggestionDialog(width, height int) string {
	dialogWidth := clamp(width-8, 32, 88)
	dialog := a.renderSuggestionsFor(dialogWidth)
	return lipgloss.Place(
		max(1, width), max(1, height),
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

const maxVisibleSuggestions = 8

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
