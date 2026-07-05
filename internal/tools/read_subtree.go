package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ReadSubtreeTool struct {
	Root string
}

func NewReadSubtreeTool(root string) *ReadSubtreeTool {
	return &ReadSubtreeTool{Root: root}
}

func (t *ReadSubtreeTool) Name() string {
	return "read_subtree"
}

func (t *ReadSubtreeTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Read a directory subtree and return a structured view of subdirectories, file names, and parsed function/variable names from source files. Use this to understand the structure of a package or directory at once instead of reading individual files.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to analyze (relative to workspace root or absolute). Use '.' for workspace root.",
				},
				"max_depth": map[string]any{
					"type":        "integer",
					"description": "Maximum directory depth to recurse (default: 3, max: 10). Set to -1 for unlimited.",
				},
			},
		},
	}
}

func (t *ReadSubtreeTool) Execute(_ context.Context, call Call) (Result, error) {
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) == "" {
		return Result{Error: "path is required"}, fmt.Errorf("path is required")
	}

	maxDepth := 3
	if depth, ok := call.Argument["max_depth"].(float64); ok {
		maxDepth = int(depth)
		if maxDepth < -1 {
			maxDepth = -1
		}
		if maxDepth > 10 {
			maxDepth = 10
		}
	}

	resolved, err := resolveWorkspacePath(t.Root, path)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	if !info.IsDir() {
		// Single file: return parsed symbols
		symbols, err := parseGoFileSymbols(resolved)
		if err != nil {
			return Result{Error: err.Error()}, err
		}
		return Result{Content: fmt.Sprintf("%s\n%s", filepath.Base(resolved), symbols)}, nil
	}

	skipDirs := map[string]bool{
		".git": true, ".idea": true, "node_modules": true,
		"vendor": true, ".openclaude": true, "build": true,
		"dist": true, ".dart_tool": true, "__pycache__": true,
		".next": true, ".turbo": true, ".mimocode": true,
		".claude": true, ".codex": true,
	}

	var result strings.Builder
	walkDepth := 0
	fileCount := 0
	const maxFiles = 500

	err = filepath.WalkDir(resolved, func(walkPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(resolved, walkPath)
		if err != nil {
			return nil
		}
		if rel == "." {
			rel = "/"
		}

		depth := strings.Count(rel, string(filepath.Separator))

		if d.IsDir() {
			if rel == "/" {
				return nil // skip root
			}
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if maxDepth >= 0 && depth > maxDepth {
				return filepath.SkipDir
			}
			result.WriteString(strings.Repeat("  ", depth) + d.Name() + "/\n")
			if depth > walkDepth {
				walkDepth = depth
			}
			return nil
		}

		if maxDepth >= 0 && depth > maxDepth {
			return nil
		}

		fileCount++
		if fileCount > maxFiles {
			result.WriteString(strings.Repeat("  ", depth) + "... truncated (too many files) ...\n")
			return filepath.SkipDir
		}

		result.WriteString(strings.Repeat("  ", depth) + d.Name() + "\n")
		return nil
	})

	if err != nil {
		return Result{Error: err.Error()}, err
	}

	// Truncate if too large
	output := result.String()
	const maxSubtreeBytes = 8000
	if len(output) > maxSubtreeBytes {
		output = output[:maxSubtreeBytes] + "\n... truncated ..."
	}

	if output == "" {
		return Result{Content: "No files found."}, nil
	}

	return Result{Content: output}, nil
}

