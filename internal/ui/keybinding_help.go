package ui

import (
	"strings"
)

// KeyBinding represents a keyboard shortcut.
type KeyBinding struct {
	Keys        string
	Action      string
	Category    string
}

// KeybindingHelp displays keyboard shortcuts.
type KeybindingHelp struct {
	visible bool
}

// NewKeybindingHelp creates a new keybinding help.
func NewKeybindingHelp() *KeybindingHelp {
	return &KeybindingHelp{}
}

// Toggle shows/hides the help panel.
func (kh *KeybindingHelp) Toggle() {
	kh.visible = !kh.visible
}

// SetVisible sets the visibility of the help panel.
func (kh *KeybindingHelp) SetVisible(visible bool) {
	kh.visible = visible
}

// IsVisible returns whether the help panel is visible.
func (kh *KeybindingHelp) IsVisible() bool {
	return kh.visible
}

// GetKeyBindings returns all available keybindings.
func (kh *KeybindingHelp) GetKeyBindings() []KeyBinding {
	return []KeyBinding{
		// Navigation
		{Keys: "↑/↓", Action: "Cycle prompt history", Category: "Navigation"},
		{Keys: "Alt+↑/↓", Action: "Scroll console", Category: "Navigation"},
		{Keys: "Ctrl+P", Action: "Fuzzy file finder", Category: "Navigation"},
		{Keys: "Tab", Action: "Accept suggestion", Category: "Navigation"},
		{Keys: "Enter", Action: "Submit input", Category: "Navigation"},

		// Editing
		{Keys: "Ctrl+C", Action: "Quit", Category: "Editing"},
		{Keys: "Esc", Action: "Cancel/Back", Category: "Editing"},
		{Keys: "Ctrl+V", Action: "Paste from clipboard", Category: "Editing"},
		{Keys: "Ctrl+W", Action: "Delete word backward", Category: "Editing"},
		{Keys: "Alt+Backspace", Action: "Delete word backward", Category: "Editing"},
		{Keys: "Ctrl+U", Action: "Delete to line start", Category: "Editing"},
		{Keys: "Alt+Delete", Action: "Delete to line start", Category: "Editing"},
		{Keys: "Ctrl+K", Action: "Delete to line end", Category: "Editing"},
		{Keys: "Home", Action: "Cursor to start", Category: "Editing"},
		{Keys: "Ctrl+A", Action: "Cursor to start", Category: "Editing"},
		{Keys: "End", Action: "Cursor to end", Category: "Editing"},
		{Keys: "Ctrl+E", Action: "Cursor to end", Category: "Editing"},

		// Commands
		{Keys: "/", Action: "Open command suggestions", Category: "Commands"},
		{Keys: "@", Action: "Open file mentions", Category: "Commands"},
		{Keys: "Shift+Tab", Action: "Cycle permission mode", Category: "Commands"},
		{Keys: "Ctrl+B", Action: "Toggle sidebar", Category: "Commands"},

		// Slash Commands
		{Keys: "/help", Action: "Show all commands", Category: "Slash Commands"},
		{Keys: "/open <path>", Action: "Preview a file", Category: "Slash Commands"},
		{Keys: "/provider", Action: "Choose provider", Category: "Slash Commands"},
		{Keys: "/model", Action: "Choose model", Category: "Slash Commands"},
		{Keys: "/theme", Action: "Choose theme", Category: "Slash Commands"},
		{Keys: "/api-key", Action: "Set API key", Category: "Slash Commands"},
		{Keys: "/status", Action: "Show configuration", Category: "Slash Commands"},
		{Keys: "/clear", Action: "Clear console", Category: "Slash Commands"},
		{Keys: "/copy", Action: "Copy last response", Category: "Slash Commands"},
		{Keys: "/undo", Action: "Undo last edit", Category: "Slash Commands"},
		{Keys: "/review", Action: "Review code changes", Category: "Slash Commands"},
		{Keys: "/diff", Action: "Show git diff", Category: "Slash Commands"},
		{Keys: "/commit", Action: "Create git commit", Category: "Slash Commands"},
		{Keys: "/compact", Action: "Summarize messages", Category: "Slash Commands"},
		{Keys: "/skills", Action: "List skills", Category: "Slash Commands"},
		{Keys: "/update", Action: "Check for updates", Category: "Slash Commands"},
		{Keys: "/style", Action: "Set response style", Category: "Slash Commands"},
	}
}

// Render produces the keybinding help view.
func (kh *KeybindingHelp) Render(width int, styles appStyles) string {
	if !kh.visible {
		return ""
	}

	var lines []string
	lines = append(lines, styles.pill.Render("Keyboard Shortcuts"))
	lines = append(lines, "")

	bindings := kh.GetKeyBindings()
	currentCategory := ""

	for _, binding := range bindings {
		if binding.Category != currentCategory {
			currentCategory = binding.Category
			lines = append(lines, "")
			lines = append(lines, styles.accent.Render(currentCategory))
		}
		lines = append(lines, styles.muted.Render("  "+binding.Keys)+" → "+binding.Action)
	}

	lines = append(lines, "")
	lines = append(lines, styles.muted.Render("Press Esc to close"))

	return strings.Join(lines, "\n")
}
