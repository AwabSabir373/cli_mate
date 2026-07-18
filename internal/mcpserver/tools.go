package mcpserver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	defaultSearchResults = 3
	defaultSearchChars   = 480 // roughly 100-140 model tokens for typical code
	defaultReadChars     = 2000
	defaultTreeEntries   = 30
)

func GetToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "project_tree",
			"description": "Compact repository map. Defaults to 2 levels and 30 entries; increase only when needed.",
			"inputSchema": objectSchema(map[string]any{
				"path":        stringProperty("workspace-relative directory; default ."),
				"max_depth":   integerProperty("depth from path; default 2, max 6", 1, 6),
				"max_entries": integerProperty("entry budget; default 30, max 200", 1, 200),
			}),
		},
		{
			"name":        "search_code",
			"description": "Token-efficient literal code search returning exact file:line anchors under a hard character budget.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"query":          stringProperty("literal text to find"),
				"path":           stringProperty("workspace-relative directory or file; default ."),
				"case_sensitive": map[string]any{"type": "boolean", "description": "default false"},
				"max_results":    integerProperty("default 3, max 20", 1, 20),
				"max_chars":      integerProperty("response budget; default 480 (~100 tokens), max 4000", 200, 4000),
			}, "query"),
		},
		{
			"name":        "read_file",
			"description": "Read an exact line window. Use search_code first, then request only nearby lines.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"path":       stringProperty("workspace-relative file"),
				"line_start": integerProperty("first line, 1-based; default 1", 1, 10000000),
				"line_end":   integerProperty("last line, inclusive; default line_start+39", 1, 10000000),
				"max_chars":  integerProperty("response budget; default 2000, max 12000", 200, 12000),
			}, "path"),
		},
		{
			"name":        "go_symbols",
			"description": "AST-backed Go symbol outline with signatures and exact locations; compact alternative to reading files.",
			"inputSchema": objectSchema(map[string]any{
				"path":        stringProperty("Go file or directory; default ."),
				"query":       stringProperty("optional symbol-name filter"),
				"max_results": integerProperty("default 20, max 100", 1, 100),
				"max_chars":   integerProperty("default 1600, max 8000", 200, 8000),
			}),
		},
		{
			"name":        "find_go_symbol",
			"description": "Find a Go function, method, type, variable, or constant by symbol name; returns signature and optional body.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"name":         stringProperty("symbol name; methods may use Receiver.Method"),
				"path":         stringProperty("Go file or directory; default ."),
				"include_body": map[string]any{"type": "boolean", "description": "include source body; default false"},
				"max_chars":    integerProperty("default 3000, max 12000", 200, 12000),
			}, "name"),
		},
		{
			"name":        "find_go_references",
			"description": "Find compact exact locations for all Go identifier references to a symbol.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"name":        stringProperty("identifier name"),
				"path":        stringProperty("Go file or directory; default ."),
				"max_results": integerProperty("default 12, max 100", 1, 100),
				"max_chars":   integerProperty("default 1600, max 8000", 200, 8000),
			}, "name"),
		},
		{
			"name":        "go_callers",
			"description": "Find functions and methods that call the named Go function or method.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"name":        stringProperty("called function or method name"),
				"path":        stringProperty("Go file or directory; default ."),
				"max_results": integerProperty("default 12, max 100", 1, 100),
			}, "name"),
		},
		{
			"name":        "replace_go_symbol",
			"description": "Replace one Go function or method body by AST location, then format the file. Requires an exact file and symbol.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"path": stringProperty("workspace-relative Go file"),
				"name": stringProperty("function or Receiver.Method"),
				"body": stringProperty("new body, with or without outer braces"),
			}, "path", "name", "body"),
		},
		{
			"name":        "go_diagnostics",
			"description": "Run go test ./... in a workspace directory and return compact compiler/test diagnostics.",
			"inputSchema": objectSchema(map[string]any{
				"path":            stringProperty("workspace-relative module directory; default ."),
				"timeout_seconds": integerProperty("default 30, max 120", 1, 120),
				"max_chars":       integerProperty("default 3000, max 12000", 200, 12000),
			}),
		},
		{
			"name":        "code_symbols",
			"description": "Language-neutral compact symbol outline for Go, Python, JS/TS, Rust, JVM, C/C++, C#, PHP, Ruby, Swift, Dart, Lua, and shell.",
			"inputSchema": objectSchema(map[string]any{
				"path":        stringProperty("source file or directory; default ."),
				"query":       stringProperty("optional symbol-name filter"),
				"language":    stringProperty("optional language filter"),
				"max_results": integerProperty("default 30, max 200", 1, 200),
				"max_chars":   integerProperty("default 2400, max 12000", 200, 12000),
			}),
		},
		{
			"name":        "find_code_symbol",
			"description": "Find a named symbol across supported languages and optionally return only that symbol's source body.",
			"inputSchema": objectSchemaRequired(map[string]any{
				"name":         stringProperty("exact symbol name"),
				"path":         stringProperty("source file or directory; default ."),
				"include_body": map[string]any{"type": "boolean", "description": "include symbol source; default false"},
				"max_chars":    integerProperty("default 3000, max 12000", 200, 12000),
			}, "name"),
		},
	}
}

