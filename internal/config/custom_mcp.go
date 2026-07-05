package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// CustomMCPFile represents the schema for the custom mcp.yml configuration.
type CustomMCPFile struct {
	Name       string            `yaml:"name"`
	Version    string            `yaml:"version"`
	Schema     string            `yaml:"schema"`
	MCPServers []CustomMCPServer `yaml:"mcpServers"`
}

// CustomMCPServer represents a single MCP server definition inside mcp.yml.
type CustomMCPServer struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// BuiltinMCPConfig returns cli_mate's own MCP server configuration.
func BuiltinMCPConfig() (MCPConfig, error) {
	exe, err := os.Executable()
	if err != nil {
		return MCPConfig{}, err
	}
	return MCPConfig{
		Name:    "cli_mcp",
		Command: exe,
		Args:    []string{"mcp-server"},
		Env:     map[string]string{},
	}, nil
}

// DefaultCustomMCPConfig returns the project-local MCP file cli_mate creates by default.
func DefaultCustomMCPConfig() (*CustomMCPFile, error) {
	builtin, err := BuiltinMCPConfig()
	if err != nil {
		return nil, err
	}
	return &CustomMCPFile{
		Name:    "CLI Mate Built-in MCP",
		Version: "0.0.1",
		Schema:  "v1",
		MCPServers: []CustomMCPServer{
			{
				Name:    builtin.Name,
				Command: builtin.Command,
				Args:    builtin.Args,
				Env:     builtin.Env,
			},
		},
	}, nil
}

// IsLegacyGeneratedDefault reports whether this is the old hard-coded sample config.
func (c *CustomMCPFile) IsLegacyGeneratedDefault() bool {
	if c == nil || c.Name != "Serena + Context7" || len(c.MCPServers) != 2 {
		return false
	}
	names := map[string]bool{}
	for _, server := range c.MCPServers {
		names[server.Name] = true
	}
	return names["serena-frontend"] && names["context7"]
}

// LoadCustomMCPConfig reads and parses the custom MCP configuration from the given path.
func LoadCustomMCPConfig(path string) (*CustomMCPFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg CustomMCPFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse mcp config: %w", err)
	}
	return &cfg, nil
}

// SaveCustomMCPConfig writes the custom MCP configuration to the given path.
func SaveCustomMCPConfig(path string, cfg *CustomMCPFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ConvertToInternal converts custom MCP server configs to the internal representation.
func (c *CustomMCPFile) ConvertToInternal() []MCPConfig {
	var out []MCPConfig
	for _, s := range c.MCPServers {
		out = append(out, MCPConfig{
			Name:    s.Name,
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		})
	}
	return out
}
