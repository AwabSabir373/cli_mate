package ui

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
)

// mcpServerEntry represents an MCP server being edited in the wizard.
type mcpServerEntry struct {
	name    string
	command string
	args    string
	envVars map[string]string
}

// mcpWizardStage represents the current stage in the MCP setup wizard.
type mcpWizardStage int

const (
	mcpStageList     mcpWizardStage = iota
	mcpStageAddName
	mcpStageAddCommand
	mcpStageAddArgs
	mcpStageAddEnv
	mcpStageConfirm
)

// mcpManager manages MCP server configuration via UI.
type mcpManager struct {
	visible    bool
	stage      mcpWizardStage
	servers    []config.MCPConfig
	cursor     int
	scrollOff  int
	editEntry  mcpServerEntry
	editEnvKey string
	err        string
}

// newMCPManager creates a new MCP manager.
func newMCPManager() *mcpManager {
	return &mcpManager{
		servers:   []config.MCPConfig{},
		editEntry: mcpServerEntry{envVars: make(map[string]string)},
	}
}

// loadFromConfig loads MCP servers from the config.
func (mm *mcpManager) loadFromConfig(cfg *config.Config) {
	if cfg != nil {
		mm.servers = cfg.MCP
	} else {
		mm.servers = nil
	}
}

// show opens the MCP manager.
func (mm *mcpManager) show() {
	mm.visible = true
	mm.stage = mcpStageList
	mm.cursor = 0
	mm.scrollOff = 0
	mm.err = ""
}

// hide closes the MCP manager.
func (mm *mcpManager) hide() {
	mm.visible = false
	mm.stage = mcpStageList
	mm.editEntry = mcpServerEntry{envVars: make(map[string]string)}
	mm.err = ""
}

// isVisible returns true if the manager is visible.
func (mm *mcpManager) isVisible() bool {
	return mm.visible
}

// handleKey processes a keypress and returns (shouldSave bool, action string).
func (mm *mcpManager) handleKey(key string) (bool, string) {
	if !mm.visible {
		return false, ""
	}

	switch mm.stage {
	case mcpStageList:
		return mm.handleListKey(key)
	case mcpStageAddName:
		return mm.handleAddNameKey(key)
	case mcpStageAddCommand:
		return mm.handleAddCommandKey(key)
	case mcpStageAddArgs:
		return mm.handleAddArgsKey(key)
	case mcpStageAddEnv:
		return mm.handleAddEnvKey(key)
	case mcpStageConfirm:
		return mm.handleConfirmKey(key)
	}
	return false, ""
}

func (mm *mcpManager) handleListKey(key string) (bool, string) {
	switch key {
	case "up", "shift+tab":
		if mm.cursor > 0 {
			mm.cursor--
		}
		mm.adjustScroll()
	case "down", "tab":
		if mm.cursor < len(mm.servers) {
			mm.cursor++
		}
		mm.adjustScroll()
	case "enter", " ":
		if mm.cursor >= 0 && mm.cursor < len(mm.servers) {
			// Select server (edit/delete)
			mm.editEntry = mcpServerEntry{
				name:    mm.servers[mm.cursor].Name,
				command: mm.servers[mm.cursor].Command,
				args:    strings.Join(mm.servers[mm.cursor].Args, " "),
				envVars: mm.servers[mm.cursor].Env,
			}
			if mm.editEntry.envVars == nil {
				mm.editEntry.envVars = make(map[string]string)
			}
			mm.stage = mcpStageConfirm
			mm.cursor = 0
		} else if mm.cursor >= len(mm.servers) {
			// Add new server
			mm.editEntry = mcpServerEntry{envVars: make(map[string]string)}
			mm.stage = mcpStageAddName
			mm.cursor = 0
		}
	case "esc":
		mm.hide()
		return false, "close"
	case "delete", "backspace":
		if mm.cursor >= 0 && mm.cursor < len(mm.servers) {
			mm.servers = append(mm.servers[:mm.cursor], mm.servers[mm.cursor+1:]...)
			if mm.cursor >= len(mm.servers) {
				mm.cursor = len(mm.servers) - 1
			}
		}
	}
	return false, ""
}

func (mm *mcpManager) handleAddNameKey(key string) (bool, string) {
	switch key {
	case "esc":
		mm.stage = mcpStageList
		mm.cursor = 0
	default:
		if key == "enter" {
			if mm.editEntry.name == "" {
				mm.err = "Server name is required."
				return false, ""
			}
			mm.stage = mcpStageAddCommand
			mm.err = ""
		}
	}
	return false, ""
}

