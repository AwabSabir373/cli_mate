package ui

// layoutTier represents the adaptive layout tier based on terminal width.
type layoutTier int

const (
	tierTiny   layoutTier = iota // < 58 columns
	tierNarrow                   // 58-79 columns
	tierMedium                   // 80-99 columns
	tierFull                     // >= 100 columns
)

const (
	tierTinyWidth   = 58
	tierNarrowWidth = 80
	tierMediumWidth = 100
)

// widthTier returns the layout tier for the given terminal width.
func widthTier(width int) layoutTier {
	switch {
	case width < tierTinyWidth:
		return tierTiny
	case width < tierNarrowWidth:
		return tierNarrow
	case width < tierMediumWidth:
		return tierMedium
	default:
		return tierFull
	}
}

// layoutConfig contains layout parameters for the current tier.
type layoutConfig struct {
	Tier            layoutTier
	ShowSidebar     bool
	ShowPlanPanel   bool
	ShowHeaderPills bool
	ConsoleLines    int
	SidebarWidth    int
	ChatWidth       int
}

type viewportHeightConfig struct {
	PanelChromeLines int
	HeaderLines      int
	PromptLines      int
	SuggestionLines  int
	PermissionLines  int
	ActivityLines    int
	SpacingLines     int
	MinLines         int
}

func computeViewportHeight(windowHeight int, cfg viewportHeightConfig) int {
	reserved := cfg.PanelChromeLines +
		cfg.HeaderLines +
		cfg.PromptLines +
		cfg.SuggestionLines +
		cfg.PermissionLines +
		cfg.ActivityLines +
		cfg.SpacingLines

	available := windowHeight - reserved
	if available < cfg.MinLines {
		return cfg.MinLines
	}
	return available
}

// computeLayout computes the layout configuration for the given width and state.
func computeLayout(width int, hasSidebarContent bool, hasPlanContent bool) layoutConfig {
	tier := widthTier(width)
	cfg := layoutConfig{
		Tier:            tier,
		ShowHeaderPills: true,
		ConsoleLines:    12,
		SidebarWidth:    0,
		ChatWidth:       width - 4,
	}

	switch tier {
	case tierTiny:
		cfg.ShowSidebar = false
		cfg.ShowPlanPanel = false
		cfg.ShowHeaderPills = false
		cfg.ConsoleLines = 8
		cfg.ChatWidth = width - 4

	case tierNarrow:
		cfg.ShowSidebar = false
		cfg.ShowPlanPanel = hasPlanContent
		cfg.ConsoleLines = 10
		cfg.ChatWidth = width - 4

	case tierMedium:
		cfg.ShowSidebar = hasSidebarContent
		cfg.ShowPlanPanel = hasPlanContent
		cfg.ConsoleLines = 12
		if cfg.ShowSidebar {
			cfg.SidebarWidth = 26
		} else {
			cfg.SidebarWidth = 0
		}
		cfg.ChatWidth = width - cfg.SidebarWidth - 6

	case tierFull:
		cfg.ShowSidebar = hasSidebarContent
		cfg.ShowPlanPanel = hasPlanContent
		cfg.ConsoleLines = 14
		if cfg.ShowSidebar {
			cfg.SidebarWidth = 30
		} else {
			cfg.SidebarWidth = 0
		}
		cfg.ChatWidth = width - cfg.SidebarWidth - 6
	}

	return cfg
}

// sidebarAvailable returns true if there's enough room for a sidebar.
func sidebarAvailable(width int) bool {
	return width >= tierNarrowWidth
}

// chatColumnWidth returns the width available for the chat column.
func chatColumnWidth(width int, sidebarWidth int) int {
	chatWidth := width - 4
	if sidebarWidth > 0 {
		chatWidth = width - sidebarWidth - 6
	}
	if chatWidth < 40 {
		chatWidth = 40
	}
	return chatWidth
}

// ──────────────────────────────────────────────────────────────────────
// Rigid 4-zone layout grid constants (top-down sizing)
//
// Component dimensions are strictly assigned from the window down.
// Fixed variables — never change regardless of content:
//
//   Header Height           = 2
//   Input Pane Height       = 3
//   Tool Stream Strip Height = 2 (active) / 0 (idle)
//
// Dynamic calculations:
//   Remaining Body Height = Total − Header − Input − ToolStream
//   Sidebar Width = 30 (or 20% of total width, whichever is smaller)
//   Viewport Width = Total − Sidebar
//
// The input box is permanently locked at the bottom. The transcript
// viewport fills all remaining space.
// ──────────────────────────────────────────────────────────────────────

