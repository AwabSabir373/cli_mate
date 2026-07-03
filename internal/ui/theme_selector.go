package ui

import (
	"strings"
)

// ThemeInfo represents a theme with metadata.
type ThemeInfo struct {
	Name        string
	Description string
	Bg          string
	Fg          string
	Accent      string
}

// ThemeSelector displays available themes for selection.
type ThemeSelector struct {
	visible   bool
	themes    []ThemeInfo
	selection int
}

// NewThemeSelector creates a new theme selector.
func NewThemeSelector() *ThemeSelector {
	themes := []ThemeInfo{
		{Name: "midnight", Description: "Dark blue theme", Bg: "#1a1b26", Fg: "#c0caf5", Accent: "#7aa2f7"},
		{Name: "matrix", Description: "Green on black", Bg: "#0d1117", Fg: "#00ff41", Accent: "#00ff41"},
		{Name: "paper", Description: "Light theme", Bg: "#ffffff", Fg: "#1a1a1a", Accent: "#0066cc"},
		{Name: "mono", Description: "Monochrome", Bg: "#000000", Fg: "#ffffff", Accent: "#ffffff"},
		{Name: "catppuccin", Description: "Pastel colors", Bg: "#1e1e2e", Fg: "#cdd6f4", Accent: "#89b4fa"},
		{Name: "dracula", Description: "Purple theme", Bg: "#282a36", Fg: "#f8f8f2", Accent: "#bd93f9"},
		{Name: "nord", Description: "Arctic theme", Bg: "#2e3440", Fg: "#d8dee9", Accent: "#88c0d0"},
		{Name: "gruvbox", Description: "Retro groove", Bg: "#282828", Fg: "#ebdbb2", Accent: "#fe8019"},
		{Name: "tokyonight", Description: "Tokyo night", Bg: "#1a1b26", Fg: "#c0caf5", Accent: "#7aa2f7"},
		{Name: "rosepine", Description: "Rose pine", Bg: "#191724", Fg: "#e0def4", Accent: "#ebbcba"},
		{Name: "solarized", Description: "Solarized dark", Bg: "#002b36", Fg: "#839496", Accent: "#268bd2"},
		{Name: "onedark", Description: "One dark", Bg: "#282c34", Fg: "#abb2bf", Accent: "#61afef"},
		{Name: "everforest", Description: "Everforest green", Bg: "#2d353b", Fg: "#d3c6aa", Accent: "#83c092"},
	}
	return &ThemeSelector{themes: themes}
}

// Toggle shows/hides the theme selector.
func (ts *ThemeSelector) Toggle() {
	ts.visible = !ts.visible
}

// SetVisible sets the visibility of the theme selector.
func (ts *ThemeSelector) SetVisible(visible bool) {
	ts.visible = visible
}

// IsVisible returns whether the theme selector is visible.
func (ts *ThemeSelector) IsVisible() bool {
	return ts.visible
}

// MoveUp moves the selection up.
func (ts *ThemeSelector) MoveUp() {
	if ts.selection > 0 {
		ts.selection--
	}
}

// MoveDown moves the selection down.
func (ts *ThemeSelector) MoveDown() {
	if ts.selection < len(ts.themes)-1 {
		ts.selection++
	}
}

// GetSelected returns the currently selected theme name.
func (ts *ThemeSelector) GetSelected() string {
	if ts.selection >= 0 && ts.selection < len(ts.themes) {
		return ts.themes[ts.selection].Name
	}
	return ""
}

// Render produces the theme selector view.
func (ts *ThemeSelector) Render(width int, styles appStyles) string {
	if !ts.visible {
		return ""
	}

	var lines []string
	lines = append(lines, styles.pill.Render("Select Theme"))
	lines = append(lines, "")

	for i, theme := range ts.themes {
		icon := "  "
		if i == ts.selection {
			icon = "▸ "
		}

		name := theme.Name
		desc := " - " + theme.Description

		if i == ts.selection {
			lines = append(lines, styles.selected.Render(icon+name)+styles.muted.Render(desc))
		} else {
			lines = append(lines, icon+styles.accent.Render(name)+styles.muted.Render(desc))
		}
	}

	lines = append(lines, "")
	lines = append(lines, styles.muted.Render("↑/↓ to navigate, Enter to select"))

	return strings.Join(lines, "\n")
}

// FormatThemeList returns a compact list of theme names for autocomplete.
func (ts *ThemeSelector) FormatThemeList() string {
	names := make([]string, len(ts.themes))
	for i, t := range ts.themes {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