func (mm *mcpManager) handleAddCommandKey(key string) (bool, string) {
	switch key {
	case "esc":
		mm.stage = mcpStageAddName
	default:
		if key == "enter" {
			if mm.editEntry.command == "" {
				mm.err = "Command is required (e.g., npx, python, node)."
				return false, ""
			}
			mm.stage = mcpStageAddArgs
			mm.err = ""
		}
	}
	return false, ""
}

func (mm *mcpManager) handleAddArgsKey(key string) (bool, string) {
	switch key {
	case "esc":
		mm.stage = mcpStageAddCommand
	default:
		if key == "enter" {
			mm.stage = mcpStageAddEnv
			mm.cursor = 0
		}
	}
	return false, ""
}

func (mm *mcpManager) handleAddEnvKey(key string) (bool, string) {
	switch key {
	case "up", "shift+tab":
		if mm.cursor > 0 {
			mm.cursor--
		}
	case "down", "tab":
		// Allow navigating to "Done" option
		envCount := len(mm.editEntry.envVars)
		if mm.cursor < envCount+1 {
			mm.cursor++
		}
	case "enter", " ":
		envCount := len(mm.editEntry.envVars)
		if mm.cursor >= envCount && mm.cursor <= envCount {
			// Done adding env vars
			mm.stage = mcpStageConfirm
			mm.cursor = 0
		}
	case "esc":
		mm.stage = mcpStageAddArgs
	}
	return false, ""
}

func (mm *mcpManager) handleConfirmKey(key string) (bool, string) {
	switch key {
	case "up", "shift+tab":
		if mm.cursor > 0 {
			mm.cursor--
		}
	case "down", "tab":
		if mm.cursor < 2 {
			mm.cursor++
		}
	case "enter", " ":
		if mm.cursor == 0 {
			// Save server
			args := strings.Fields(mm.editEntry.args)
			mm.saveCurrentServer(args)
			mm.stage = mcpStageList
			mm.cursor = 0
			return true, "save"
		} else if mm.cursor == 1 {
			// Edit - Go back to name
			mm.stage = mcpStageAddName
			mm.cursor = 0
		} else if mm.cursor == 2 {
			// Delete
			mm.deleteCurrentServer()
			mm.stage = mcpStageList
			mm.cursor = 0
			return true, "delete"
		}
	case "esc":
		mm.stage = mcpStageList
		mm.cursor = 0
	}
	return false, ""
}

func (mm *mcpManager) saveCurrentServer(args []string) {
	cfg := config.MCPConfig{
		Name:    mm.editEntry.name,
		Command: mm.editEntry.command,
		Args:    args,
		Env:     mm.editEntry.envVars,
	}

	// Find and replace or append
	found := false
	for i, s := range mm.servers {
		if s.Name == mm.editEntry.name {
			mm.servers[i] = cfg
			found = true
			break
		}
	}
	if !found {
		mm.servers = append(mm.servers, cfg)
	}
}

func (mm *mcpManager) deleteCurrentServer() {
	for i, s := range mm.servers {
		if s.Name == mm.editEntry.name {
			mm.servers = append(mm.servers[:i], mm.servers[i+1:]...)
			break
		}
	}
}

// applyToConfig saves the MCP servers to the app's config.
func (mm *mcpManager) applyToConfig(a *App) {
	// The servers are already in mm.servers - we need to write them to cfg.MCP
	// Since config.Config.MCP is a slice, we need to replace it
	if a.cfg != nil {
		a.cfg.MCP = mm.servers
	}
	a.saveSettings()
}

func (mm *mcpManager) adjustScroll() {
	maxVisible := 10
	if mm.cursor < mm.scrollOff {
		mm.scrollOff = mm.cursor
	}
	if mm.cursor >= mm.scrollOff+maxVisible {
		mm.scrollOff = mm.cursor - maxVisible + 1
	}
}

// render renders the MCP manager UI.
func renderMCPManager(mm *mcpManager, styles appStyles, width int) string {
	if !mm.visible {
		return ""
	}

	switch mm.stage {
	case mcpStageList:
		return mm.renderList(styles, width)
	case mcpStageAddName:
		return mm.renderAddName(styles, width)
	case mcpStageAddCommand:
		return mm.renderAddCommand(styles, width)
	case mcpStageAddArgs:
		return mm.renderAddArgs(styles, width)
	case mcpStageAddEnv:
		return mm.renderAddEnv(styles, width)
	case mcpStageConfirm:
		return mm.renderConfirm(styles, width)
	}
	return ""
}

