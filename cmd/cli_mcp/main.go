package main

import (
	"fmt"
	"os"

	"cli_mate/internal/mcpserver"
)

func main() {
	server := mcpserver.NewServer()

	// Register built-in project context tools
	mcpserver.RegisterBuiltinTools(server)

	// In MCP over stdio, we don't usually write logs to stdout, because stdout is used for JSON-RPC.
	// If logging is needed, it should go to stderr.
	fmt.Fprintf(os.Stderr, "cli_mcp server started, listening on stdin...\n")

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "cli_mcp server error: %v\n", err)
		os.Exit(1)
	}
}
