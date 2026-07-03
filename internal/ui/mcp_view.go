package ui

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
)

// MCPServerInfo represents a connected MCP server.
type MCPServerInfo struct {
	Name    string
	Status  string // "connected", "disconnected", "error"
	Tools   int
	Error   string
}

// MCPManagerView displays MCP server status and allows management.
type MCPManagerView struct {
	visible bool
	servers []MCPServerInfo
}

// NewMCPManagerView creates a new MCP manager view.
func NewMCPManagerView() *MCPManagerView {
	return &MCPManagerView{}
}

// Toggle shows/hides the MCP manager view.
func (m *MCPManagerView) Toggle() {
	m.visible = !m.visible
}

// SetVisible sets the visibility of the MCP manager view.
func (m *MCPManagerView) SetVisible(visible bool) {
	m.visible = visible
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

// Render produces the MCP manager view.
func (m *MCPManagerView) Render(width int, styles appStyles) string {
	if !m.visible {
		return ""
	}

	var lines []string
	lines = append(lines, styles.pill.Render("MCP Servers"))
	lines = append(lines, "")

	if len(m.servers) == 0 {
		lines = append(lines, styles.muted.Render("No MCP servers configured."))
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
				lines = append(lines, styles.muted.Render("   Error: "+server.Error))
			}
		}
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
