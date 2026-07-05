package ui

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
)

// MCPServerInfo represents a connected MCP server.
type MCPServerInfo struct {
	Name   string
	Status string // "connected", "disconnected", "error", "configured"
	Tools  int
	Error  string
}

// MCPWizardStep represents a step in the MCP setup wizard.
type MCPWizardStep string

const (
	MCPWizardStepName    MCPWizardStep = "name"
	MCPWizardStepCommand MCPWizardStep = "command"
	MCPWizardStepArgs    MCPWizardStep = "args"
	MCPWizardStepReview  MCPWizardStep = "review"
)

// MCPManagerView displays MCP server status and allows management with a setup wizard.
type MCPManagerView struct {
	visible    bool
	servers    []MCPServerInfo
	wizard     bool
	wizardStep MCPWizardStep
	newName    string
	newCommand string
	newArgs    string
	newEnv     map[string]string
}

// NewMCPManagerView creates a new MCP manager view.
func NewMCPManagerView() *MCPManagerView {
	return &MCPManagerView{}
}

// Toggle shows/hides the MCP manager view.
func (m *MCPManagerView) Toggle() {
	if !m.visible {
		m.wizard = false
	}
	m.visible = !m.visible
}

// SetVisible sets the visibility of the MCP manager view.
func (m *MCPManagerView) SetVisible(visible bool) {
	m.visible = visible
	if !visible {
		m.wizard = false
	}
}

// IsVisible returns whether the MCP manager view is visible.
func (m *MCPManagerView) IsVisible() bool {
	return m.visible
}

// UpdateServers updates the list of MCP servers.
func (m *MCPManagerView) UpdateServers(servers []MCPServerInfo) {
	m.servers = servers
}

// LoadFromConfig loads MCP server info from config.
func (m *MCPManagerView) LoadFromConfig(cfg *config.Config) {
	m.servers = nil
	for _, mc := range cfg.MCP {
		m.servers = append(m.servers, MCPServerInfo{
			Name:   mc.Name,
			Status: "configured",
			Tools:  0,
		})
	}
}

// StartWizard begins the MCP setup wizard flow.
func (m *MCPManagerView) StartWizard() {
	m.wizard = true
	m.wizardStep = MCPWizardStepName
	m.newName = ""
	m.newCommand = ""
	m.newArgs = ""
	m.newEnv = make(map[string]string)
	m.visible = true
}

// SetWizardField sets a wizard field and advances to the next step.
func (m *MCPManagerView) SetWizardField(value string) bool {
	switch m.wizardStep {
	case MCPWizardStepName:
		if strings.TrimSpace(value) == "" {
			return false
		}
		m.newName = strings.TrimSpace(value)
		m.wizardStep = MCPWizardStepCommand
		return true
	case MCPWizardStepCommand:
		if strings.TrimSpace(value) == "" {
			return false
		}
		m.newCommand = strings.TrimSpace(value)
		m.wizardStep = MCPWizardStepArgs
		return true
	case MCPWizardStepArgs:
		m.newArgs = strings.TrimSpace(value)
		m.wizardStep = MCPWizardStepReview
		return true
	case MCPWizardStepReview:
		return false
	}
	return false
}

// CompleteWizard finishes the wizard and returns the MCP config.
func (m *MCPManagerView) CompleteWizard() *config.MCPConfig {
	if !m.wizard || m.newName == "" || m.newCommand == "" {
		return nil
	}
	m.wizard = false
	return &config.MCPConfig{
		Name:    m.newName,
		Command: m.newCommand,
		Args:    parseArgs(m.newArgs),
		Env:     m.newEnv,
	}
}

// CancelWizard cancels the MCP setup wizard.
func (m *MCPManagerView) CancelWizard() {
	m.wizard = false
}

// IsWizardActive returns whether the MCP setup wizard is active.
func (m *MCPManagerView) IsWizardActive() bool {
	return m.wizard
}

// CurrentWizardStep returns the current wizard step.
func (m *MCPManagerView) CurrentWizardStep() MCPWizardStep {
	return m.wizardStep
}