const (
	// headerHeight is the fixed height of the top header bar.
	// Always 2 lines: one content row + one separator gap.
	headerHeight = 2
	// inputPaneHeight is the fixed height of the user input box.
	// Always 3 lines: top border + content row + bottom border.
	inputPaneHeight = 3
	// toolStreamStripHeightActive is the fixed height of the tool
	// stream strip when a tool is actively streaming or loading.
	toolStreamStripHeightActive = 2
	// minTranscriptHeight is the minimum lines for the transcript viewport.
	minTranscriptHeight = 3
	// suggestionHeaderHeight is the fixed height allocated for the
	// suggestion list (rendered inside the body zone).
	suggestionHeaderHeight = 2
	// permissionHeaderHeight is the fixed height allocated for the
	// permission prompt (rendered inside the body zone).
	permissionHeaderHeight = 4
	// sidebarFixedWidth is the preferred sidebar width in characters.
	sidebarFixedWidth = 30
	// sidebarMaxRatio caps the sidebar at 20% of terminal width.
	sidebarMaxRatio = 0.2
	// panelBorderChrome accounts for the outer panel border (top+bottom)
	// and padding (top+bottom) used by the layout-level border.
	panelBorderChrome = 4
)

// gridLayout describes the rigid 4-zone layout for the current frame.
// All dimensions are computed top-down from the terminal size.
type gridLayout struct {
	// Fixed zone heights (top-down from window)
	HeaderHeight     int // always headerHeight (2)
	ToolPaneHeight   int // 0 when idle, toolStreamStripHeightActive (2) when active
	InputHeight      int // always inputPaneHeight (3)
	BodyHeight       int // flex: fills remaining space
	TranscriptHeight int // body height minus suggestion/permission chrome

	// Horizontal dimensions (top-down from window)
	TotalWidth   int // total terminal width
	BodyWidth    int // total body width inside the panel
	SidebarWidth int // width of the sidebar (0 if hidden)
	ChatWidth    int // width of the main chat column

	// State flags
	ShowSidebar     bool
	ShowPlanPanel   bool
	ShowHeaderPills bool
	Tier            layoutTier
}

// computeGridLayout computes the rigid grid layout for the current frame.
// All dimensions are strictly assigned from the terminal size downward.
func computeGridLayout(width, height int, hasSidebarContent, hasPlanContent bool,
	suggestionLines, permissionLines int,
	toolPaneActive bool) gridLayout {

	tier := widthTier(width)

	// ── Fixed zone heights (top-down) ──
	headerH := headerHeight
	inputH := inputPaneHeight
	toolH := 0
	if toolPaneActive {
		toolH = toolStreamStripHeightActive
	}

	// ── Remaining body height: total − header − input − tool ──
	bodyH := height - headerH - toolH - inputH
	if bodyH < minTranscriptHeight {
		bodyH = minTranscriptHeight
	}

	// ── Horizontal split (top-down from width) ──
	showSidebar := false
	sidebarW := 0
	showPlan := hasPlanContent
	showPills := true

	switch tier {
	case tierTiny:
		showSidebar = false
		showPlan = false
		showPills = false
	case tierNarrow:
		showSidebar = false
	case tierMedium:
		showSidebar = hasSidebarContent
		if showSidebar {
			sidebarW = min(sidebarFixedWidth, int(float64(width)*sidebarMaxRatio))
		}
	case tierFull:
		showSidebar = hasSidebarContent
		if showSidebar {
			sidebarW = min(sidebarFixedWidth, int(float64(width)*sidebarMaxRatio))
		}
	}

	// ── Viewport width: total − sidebar (with minimum) ──
	chatW := width - 4 // default: full width minus panel chrome
	if showSidebar && sidebarW > 0 {
		chatW = width - sidebarW - 6 // sidebar + divider + padding
	}
	if chatW < 40 {
		chatW = 40
	}

	bodyW := width - 4 // total body width inside the panel

	// ── Transcript height: body minus suggestion/permission chrome ──
	transcriptH := bodyH - suggestionLines - permissionLines
	if transcriptH < minTranscriptHeight {
		transcriptH = minTranscriptHeight
	}

	return gridLayout{
		HeaderHeight:     headerH,
		ToolPaneHeight:   toolH,
		InputHeight:      inputH,
		BodyHeight:       bodyH,
		TranscriptHeight: transcriptH,
		TotalWidth:       width,
		BodyWidth:        bodyW,
		SidebarWidth:     sidebarW,
		ChatWidth:        chatW,
		ShowSidebar:      showSidebar,
		ShowPlanPanel:    showPlan,
		ShowHeaderPills:  showPills,
		Tier:             tier,
	}
}
