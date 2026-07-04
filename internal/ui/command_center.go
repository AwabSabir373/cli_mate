package ui

import (
	"fmt"
	"sort"
	"strings"

	"cli_mate/internal/usercommands"
)

// commandCategory groups commands by function.
type commandCategory struct {
	name     string
	commands []suggestion
}

// commandCenter provides a full command palette UI.
type commandCenter struct {
	visible     bool
	query       string
	cursor      int
	categories  []commandCategory
	filtered    []suggestion
	recentCount int // how many recent commands to show
}

// newCommandCenter creates a new command center.
func newCommandCenter() *commandCenter {
	return &commandCenter{
		recentCount: 5,
	}
}

// show opens the command center.
func (cc *commandCenter) show() {
	cc.visible = true
	cc.query = ""
	cc.cursor = 0
}

// hide closes the command center.
func (cc *commandCenter) hide() {
	cc.visible = false
	cc.query = ""
	cc.cursor = 0
}

// isVisible returns true if the command center is visible.
func (cc *commandCenter) isVisible() bool {
	return cc.visible
}

// buildCategories builds the command categories from available commands.
func (cc *commandCenter) buildCategories(userCmds []usercommands.Command) {
	cc.categories = []commandCategory{
		{
			name: "Setup & Configuration",
			commands: []suggestion{
				{Value: "/setup", Label: "/setup", Description: "Run the interactive setup wizard"},
				{Value: "/provider ", Label: "/provider", Description: "Choose AI provider"},
				{Value: "/model ", Label: "/model", Description: "Choose model for active provider"},
				{Value: "/api-key ", Label: "/api-key", Description: "Set or update API key"},
				{Value: "/base-url ", Label: "/base-url", Description: "Set custom API endpoint URL"},
				{Value: "/connect", Label: "/connect", Description: "Validate and connect to provider"},
			},
		},
		{
			name: "Session Management",
			commands: []suggestion{
				{Value: "/resume", Label: "/resume", Description: "Resume a previous session"},
				{Value: "/compact", Label: "/compact", Description: "Summarize older messages to save context"},
				{Value: "/clear", Label: "/clear", Description: "Clear the console"},
				{Value: "/status", Label: "/status", Description: "Show active provider configuration"},
			},
		},
		{
			name: "MCP & Extensions",
			commands: []suggestion{
				{Value: "/mcp", Label: "/mcp", Description: "Manage MCP servers"},
				{Value: "/skills", Label: "/skills", Description: "List available skills"},
			},
		},
		{
			name: "Code & Review",
			commands: []suggestion{
				{Value: "/review", Label: "/review", Description: "Review code changes"},
				{Value: "/diff ", Label: "/diff", Description: "Show git diff"},
				{Value: "/commit ", Label: "/commit", Description: "Create a git commit"},
				{Value: "/open ", Label: "/open", Description: "Preview a file in the terminal"},
				{Value: "/copy", Label: "/copy", Description: "Copy last AI response to clipboard"},
				{Value: "/undo", Label: "/undo", Description: "Undo the last file edit"},
			},
		},
		{
			name: "Appearance",
			commands: []suggestion{
				{Value: "/theme ", Label: "/theme", Description: "Choose terminal theme"},
				{Value: "/style ", Label: "/style", Description: "Set response style (concise/explanatory/review)"},
			},
		},
		{
			name: "Advanced",
			commands: []suggestion{
				{Value: "/max-tokens ", Label: "/max-tokens", Description: "Set custom context level limit"},
				{Value: "/approve", Label: "/approve", Description: "Toggle auto-approve for tool execution"},
				{Value: "/update", Label: "/update", Description: "Check for new version"},
				{Value: "/help", Label: "/help", Description: "Show all available commands"},
			},
		},
	}

	// Add user commands to a custom category
	if len(userCmds) > 0 {
		var customCmds []suggestion
		for _, cmd := range userCmds {
			desc := cmd.Description
			if desc == "" {
				desc = "Custom command"
			}
			customCmds = append(customCmds, suggestion{
				Value:       "/" + cmd.Name + " ",
				Label:       "/" + cmd.Name,
				Description: desc,
			})
		}
		cc.categories = append(cc.categories, commandCategory{
			name:     "Custom Commands",
			commands: customCmds,
		})
	}
}