func objectSchema(properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
}

func objectSchemaRequired(properties map[string]any, required ...string) map[string]any {
	schema := objectSchema(properties)
	schema["required"] = required
	return schema
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerProperty(description string, minimum, maximum int) map[string]any {
	return map[string]any{"type": "integer", "description": description, "minimum": minimum, "maximum": maximum}
}

func RegisterBuiltinTools(s *Server) {
	s.RegisterTool("project_tree", handleProjectTree)
	s.RegisterTool("search_code", handleSearchCode)
	s.RegisterTool("read_file", handleReadFile)
	s.RegisterTool("go_symbols", handleGoSymbols)
	s.RegisterTool("find_go_symbol", handleFindGoSymbol)
	s.RegisterTool("find_go_references", handleFindGoReferences)
	s.RegisterTool("go_callers", handleGoCallers)
	s.RegisterTool("replace_go_symbol", handleReplaceGoSymbol)
	s.RegisterTool("go_diagnostics", handleGoDiagnostics)
	s.RegisterTool("code_symbols", handleCodeSymbols)
	s.RegisterTool("find_code_symbol", handleFindCodeSymbol)
}

func resolveWorkspacePath(target string) (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get workspace: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	if strings.TrimSpace(target) == "" {
		target = "."
	}
	resolved := filepath.FromSlash(target)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}
	resolved = filepath.Clean(resolved)
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("access denied: path %q escapes workspace", target)
	}
	// Resolve existing symlinks so an in-workspace link cannot escape the root.
	if evaluated, evalErr := filepath.EvalSymlinks(resolved); evalErr == nil {
		evalRel, relErr := filepath.Rel(root, evaluated)
		if relErr != nil || evalRel == ".." || strings.HasPrefix(evalRel, ".."+string(filepath.Separator)) || filepath.IsAbs(evalRel) {
			return "", fmt.Errorf("access denied: path %q resolves outside workspace", target)
		}
		resolved = evaluated
	}
	return resolved, nil
}

func handleProjectTree(ctx context.Context, params map[string]any) (any, error) {
	root, err := resolveWorkspacePath(stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	maxDepth := intParam(params, "max_depth", 2, 1, 6)
	maxEntries := intParam(params, "max_entries", defaultTreeEntries, 1, 200)
	var entries []string
	stop := errors.New("entry budget reached")
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(filepath.ToSlash(rel), "/") + 1
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			name += "/"
		}
		entries = append(entries, name)
		if len(entries) >= maxEntries {
			return stop
		}
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		return nil, fmt.Errorf("map project: %w", err)
	}
	sort.Strings(entries)
	if errors.Is(err, stop) {
		entries = append(entries, "... truncated; narrow path or raise max_entries")
	}
	return strings.Join(entries, "\n"), nil
}

