package config

import "testing"

func TestDefaultCustomMCPConfigUsesBuiltinServer(t *testing.T) {
	cfg, err := DefaultCustomMCPConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "CLI Mate Built-in MCP" {
		t.Fatalf("expected built-in MCP config name, got %q", cfg.Name)
	}
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("expected one MCP server, got %d", len(cfg.MCPServers))
	}
	server := cfg.MCPServers[0]
	if server.Name != "cli_mcp" {
		t.Fatalf("expected cli_mcp server, got %q", server.Name)
	}
	if len(server.Args) != 1 || server.Args[0] != "mcp-server" {
		t.Fatalf("expected mcp-server args, got %#v", server.Args)
	}
}

func TestCustomMCPFileDetectsLegacyGeneratedDefault(t *testing.T) {
	cfg := &CustomMCPFile{
		Name: "Serena + Context7",
		MCPServers: []CustomMCPServer{
			{Name: "serena-frontend"},
			{Name: "context7"},
		},
	}
	if !cfg.IsLegacyGeneratedDefault() {
		t.Fatal("expected legacy generated default to be detected")
	}

	cfg.MCPServers = append(cfg.MCPServers, CustomMCPServer{Name: "custom"})
	if cfg.IsLegacyGeneratedDefault() {
		t.Fatal("expected modified config to be treated as user-managed")
	}
}
