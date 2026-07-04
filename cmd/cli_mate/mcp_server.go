package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"cli_mate/internal/mcpserver"
)

func newMCPServerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-server",
		Short: "Start the built-in MCP server for project context",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcpserver.NewServer()

			// Register built-in project context tools
			mcpserver.RegisterBuiltinTools(server)

			fmt.Fprintf(os.Stderr, "cli_mate built-in MCP server started, listening on stdin...\n")

			if err := server.Start(); err != nil {
				return fmt.Errorf("mcp server error: %w", err)
			}
			return nil
		},
	}
}