func (mm *mcpManager) renderList(styles appStyles, width int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" MCP Servers "))
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("Manage your MCP (Model Context Protocol) servers:"))
	b.WriteString("\n\n")

	if len(mm.servers) == 0 {
		b.WriteString(styles.muted.Render("  No MCP servers configured."))
		b.WriteString("\n\n")
	}

	maxVisible := 10
	start := mm.scrollOff
	end := start + maxVisible
	if end > len(mm.servers) {
		end = len(mm.servers)
	}

	for i := start; i < end; i++ {
		srv := mm.servers[i]
		label := fmt.Sprintf("%s  (%s %s)", srv.Name, srv.Command, strings.Join(srv.Args, " "))
		if len(label) > 60 {
			label = label[:60] + "..."
		}
		if i == mm.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", label)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", label))
			b.WriteString("\n")
		}
	}

	// Add new option
	addLabel := "+ Add MCP Server"
	if mm.cursor == len(mm.servers) {
		b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", addLabel)))
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("   %s", addLabel))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter edit/add · Del remove · Esc close"))
	b.WriteString("\n")
	return b.String()
}

func (mm *mcpManager) renderAddName(styles appStyles, width int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Add MCP Server: Name "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Enter a name for this MCP server:"))
	b.WriteString("\n\n")
	b.WriteString(styles.prompt.Render("  Name: "))
	b.WriteString(styles.input.Render(mm.editEntry.name))
	if mm.editEntry.name == "" {
		b.WriteString(styles.cursor.Render("█"))
	}
	b.WriteString("\n")
	if mm.err != "" {
		b.WriteString("\n")
		b.WriteString(styles.error.Render(mm.err))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Type the name · Enter confirm · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (mm *mcpManager) renderAddCommand(styles appStyles, width int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Add MCP Server: Command "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Enter the command to start the MCP server:"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  (e.g., npx, python, node, docker)"))
	b.WriteString("\n\n")
	b.WriteString(styles.prompt.Render("  Command: "))
	b.WriteString(styles.input.Render(mm.editEntry.command))
	if mm.editEntry.command == "" {
		b.WriteString(styles.cursor.Render("█"))
	}
	b.WriteString("\n")
	if mm.err != "" {
		b.WriteString("\n")
		b.WriteString(styles.error.Render(mm.err))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Type the command · Enter confirm · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (mm *mcpManager) renderAddArgs(styles appStyles, width int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Add MCP Server: Arguments "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Enter any arguments for the command (optional):"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  (e.g., -y @modelcontextprotocol/server-filesystem /path)"))
	b.WriteString("\n\n")
	b.WriteString(styles.prompt.Render("  Args: "))
	b.WriteString(styles.input.Render(mm.editEntry.args))
	if mm.editEntry.args == "" {
		b.WriteString(styles.cursor.Render("█"))
	}
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("  Type arguments separated by spaces · Enter confirm · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (mm *mcpManager) renderAddEnv(styles appStyles, width int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Add MCP Server: Environment "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Environment variables (optional). Press Enter on \"Done\" to continue:"))
	b.WriteString("\n\n")

	envKeys := make([]string, 0, len(mm.editEntry.envVars))
	for k := range mm.editEntry.envVars {
		envKeys = append(envKeys, k)
	}

	for i, k := range envKeys {
		label := fmt.Sprintf("  %s = %s", k, mm.editEntry.envVars[k])
		if len(label) > 50 {
			label = label[:50] + "..."
		}
		if i == mm.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", label)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", label))
			b.WriteString("\n")
		}
	}

	doneLabel := "✓ Done"
	if mm.cursor == len(envKeys) {
		b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", doneLabel)))
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("   %s", doneLabel))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (mm *mcpManager) renderConfirm(styles appStyles, width int) string {
	var b strings.Builder
	if mm.editEntry.name != "" && mm.findServerIndex(mm.editEntry.name) >= 0 {
		b.WriteString(styles.pill.Render(" Edit MCP Server "))
	} else {
		b.WriteString(styles.pill.Render(" Review MCP Server "))
	}
	b.WriteString("\n\n")

	b.WriteString(styles.accent.Render("  Name:"))
	b.WriteString(fmt.Sprintf("    %s", mm.editEntry.name))
	b.WriteString("\n")
	b.WriteString(styles.accent.Render("  Command:"))
	b.WriteString(fmt.Sprintf(" %s", mm.editEntry.command))
	b.WriteString("\n")
	if mm.editEntry.args != "" {
		b.WriteString(styles.accent.Render("  Args:"))
		b.WriteString(fmt.Sprintf("     %s", mm.editEntry.args))
		b.WriteString("\n")
	}
	if len(mm.editEntry.envVars) > 0 {
		b.WriteString(styles.accent.Render("  Env:"))
		b.WriteString(fmt.Sprintf("      %d variable(s)", len(mm.editEntry.envVars)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	options := []string{"✓ Save Server", "← Edit", "✕ Delete Server"}
	for i, opt := range options {
		if i == mm.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf("  %s", opt)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s", opt))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (mm *mcpManager) findServerIndex(name string) int {
	for i, s := range mm.servers {
		if s.Name == name {
			return i
		}
	}
	return -1
}
