package ui

// layoutTier represents the adaptive layout tier based on terminal width.
type layoutTier int

const (
	tierTiny   layoutTier = iota // < 58 columns
	tierNarrow                    // 58-79 columns
	tierMedium                    // 80-99 columns
	tierFull                      // >= 100 columns
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