func handleSearchCode(ctx context.Context, params map[string]any) (any, error) {
	query := stringParam(params, "query", "")
	if query == "" {
		return nil, fmt.Errorf("missing or invalid query")
	}
	root, err := resolveWorkspacePath(stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	caseSensitive, _ := params["case_sensitive"].(bool)
	maxResults := intParam(params, "max_results", defaultSearchResults, 1, 20)
	maxChars := intParam(params, "max_chars", defaultSearchChars, 200, 4000)
	needle := query
	if !caseSensitive {
		needle = strings.ToLower(needle)
	}
	workspace, _ := os.Getwd()
	results := make([]string, 0, maxResults)
	truncated := false
	stop := errors.New("search budget reached")

	visit := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		defer func() { _ = file.Close() }()
		scanner := bufio.NewScanner(io.LimitReader(file, 2<<20))
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if !utf8.ValidString(line) || strings.IndexByte(line, 0) >= 0 {
				return nil
			}
			haystack := line
			if !caseSensitive {
				haystack = strings.ToLower(haystack)
			}
			if !strings.Contains(haystack, needle) {
				continue
			}
			rel, _ := filepath.Rel(workspace, path)
			preview := compactLine(line, 140)
			candidate := fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), lineNo, preview)
			if outputLength(results)+len(candidate)+1 > maxChars || len(results) >= maxResults {
				truncated = true
				return stop
			}
			results = append(results, candidate)
		}
		return nil
	}
	info, statErr := os.Stat(root)
	if statErr != nil {
		return nil, statErr
	}
	if info.IsDir() {
		err = filepath.WalkDir(root, visit)
	} else {
		err = visit(root, fs.FileInfoToDirEntry(info), nil)
	}
	if err != nil && !errors.Is(err, stop) {
		return nil, fmt.Errorf("search code: %w", err)
	}
	if len(results) == 0 {
		return "no matches", nil
	}
	if truncated {
		results = append(results, "... truncated; narrow query/path")
	}
	return strings.Join(results, "\n"), nil
}

func handleReadFile(ctx context.Context, params map[string]any) (any, error) {
	requested := stringParam(params, "path", "")
	if requested == "" {
		return nil, fmt.Errorf("missing or invalid path")
	}
	path, err := resolveWorkspacePath(requested)
	if err != nil {
		return nil, err
	}
	start := intParam(params, "line_start", 1, 1, 10000000)
	end := intParam(params, "line_end", start+39, start, 10000000)
	maxChars := intParam(params, "max_chars", defaultReadChars, 200, 12000)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", requested, err)
	}
	defer func() { _ = file.Close() }()

	var lines []string
	used := 0
	last := start - 1
	truncated := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if lineNo < start {
			continue
		}
		if lineNo > end {
			break
		}
		line := fmt.Sprintf("%d|%s", lineNo, scanner.Text())
		if used+len(line)+1 > maxChars {
			truncated = true
			break
		}
		lines = append(lines, line)
		used += len(line) + 1
		last = lineNo
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", requested, err)
	}
	header := fmt.Sprintf("%s:%d-%d", filepath.ToSlash(requested), start, last)
	if len(lines) == 0 {
		return header + "\n(no lines)", nil
	}
	if truncated {
		lines = append(lines, "... truncated; request a smaller line window")
	}
	return header + "\n" + strings.Join(lines, "\n"), nil
}

func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "dist", "build", "target", "coverage", "__pycache__":
		return true
	default:
		return false
	}
}

func stringParam(params map[string]any, key, fallback string) string {
	value, ok := params[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func intParam(params map[string]any, key string, fallback, minimum, maximum int) int {
	value := fallback
	switch raw := params[key].(type) {
	case float64:
		value = int(raw)
	case int:
		value = raw
	case string:
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func compactLine(line string, limit int) string {
	line = strings.Join(strings.Fields(line), " ")
	if len(line) <= limit {
		return line
	}
	return line[:limit] + "..."
}

func outputLength(lines []string) int {
	total := 0
	for _, line := range lines {
		total += len(line) + 1
	}
	return total
}
