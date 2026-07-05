package mcpserver

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// GetToolDefinitions returns the schema for the tools provided by this MCP server.
func GetToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "project_tree",
			"description": "Returns a concise tree map of the repository structure to understand the project context.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "search_code",
			"description": "Searches for a specific string across the project files.",
			"inputSchema": map[string]any{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The string or keyword to search for",
					},
				},
			},
		},
		{
			"name":        "read_file",
			"description": "Reads the contents of a specific file.",
			"inputSchema": map[string]any{
				"type":     "object",
				"required": []string{"path"},
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "The relative or absolute path to the file",
					},
				},
			},
		},
	}
}

// RegisterBuiltinTools registers the core context tools onto the server.
func RegisterBuiltinTools(s *Server) {
	s.RegisterTool("project_tree", handleProjectTree)
	s.RegisterTool("search_code", handleSearchCode)
	s.RegisterTool("read_file", handleReadFile)
}

// checkBoundary ensures the requested path is within the current working directory.
func checkBoundary(target string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get cwd: %w", err)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("could not resolve absolute path: %w", err)
	}
	if !strings.HasPrefix(abs, cwd) {
		return fmt.Errorf("access denied: path %q escapes workspace boundary", target)
	}
	return nil
}

func handleProjectTree(ctx context.Context, params map[string]any) (any, error) {
	var builder strings.Builder
	builder.WriteString("Project Tree:\n")

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		// Skip hidden directories like .git or .idea
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			return filepath.SkipDir
		}

		// Skip common binary/build directories
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "dist") {
			return filepath.SkipDir
		}

		if path != "." {
			indent := strings.Repeat("  ", strings.Count(path, string(os.PathSeparator)))
			if d.IsDir() {
				builder.WriteString(fmt.Sprintf("%s%s/\n", indent, d.Name()))
			} else {
				builder.WriteString(fmt.Sprintf("%s%s\n", indent, d.Name()))
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return builder.String(), nil
}

func handleSearchCode(ctx context.Context, params map[string]any) (any, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid 'query' parameter")
	}

	var results []string

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
			return filepath.SkipDir
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "dist") {
			return filepath.SkipDir
		}

		if !d.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 1
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				line := scanner.Text()
				if strings.Contains(line, query) {
					results = append(results, fmt.Sprintf("%s:%d: %s", path, lineNum, strings.TrimSpace(line)))
				}
				lineNum++
			}
		}
		return nil
	})

	if err != nil {
		if err == context.Canceled {
			return nil, fmt.Errorf("search cancelled")
		}
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No results found for query.", nil
	}

	// Limit results to avoid massive token usage
	if len(results) > 100 {
		results = append(results[:100], "... (results truncated, refine your search)")
	}

	return strings.Join(results, "\n"), nil
}

func handleReadFile(ctx context.Context, params map[string]any) (any, error) {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("missing or invalid 'path' parameter")
	}

	// Sandboxing: Check if the path escapes the workspace root
	if err := checkBoundary(path); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", path, err)
	}

	// Add a length limit for safety (e.g., 50KB) to avoid token blowout
	if len(content) > 50000 {
		return string(content[:50000]) + "\n... (file truncated due to size limit to save tokens)", nil
	}

	return string(content), nil
}