// Render produces the MCP manager view.
func (m *MCPManagerView) Render(width int, styles appStyles) string {
	if !m.visible {
		return ""
	}

	var lines []string

	if m.wizard {
		lines = append(lines, styles.pill.Render("MCP Server Setup"))
		lines = append(lines, "")

		switch m.wizardStep {
		case MCPWizardStepName:
			lines = append(lines, "Step 1: Server Name")
			lines = append(lines, "")
			lines = append(lines, "Enter a name for the MCP server:")
			lines = append(lines, styles.muted.Render("(e.g., \"filesystem\", \"github\", \"postgres\")"))
			if m.newName != "" {
				lines = append(lines, "")
				lines = append(lines, "Current: "+m.newName)
			}

		case MCPWizardStepCommand:
			lines = append(lines, fmt.Sprintf("Step 2: Command for \"%s\"", m.newName))
			lines = append(lines, "")
			lines = append(lines, "Enter the command to start the MCP server:")
			lines = append(lines, styles.muted.Render("(e.g., \"npx\", \"uvx\", or a path to a binary)"))
			if m.newCommand != "" {
				lines = append(lines, "")
				lines = append(lines, "Current: "+m.newCommand)
			}

		case MCPWizardStepArgs:
			lines = append(lines, fmt.Sprintf("Step 3: Arguments for \"%s\"", m.newName))
			lines = append(lines, "")
			lines = append(lines, "Enter arguments (space-separated), or leave empty:")
			lines = append(lines, styles.muted.Render("(e.g., \"-y @modelcontextprotocol/server-filesystem /path\")"))
			if m.newArgs != "" {
				lines = append(lines, "")
				lines = append(lines, "Current: "+m.newArgs)
			}

		case MCPWizardStepReview:
			lines = append(lines, styles.success.Render("✓ MCP Server Configuration"))
			lines = append(lines, "")
			lines = append(lines, "Review the configuration:")
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("  Name:    %s", m.newName))
			lines = append(lines, fmt.Sprintf("  Command: %s", m.newCommand))
			if m.newArgs != "" {
				lines = append(lines, fmt.Sprintf("  Args:    %s", m.newArgs))
			}
			lines = append(lines, "")
			lines = append(lines, styles.muted.Render("Press Enter to confirm, Esc to cancel"))
		}

		return strings.Join(lines, "\n")
	}

	// Non-wizard: display server list
	lines = append(lines, styles.pill.Render("MCP Servers"))
	lines = append(lines, "")

	if len(m.servers) == 0 {
		lines = append(lines, styles.muted.Render("No MCP servers configured."))
		lines = append(lines, "")
		lines = append(lines, styles.muted.Render("Type /mcp-add to start the setup wizard."))
		lines = append(lines, "")
		lines = append(lines, "Add servers in your config file:")
		lines = append(lines, "mcp:")
		lines = append(lines, "  - name: my-server")
		lines = append(lines, "    command: npx")
		lines = append(lines, "    args: [\"-y\", \"@modelcontextprotocol/server-filesystem\"]")
	} else {
		for _, server := range m.servers {
			icon := serverStatusIcon(server.Status)
			lines = append(lines, fmt.Sprintf("%s %s", icon, server.Name))
			if server.Tools > 0 {
				lines = append(lines, fmt.Sprintf("   %d tools available", server.Tools))
			}
			if server.Error != "" {
				lines = append(lines, fmt.Sprintf("   %s%s", styles.muted.Render("Error: "), server.Error))
			}
		}
		lines = append(lines, "")
		lines = append(lines, styles.muted.Render("Type /mcp-add to add another server"))
	}

	return strings.Join(lines, "\n")
}

func serverStatusIcon(status string) string {
	switch status {
	case "connected":
		return "●"
	case "disconnected":
		return "○"
	case "error":
		return "✗"
	default:
		return "◆"
	}
}

// parseArgs splits a space-separated argument string into a slice.
func parseArgs(args string) []string {
	if strings.TrimSpace(args) == "" {
		return nil
	}
	// Simple parse: split by spaces, respecting double quotes
	var result []string
	current := strings.Builder{}
	inQuotes := false
	for _, r := range args {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
