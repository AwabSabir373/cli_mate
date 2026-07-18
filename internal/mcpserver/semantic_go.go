package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type goSymbol struct {
	Name      string
	Kind      string
	Path      string
	Line      int
	EndLine   int
	Start     int
	End       int
	BodyStart int
	BodyEnd   int
	Signature string
}

type parsedGoFile struct {
	path    string
	relPath string
	source  []byte
	file    *ast.File
	fset    *token.FileSet
	symbols []goSymbol
}

func handleGoSymbols(ctx context.Context, params map[string]any) (any, error) {
	files, err := loadGoFiles(ctx, stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	query := strings.ToLower(stringParam(params, "query", ""))
	maxResults := intParam(params, "max_results", 20, 1, 100)
	maxChars := intParam(params, "max_chars", 1600, 200, 8000)
	var rows []string
	for _, file := range files {
		for _, symbol := range file.symbols {
			if query != "" && !strings.Contains(strings.ToLower(symbol.Name), query) {
				continue
			}
			row := fmt.Sprintf("%s:%d %s %s :: %s", symbol.Path, symbol.Line, symbol.Kind, symbol.Name, symbol.Signature)
			if len(rows) >= maxResults || outputLength(rows)+len(row)+1 > maxChars {
				rows = append(rows, "... truncated; narrow query/path")
				return strings.Join(rows, "\n"), nil
			}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return "no symbols", nil
	}
	return strings.Join(rows, "\n"), nil
}

func handleFindGoSymbol(ctx context.Context, params map[string]any) (any, error) {
	name := stringParam(params, "name", "")
	if name == "" {
		return nil, fmt.Errorf("missing symbol name")
	}
	files, err := loadGoFiles(ctx, stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	includeBody, _ := params["include_body"].(bool)
	maxChars := intParam(params, "max_chars", 3000, 200, 12000)
	var matches []string
	for _, file := range files {
		for _, symbol := range file.symbols {
			if symbol.Name != name && !strings.EqualFold(symbol.Name, name) {
				continue
			}
			text := fmt.Sprintf("%s:%d-%d %s %s", symbol.Path, symbol.Line, symbol.EndLine, symbol.Kind, symbol.Signature)
			if includeBody {
				snippet := string(file.source[symbol.Start:symbol.End])
				text += "\n" + snippet
			}
			if outputLength(matches)+len(text)+1 > maxChars {
				matches = append(matches, "... truncated; use a narrower path or disable include_body")
				return strings.Join(matches, "\n\n"), nil
			}
			matches = append(matches, text)
		}
	}
	if len(matches) == 0 {
		return "symbol not found: " + name, nil
	}
	return strings.Join(matches, "\n\n"), nil
}

func handleFindGoReferences(ctx context.Context, params map[string]any) (any, error) {
	name := stringParam(params, "name", "")
	if name == "" {
		return nil, fmt.Errorf("missing symbol name")
	}
	files, err := loadGoFiles(ctx, stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	maxResults := intParam(params, "max_results", 12, 1, 100)
	maxChars := intParam(params, "max_chars", 1600, 200, 8000)
	var rows []string
	for _, file := range files {
		lines := bytes.Split(file.source, []byte("\n"))
		ast.Inspect(file.file, func(node ast.Node) bool {
			if len(rows) >= maxResults {
				return false
			}
			ident, ok := node.(*ast.Ident)
			if !ok || ident.Name != name {
				return true
			}
			position := file.fset.Position(ident.Pos())
			preview := ""
			if position.Line > 0 && position.Line <= len(lines) {
				preview = compactLine(string(lines[position.Line-1]), 140)
			}
			row := fmt.Sprintf("%s:%d:%d %s", file.relPath, position.Line, position.Column, preview)
			if outputLength(rows)+len(row)+1 <= maxChars {
				rows = append(rows, row)
			}
			return true
		})
		if len(rows) >= maxResults || outputLength(rows) >= maxChars {
			break
		}
	}
	if len(rows) == 0 {
		return "no references", nil
	}
	if len(rows) >= maxResults {
		rows = append(rows, "... truncated; narrow path")
	}
	return strings.Join(rows, "\n"), nil
}

func handleGoCallers(ctx context.Context, params map[string]any) (any, error) {
	name := stringParam(params, "name", "")
	if name == "" {
		return nil, fmt.Errorf("missing symbol name")
	}
	files, err := loadGoFiles(ctx, stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	maxResults := intParam(params, "max_results", 12, 1, 100)
	var rows []string
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			caller := functionName(fn)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || calledName(call.Fun) != name {
					return true
				}
				position := file.fset.Position(call.Pos())
				rows = append(rows, fmt.Sprintf("%s:%d %s -> %s", file.relPath, position.Line, caller, name))
				return len(rows) < maxResults
			})
			if len(rows) >= maxResults {
				break
			}
		}
		if len(rows) >= maxResults {
			break
		}
	}
	if len(rows) == 0 {
		return "no callers", nil
	}
	return strings.Join(rows, "\n"), nil
}

func handleReplaceGoSymbol(ctx context.Context, params map[string]any) (any, error) {
	requested := stringParam(params, "path", "")
	name := stringParam(params, "name", "")
	body := stringParam(params, "body", "")
	if requested == "" || name == "" || body == "" {
		return nil, fmt.Errorf("path, name, and body are required")
	}
	path, err := resolveWorkspacePath(requested)
	if err != nil {
		return nil, err
	}
	files, err := loadGoFiles(ctx, requested)
	if err != nil {
		return nil, err
	}
	if len(files) != 1 || files[0].path != path {
		return nil, fmt.Errorf("path must identify one Go file")
	}
	var match *goSymbol
	for index := range files[0].symbols {
		symbol := &files[0].symbols[index]
		if symbol.Name == name && symbol.Kind == "func" {
			if match != nil {
				return nil, fmt.Errorf("symbol %q is ambiguous; use receiver-qualified name", name)
			}
			match = symbol
		}
	}
	if match == nil || match.BodyStart < 0 {
		return nil, fmt.Errorf("function or method not found: %s", name)
	}
	replacement := strings.TrimSpace(body)
	if !strings.HasPrefix(replacement, "{") {
		replacement = "{\n" + replacement + "\n}"
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "replacement.go", "package p\nfunc replacement() "+replacement, parser.AllErrors); err != nil {
		return nil, fmt.Errorf("invalid replacement body: %w", err)
	}
	updated := append([]byte(nil), files[0].source[:match.BodyStart]...)
	updated = append(updated, replacement...)
	updated = append(updated, files[0].source[match.BodyEnd:]...)
	formatted, err := format.Source(updated)
	if err != nil {
		return nil, fmt.Errorf("replacement produced invalid Go: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, formatted, info.Mode().Perm()); err != nil {
		return nil, fmt.Errorf("write replacement: %w", err)
	}
	return fmt.Sprintf("replaced %s in %s; run go_diagnostics", name, filepath.ToSlash(requested)), nil
}

func handleGoDiagnostics(ctx context.Context, params map[string]any) (any, error) {
	path, err := resolveWorkspacePath(stringParam(params, "path", "."))
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(intParam(params, "timeout_seconds", 30, 1, 120)) * time.Second
	maxChars := intParam(params, "max_chars", 3000, 200, 12000)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, "go", "test", "./...")
	command.Dir = path
	output, runErr := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if len(text) > maxChars {
		text = text[:maxChars] + "\n... diagnostics truncated"
	}
	if runCtx.Err() != nil {
		return nil, fmt.Errorf("diagnostics timed out after %s", timeout)
	}
	if runErr != nil {
		if text == "" {
			text = runErr.Error()
		}
		return "FAIL\n" + text, nil
	}
	if text == "" {
		text = "go test ./... passed"
	}
	return "PASS\n" + text, nil
}

func loadGoFiles(ctx context.Context, requested string) ([]parsedGoFile, error) {
	root, err := resolveWorkspacePath(requested)
	if err != nil {
		return nil, err
	}
	workspace, _ := os.Getwd()
	var paths []string
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Ext(root) != ".go" {
			return nil, fmt.Errorf("path is not a Go file: %s", requested)
		}
		paths = append(paths, root)
	} else {
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() && path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			if !entry.IsDir() && filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	files := make([]parsedGoFile, 0, len(paths))
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(workspace, path)
		parsed := parsedGoFile{path: path, relPath: filepath.ToSlash(rel), source: source, file: file, fset: fset}
		parsed.symbols = collectGoSymbols(parsed)
		files = append(files, parsed)
	}
	return files, nil
}

