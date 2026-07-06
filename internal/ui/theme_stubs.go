package ui

// rebuildStreamingFadePalette rebuilds the streaming fade palette after a theme change.
// Called by applyTheme when the theme is swapped.
func rebuildStreamingFadePalette() {
	// The streaming fade state is per-App instance and recreated on theme change.
	// This stub satisfies the theme_select.go reference from zero's TUI.
}

// defaultRenderCache is a package-level render cache reference used by theme_select.go.
// In cli_mate, the render cache lives on the App struct, but theme_select.go
// references this to clear cached entries after a theme swap.
var defaultRenderCache *renderCache
