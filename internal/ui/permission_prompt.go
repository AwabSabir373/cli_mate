package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"cli_mate/internal/tools"
)

// permissionOption represents a selectable choice in the permission prompt.
type permissionOption struct {
	label  string
	hotkey string
	choice string
}

// permissionPrompt manages the interactive approval UI.
type permissionPrompt struct {
	call    tools.Call
	options []permissionOption
	cursor  int
	active  bool
}

// newPermissionPrompt creates a new permission prompt for the given tool call.
func newPermissionPrompt(call tools.Call) *permissionPrompt {
	p := &permissionPrompt{
		call:   call,
		active: true,
	}

	// Build options based on the tool call
	p.options = append(p.options, permissionOption{
		label:  "Allow once",
		hotkey: "a",
		choice: "allow",
	})
	p.options = append(p.options, permissionOption{
		label:  "Deny",
		hotkey: "d",
		choice: "deny",
	})
	p.options = append(p.options, permissionOption{
		label:  "Always allow tool",
		hotkey: "t",
		choice: "always_allow_tool",
	})

	// Add directory option if there's a path
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) != "" {
		p.options = append(p.options, permissionOption{
			label:  "Always allow directory",
			hotkey: "p",
			choice: "always_allow_dir",
		})
	}

	return p
}

// handleKey processes a keypress and returns the result and whether it's resolved.
func (p *permissionPrompt) handleKey(key string) (string, bool) {
	if !p.active {
		return "", true
	}

	switch key {
	case "up", "shift+tab":
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)
		return "", false
	case "down", "tab":
		p.cursor = (p.cursor + 1) % len(p.options)
		return "", false
	case "enter", " ":
		if p.cursor >= 0 && p.cursor < len(p.options) {
			p.active = false
			return p.options[p.cursor].choice, true
		}
	case "a", "A":
		p.active = false
		return "allow", true
	case "d", "D":
		p.active = false
		return "deny", true
	case "t", "T":
		p.active = false
		return "always_allow_tool", true
	default:
		// Check hotkeys
		for _, opt := range p.options {
			if strings.EqualFold(key, opt.hotkey) {
				p.active = false
				return opt.choice, true
			}
		}
	}

	return "", false
}

// render renders the permission prompt UI.
func (p *permissionPrompt) render(styles appStyles, _ int) string {
	if !p.active {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(styles.pill.Render("⚠ Permission Required"))
	b.WriteString("\n\n")

	// Tool info
	toolLabel := p.call.Name
	path, _ := p.call.Argument["path"].(string)
	if path != "" {
		toolLabel += " " + path
	}
	b.WriteString(styles.accent.Render(toolLabel))
	b.WriteString("\n\n")

	// Options
	for i, opt := range p.options {
		prefix := "  "
		if i == p.cursor {
			prefix = "▸ "
		}
		if i == p.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf("%s%s [%s]", prefix, opt.label, opt.hotkey)))
		} else {
			b.WriteString(fmt.Sprintf("  %s [%s]", opt.label, opt.hotkey))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc cancel"))

	return b.String()
}

// permissionPromptOld generates the old-style text prompt for backward compatibility.
func permissionPromptOld(call tools.Call) string {
	label := call.Name
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) != "" {
		label += " " + path
	}

	prompt := lipgloss.NewStyle().Bold(true).Render("Allow tool ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(label) +
		"\n  [a] allow once  [d] deny  [t] always allow tool"

	if strings.TrimSpace(path) != "" {
		prompt += "  [p] always allow directory"
	}

	return prompt
}