func collectGoSymbols(file parsedGoFile) []goSymbol {
	var symbols []goSymbol
	for _, decl := range file.file.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			start, end := offsets(file.fset, node.Pos(), node.End())
			bodyStart, bodyEnd := -1, -1
			if node.Body != nil {
				bodyStart, bodyEnd = offsets(file.fset, node.Body.Pos(), node.Body.End())
			}
			symbols = append(symbols, newGoSymbol(file, functionName(node), "func", node, start, end, bodyStart, bodyEnd, functionSignature(file, node)))
		case *ast.GenDecl:
			kind := strings.ToLower(node.Tok.String())
			for _, spec := range node.Specs {
				switch item := spec.(type) {
				case *ast.TypeSpec:
					start, end := offsets(file.fset, item.Pos(), item.End())
					symbols = append(symbols, newGoSymbol(file, item.Name.Name, "type", item, start, end, -1, -1, kind+" "+item.Name.Name))
				case *ast.ValueSpec:
					start, end := offsets(file.fset, item.Pos(), item.End())
					for _, name := range item.Names {
						symbols = append(symbols, newGoSymbol(file, name.Name, kind, item, start, end, -1, -1, kind+" "+name.Name))
					}
				}
			}
		}
	}
	return symbols
}

func newGoSymbol(file parsedGoFile, name, kind string, node ast.Node, start, end, bodyStart, bodyEnd int, signature string) goSymbol {
	return goSymbol{
		Name: name, Kind: kind, Path: file.relPath,
		Line: file.fset.Position(node.Pos()).Line, EndLine: file.fset.Position(node.End()).Line,
		Start: start, End: end, BodyStart: bodyStart, BodyEnd: bodyEnd, Signature: signature,
	}
}

func offsets(fset *token.FileSet, start, end token.Pos) (startOffset, endOffset int) {
	return fset.Position(start).Offset, fset.Position(end).Offset
}

func functionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	receiver := exprName(fn.Recv.List[0].Type)
	return receiver + "." + fn.Name.Name
}

func functionSignature(file parsedGoFile, fn *ast.FuncDecl) string {
	end := file.fset.Position(fn.Type.End()).Offset
	start := file.fset.Position(fn.Pos()).Offset
	if start >= 0 && end > start && end <= len(file.source) {
		return compactLine(string(file.source[start:end]), 240)
	}
	return "func " + functionName(fn)
}

func exprName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return exprName(node.X)
	case *ast.IndexExpr:
		return exprName(node.X)
	case *ast.IndexListExpr:
		return exprName(node.X)
	case *ast.SelectorExpr:
		return exprName(node.X) + "." + node.Sel.Name
	default:
		return "?"
	}
}

func calledName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return node.Sel.Name
	case *ast.IndexExpr:
		return calledName(node.X)
	case *ast.IndexListExpr:
		return calledName(node.X)
	default:
		return ""
	}
}