func parseGoFileSymbols(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var funcs, types, vars []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "func ") {
			name := extractIdentifier(line, "func ")
			if name != "" {
				funcs = append(funcs, name)
			}
		} else if strings.HasPrefix(line, "type ") {
			name := extractIdentifier(line, "type ")
			if name != "" {
				types = append(types, name)
			}
		} else if strings.HasPrefix(line, "var ") {
			name := extractIdentifier(line, "var ")
			if name != "" {
				vars = append(vars, name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	var parts []string
	if len(funcs) > 0 {
		sort.Strings(funcs)
		parts = append(parts, "func: "+strings.Join(funcs, ", "))
	}
	if len(types) > 0 {
		sort.Strings(types)
		parts = append(parts, "type: "+strings.Join(types, ", "))
	}
	if len(vars) > 0 {
		sort.Strings(vars)
		parts = append(parts, "var: "+strings.Join(vars, ", "))
	}

	if len(parts) == 0 {
		return "", nil
	}
	return "[" + strings.Join(parts, "; ") + "]", nil
}

func parseSourceSymbols(path string, ext string) (string, error) {
	switch ext {
	case ".go":
		return parseGoFileSymbols(path)
	case ".py":
		return parsePyFileSymbols(path)
	case ".ts", ".tsx":
		return parseTSFileSymbols(path)
	case ".js", ".jsx":
		return parseJSFileSymbols(path)
	case ".rs":
		return parseRustFileSymbols(path)
	case ".java":
		return parseJavaFileSymbols(path)
	default:
		return "", nil
	}
}

func parsePyFileSymbols(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var funcs, classes []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		// Only match top-level definitions (no leading spaces)
		if line != trimmed {
			continue
		}
		if strings.HasPrefix(trimmed, "def ") {
			name := extractIdentifier(trimmed, "def ")
			if name != "" {
				funcs = append(funcs, name)
			}
		} else if strings.HasPrefix(trimmed, "class ") {
			name := extractIdentifier(trimmed, "class ")
			if name != "" {
				classes = append(classes, name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	var parts []string
	if len(funcs) > 0 {
		sort.Strings(funcs)
		parts = append(parts, "func: "+strings.Join(funcs, ", "))
	}
	if len(classes) > 0 {
		sort.Strings(classes)
		parts = append(parts, "class: "+strings.Join(classes, ", "))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "[" + strings.Join(parts, "; ") + "]", nil
}

func extractIdentifier(line, prefix string) string {
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimSpace(rest)
	// Handle generic types: "func Foo[T any]("
	if idx := strings.IndexAny(rest, "(<["); idx >= 0 {
		return strings.TrimSpace(rest[:idx])
	}
	// Handle var/type with = or struct/interface
	if idx := strings.IndexAny(rest, " ="); idx >= 0 {
		return strings.TrimSpace(rest[:idx])
	}
	return rest
}

func parseTSFileSymbols(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var funcs, classes, interfaces, vars []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "export function "), strings.HasPrefix(line, "export async function "):
			prefix := "export async function "
			if !strings.HasPrefix(line, prefix) {
				prefix = "export function "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				funcs = append(funcs, name)
			}
		case strings.HasPrefix(line, "function "):
			name := extractIdentifier(line, "function ")
			if name != "" {
				funcs = append(funcs, name)
			}
		case strings.HasPrefix(line, "export class "):
			name := extractIdentifier(line, "export class ")
			if name != "" {
				classes = append(classes, name)
			}
		case strings.HasPrefix(line, "class "):
			name := extractIdentifier(line, "class ")
			if name != "" {
				classes = append(classes, name)
			}
		case strings.HasPrefix(line, "export interface "):
			name := extractIdentifier(line, "export interface ")
			if name != "" {
				interfaces = append(interfaces, name)
			}
		case strings.HasPrefix(line, "interface "):
			name := extractIdentifier(line, "interface ")
			if name != "" {
				interfaces = append(interfaces, name)
			}
		case strings.HasPrefix(line, "export const "), strings.HasPrefix(line, "export let "), strings.HasPrefix(line, "export var "):
			for _, p := range []string{"export const ", "export let ", "export var "} {
				if strings.HasPrefix(line, p) {
					name := extractIdentifier(line, p)
					if name != "" {
						vars = append(vars, name)
					}
					break
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	var parts []string
	if len(funcs) > 0 {
		sort.Strings(funcs)
		parts = append(parts, "func: "+strings.Join(funcs, ", "))
	}
	if len(classes) > 0 {
		sort.Strings(classes)
		parts = append(parts, "class: "+strings.Join(classes, ", "))
	}
	if len(interfaces) > 0 {
		sort.Strings(interfaces)
		parts = append(parts, "interface: "+strings.Join(interfaces, ", "))
	}
	if len(vars) > 0 {
		sort.Strings(vars)
		parts = append(parts, "const: "+strings.Join(vars, ", "))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "[" + strings.Join(parts, "; ") + "]", nil
}

func parseJSFileSymbols(path string) (string, error) {
	return parseTSFileSymbols(path) // JS uses same patterns
}

func parseRustFileSymbols(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var funcs, structs, enums, traits []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "pub fn "), strings.HasPrefix(line, "fn "):
			prefix := "pub fn "
			if !strings.HasPrefix(line, prefix) {
				prefix = "fn "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				funcs = append(funcs, name)
			}
		case strings.HasPrefix(line, "pub struct "), strings.HasPrefix(line, "struct "):
			prefix := "pub struct "
			if !strings.HasPrefix(line, prefix) {
				prefix = "struct "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				structs = append(structs, name)
			}
		case strings.HasPrefix(line, "pub enum "), strings.HasPrefix(line, "enum "):
			prefix := "pub enum "
			if !strings.HasPrefix(line, prefix) {
				prefix = "enum "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				enums = append(enums, name)
			}
		case strings.HasPrefix(line, "pub trait "), strings.HasPrefix(line, "trait "):
			prefix := "pub trait "
			if !strings.HasPrefix(line, prefix) {
				prefix = "trait "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				traits = append(traits, name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	var parts []string
	if len(funcs) > 0 {
		sort.Strings(funcs)
		parts = append(parts, "fn: "+strings.Join(funcs, ", "))
	}
	if len(structs) > 0 {
		sort.Strings(structs)
		parts = append(parts, "struct: "+strings.Join(structs, ", "))
	}
	if len(enums) > 0 {
		sort.Strings(enums)
		parts = append(parts, "enum: "+strings.Join(enums, ", "))
	}
	if len(traits) > 0 {
		sort.Strings(traits)
		parts = append(parts, "trait: "+strings.Join(traits, ", "))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "[" + strings.Join(parts, "; ") + "]", nil
}

func parseJavaFileSymbols(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var classes, interfaces, methods []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "public class "), strings.HasPrefix(line, "class "):
			prefix := "public class "
			if !strings.HasPrefix(line, prefix) {
				prefix = "class "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				classes = append(classes, name)
			}
		case strings.HasPrefix(line, "public interface "), strings.HasPrefix(line, "interface "):
			prefix := "public interface "
			if !strings.HasPrefix(line, prefix) {
				prefix = "interface "
			}
			name := extractIdentifier(line, prefix)
			if name != "" {
				interfaces = append(interfaces, name)
			}
		case strings.HasPrefix(line, "public "):
			// Try to match methods: "public Type methodName("
			rest := strings.TrimPrefix(line, "public ")
			if idx := strings.Index(rest, "("); idx >= 0 {
				// Find the word just before '('
				beforeParen := strings.TrimSpace(rest[:idx])
				words := strings.Fields(beforeParen)
				if len(words) >= 2 {
					// Check it's not a class/interface declaration
					_, err := fmt.Sscanf(words[len(words)-1], "%s", &rest)
					if err == nil || words[len(words)-2] != "class" && words[len(words)-2] != "interface" {
						name := words[len(words)-1]
						if name != "" {
							methods = append(methods, name)
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	var parts []string
	if len(classes) > 0 {
		sort.Strings(classes)
		parts = append(parts, "class: "+strings.Join(classes, ", "))
	}
	if len(interfaces) > 0 {
		sort.Strings(interfaces)
		parts = append(parts, "interface: "+strings.Join(interfaces, ", "))
	}
	if len(methods) > 0 {
		sort.Strings(methods)
		parts = append(parts, "method: "+strings.Join(methods, ", "))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "[" + strings.Join(parts, "; ") + "]", nil
}
