package ui

import "strings"

// themeMode is the operator's palette preference.
type themeMode string

const (
	themeAuto  themeMode = "auto" // detect terminal background (default)
	themeDark  themeMode = "dark"
	themeLight themeMode = "light"
)

// themeModes lists the values /theme accepts, in picker order: `auto` first, then
// every registered theme (theme_palettes.go). It is the single ordered source
// feeding both the picker and the /theme state list — adding a registry entry
// extends it automatically.
var themeModes = append([]string{string(themeAuto)}, themeNames()...)

// resolveThemeMode picks the first accepted preference from candidates in
// precedence order — the caller passes them highest-first: the --theme flag, then
// ZERO_THEME, then the persisted config theme. A value is accepted if it is `auto`
// or names a registered theme; unrecognized/blank values are skipped, and an empty
// list (or all-unrecognized) falls back to auto.
func resolveThemeMode(candidates ...string) themeMode {
	for _, v := range candidates {
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "" {
			continue
		}
		if s == string(themeAuto) {
			return themeAuto
		}
		if _, ok := lookupTheme(s); ok {
			return themeMode(s)
		}
	}
	return themeAuto
}

// validThemeMode reports whether s names a theme mode (for /theme validation):
// `auto` or any registered theme.
func validThemeMode(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == string(themeAuto) {
		return true
	}
	_, ok := lookupTheme(s)
	return ok
}

// ValidThemeArg reports whether s is an acceptable --theme / ZERO_THEME value
// (`auto` or a registered theme name). Exported so the CLI flag validator shares
// this one source of truth instead of hardcoding the theme list.
func ValidThemeArg(s string) bool { return validThemeMode(s) }

// applyTheme swaps the active palette (zeroTheme) and the globals derived from it
// — the streaming-fade ramp and the static render cache — so a switch repaints
// every subsequent render. For themeAuto it resolves to dark/light from
// hasDarkBackground; explicit dark/light ignore it. Returns the concrete mode
// applied (never auto). Must run on the Bubble Tea update goroutine (or before the
// program starts), like every other zeroTheme access.
func applyTheme(mode themeMode, hasDarkBackground bool) themeMode {
	resolved := mode
	if mode == themeAuto {
		resolved = themeDark
		if !hasDarkBackground {
			resolved = themeLight
		}
	}
	// Resolve the (now concrete) mode to its registered palette; an unknown name
	// falls back to the dark built-in so a bad value can never leave zeroTheme unset.
	entry, ok := lookupTheme(string(resolved))
	if !ok {
		entry, _ = lookupTheme(string(themeDark))
	}
	zeroTheme = buildTheme(entry.Palette)
	rebuildStreamingFadePalette()
	if defaultRenderCache != nil {
		defaultRenderCache.clear() // old-palette entries must not be reused
	}
	return resolved
}

// previewSelectedTheme makes the live palette match the theme picker's current
// state. Called on every change to the picker's selection or filter so the
// whole UI renders the mode the popup points at.
func (a *App) previewSelectedTheme() {
	if a.picker == nil {
		return
	}
	if gp := a.picker; gp != nil && gp.cursor >= 0 && gp.cursor < len(gp.items) {
		item := gp.items[gp.cursor]
		applyTheme(themeMode(item.Value), a.hasDarkBg)
		return
	}
	a.restoreCommittedTheme()
}

// restoreCommittedTheme re-applies the committed theme after a preview is dismissed
// without choosing (Esc).
func (a *App) restoreCommittedTheme() {
	applyTheme(a.themeMode, a.hasDarkBg)
}

// handleThemeCommand implements /theme [name]: `list` shows state, a registered
// theme name (or `auto`) switches the active palette live.
func (a *App) handleThemeCommand(args string) {
	arg := strings.ToLower(strings.TrimSpace(args))
	if arg == "" || arg == "list" {
		a.appendLog("system", a.themeStateText())
		return
	}
	if !validThemeMode(arg) {
		a.appendLog("error", "Unknown theme: "+arg+" (use /theme with no argument to pick from the list)")
		return
	}
	a.themeMode = themeMode(arg)
	resolved := applyTheme(a.themeMode, a.hasDarkBg)
	active := arg
	if a.themeMode == themeAuto {
		active = "auto (" + string(resolved) + ")"
	}
	a.appendLog("system", "Active theme: "+active)
	a.saveSettings()
}

// themeStateText renders the /theme state view.
func (a *App) themeStateText() string {
	active := string(a.themeMode)
	if a.themeMode == themeAuto {
		bg := "light"
		if a.hasDarkBg {
			bg = "dark"
		}
		active = "auto (" + bg + ")"
	}
	return "Theme\nactive theme: " + active + "\navailable: " + strings.Join(themeModes, ", ")
}