// filter filters commands by query.
func (cc *commandCenter) filter(query string) {
	cc.query = query
	cc.cursor = 0

	if query == "" {
		cc.filtered = nil
		return
	}

	query = strings.ToLower(query)
	var matches []suggestion

	for _, cat := range cc.categories {
		for _, cmd := range cat.commands {
			label := strings.ToLower(strings.TrimPrefix(cmd.Label, "/"))
			desc := strings.ToLower(cmd.Description)
			if strings.Contains(label, query) || strings.Contains(desc, query) {
				matches = append(matches, cmd)
			}
		}
	}

	// Sort by relevance: prefix match first, then contains
	sort.Slice(matches, func(i, j int) bool {
		iLabel := strings.TrimPrefix(matches[i].Label, "/")
		jLabel := strings.TrimPrefix(matches[j].Label, "/")
		iPrefix := strings.HasPrefix(iLabel, query)
		jPrefix := strings.HasPrefix(jLabel, query)
		if iPrefix != jPrefix {
			return iPrefix
		}
		return len(iLabel) < len(jLabel)
	})

	cc.filtered = matches
}

// selectedCommand returns the currently selected command suggestion.
func (cc *commandCenter) selectedCommand() *suggestion {
	items := cc.displayedItems()
	if len(items) == 0 {
		return nil
	}
	if cc.cursor < 0 {
		cc.cursor = 0
	}
	if cc.cursor >= len(items) {
		cc.cursor = len(items) - 1
	}
	return &items[cc.cursor]
}

// displayedItems returns the items to display.
func (cc *commandCenter) displayedItems() []suggestion {
	if cc.filtered != nil {
		return cc.filtered
	}
	// Show all commands grouped by category
	var all []suggestion
	for _, cat := range cc.categories {
		all = append(all, cat.commands...)
	}
	return all
}

// handleKey processes a key press and returns the command to execute or empty string.
func (cc *commandCenter) handleKey(key string) string {
	if !cc.visible {
		return ""
	}

	items := cc.displayedItems()

	switch key {
	case "up", "shift+tab":
		if cc.cursor > 0 {
			cc.cursor--
		}
		return ""
	case "down", "tab":
		if cc.cursor < len(items)-1 {
			cc.cursor++
		}
		return ""
	case "enter", " ":
		if len(items) > 0 && cc.cursor >= 0 && cc.cursor < len(items) {
			cmd := items[cc.cursor].Value
			cc.hide()
			return cmd
		}
		return ""
	case "esc":
		cc.hide()
		return ""
	case "backspace":
		if len(cc.query) > 0 {
			cc.query = cc.query[:len(cc.query)-1]
			cc.filter(cc.query)
		}
		return ""
	default:
		if len(key) == 1 {
			cc.query += key
			cc.filter(cc.query)
		}
		return ""
	}
}

// render renders the command center UI.
func (cc *commandCenter) render(styles appStyles, width int) string {
	if !cc.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" Command Palette "))
	b.WriteString("\n\n")

	// Search input
	b.WriteString(styles.prompt.Render("  Search: "))
	if cc.query == "" {
		b.WriteString(styles.cursor.Render("█"))
		b.WriteString(styles.muted.Render(" Type to filter commands..."))
	} else {
		b.WriteString(styles.input.Render(cc.query))
		b.WriteString(styles.cursor.Render("█"))
	}
	b.WriteString("\n\n")

	if cc.query != "" && len(cc.filtered) == 0 {
		b.WriteString(styles.muted.Render("  No matching commands."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Type a different search or press Esc to close."))
		b.WriteString("\n")
		return b.String()
	}

	// Show filtered results or categories
	if cc.filtered != nil {
		for i, cmd := range cc.filtered {
			if i == cc.cursor {
				b.WriteString(styles.selected.Render(fmt.Sprintf("  %s", cmd.Label)))
				b.WriteString("\n")
				b.WriteString(styles.muted.Render(fmt.Sprintf("     %s", cmd.Description)))
				b.WriteString("\n")
			} else {
				b.WriteString(fmt.Sprintf("  %s", cmd.Label))
				b.WriteString("\n")
				b.WriteString(styles.muted.Render(fmt.Sprintf("     %s", cmd.Description)))
				b.WriteString("\n")
			}
		}
	} else {
		// Show categories
		for _, cat := range cc.categories {
			b.WriteString(styles.sidebarTitle.Render(fmt.Sprintf("  %s:", cat.name)))
			b.WriteString("\n")
			for _, cmd := range cat.commands {
				b.WriteString(fmt.Sprintf("    %s  ", cmd.Label))
				b.WriteString(styles.muted.Render(cmd.Description))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc close · Type to search"))
	b.WriteString("\n")
	return b.String()
}
