package mcpserver

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type languageSpec struct {
	name     string
	patterns []symbolPattern
}

type symbolPattern struct {
	kind  string
	regex *regexp.Regexp
	// kindGroup allows the source keyword (class, interface, fn...) to become
	// the result kind. nameGroup is always the captured symbol name.
	kindGroup int
	nameGroup int
}

type codeSymbol struct {
	Name      string
	Kind      string
	Language  string
	Path      string
	Line      int
	Signature string
	StartLine int
	EndLine   int
}

const (
	maxIndexedSourceFiles = 5000
	maxIndexedFileBytes   = 2 << 20
)

var languageByExtension = buildLanguageRegistry()

func buildLanguageRegistry() map[string]languageSpec {
	pattern := func(kind, expression string, kindGroup, nameGroup int) symbolPattern {
		return symbolPattern{kind: kind, regex: regexp.MustCompile(expression), kindGroup: kindGroup, nameGroup: nameGroup}
	}
	registry := map[string]languageSpec{}
	register := func(extensions []string, spec languageSpec) {
		for _, extension := range extensions {
			registry[extension] = spec
		}
	}

	register([]string{".py", ".pyi"}, languageSpec{name: "python", patterns: []symbolPattern{
		pattern("", `^\s*(?:async\s+)?(def|class)\s+([A-Za-z_]\w*)`, 1, 2),
	}})
	register([]string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}, languageSpec{name: "typescript", patterns: []symbolPattern{
		pattern("", `^\s*(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:async\s+)?(class|interface|type|enum|function|namespace)\s+([A-Za-z_$][\w$]*)`, 1, 2),
		pattern("function", `^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`, 0, 1),
	}})
	register([]string{".rs"}, languageSpec{name: "rust", patterns: []symbolPattern{
		pattern("", `^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(fn|struct|enum|trait|mod|type|union)\s+([A-Za-z_]\w*)`, 1, 2),
		pattern("impl", `^\s*impl(?:<[^>]+>)?\s+([^\s{]+)`, 0, 1),
	}})
	register([]string{".java", ".kt", ".kts", ".scala"}, languageSpec{name: "jvm", patterns: []symbolPattern{
		pattern("", `^\s*(?:(?:public|private|protected|internal|abstract|final|sealed|open|data|static)\s+)*(class|interface|enum|record|object|trait)\s+([A-Za-z_]\w*)`, 1, 2),
		pattern("function", `^\s*(?:(?:public|private|protected|internal|static|final|open|override|suspend|synchronized|abstract)\s+)*(?:fun\s+)?(?:[\w<>,.?\[\]]+\s+)?([A-Za-z_]\w*)\s*\([^;]*\)\s*(?:\{|=)`, 0, 1),
	}})
	register([]string{".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh"}, languageSpec{name: "cpp", patterns: []symbolPattern{
		pattern("", `^\s*(?:template\s*<[^>]+>\s*)?(class|struct|enum|union|namespace)\s+([A-Za-z_]\w*)`, 1, 2),
		pattern("function", `^\s*(?:[\w:*&<>,~]+\s+)+([A-Za-z_~]\w*(?:::\w+)*)\s*\([^;]*\)\s*(?:const\s*)?\{`, 0, 1),
	}})
	register([]string{".cs"}, languageSpec{name: "csharp", patterns: []symbolPattern{
		pattern("", `^\s*(?:(?:public|private|protected|internal|static|abstract|sealed|partial)\s+)*(class|struct|interface|enum|record|namespace)\s+([A-Za-z_]\w*)`, 1, 2),
		pattern("function", `^\s*(?:(?:public|private|protected|internal|static|virtual|override|async|sealed)\s+)+(?:[\w<>,.?\[\]]+\s+)([A-Za-z_]\w*)\s*\([^;]*\)\s*(?:\{|=>)`, 0, 1),
	}})
	register([]string{".php"}, languageSpec{name: "php", patterns: []symbolPattern{
		pattern("", `^\s*(?:(?:public|private|protected|final|abstract|static)\s+)*(class|interface|trait|enum|function)\s+([A-Za-z_]\w*)`, 1, 2),
	}})
	register([]string{".rb", ".rake"}, languageSpec{name: "ruby", patterns: []symbolPattern{
		pattern("", `^\s*(class|module|def)\s+(?:self\.)?([A-Za-z_]\w*[!?=]?)`, 1, 2),
	}})
	register([]string{".swift"}, languageSpec{name: "swift", patterns: []symbolPattern{
		pattern("", `^\s*(?:(?:public|private|internal|open|final|static|class)\s+)*(class|struct|protocol|enum|actor|func)\s+([A-Za-z_]\w*)`, 1, 2),
	}})
	register([]string{".dart"}, languageSpec{name: "dart", patterns: []symbolPattern{
		pattern("", `^\s*(?:abstract\s+)?(class|mixin|enum|extension|typedef)\s+([A-Za-z_]\w*)`, 1, 2),
		pattern("function", `^\s*(?:[\w<>,?\[\]]+\s+)([A-Za-z_]\w*)\s*\([^;]*\)\s*(?:\{|=>)`, 0, 1),
	}})
	register([]string{".lua"}, languageSpec{name: "lua", patterns: []symbolPattern{
		pattern("function", `^\s*(?:local\s+)?function\s+([A-Za-z_]\w*(?:[.:]\w+)*)`, 0, 1),
	}})
	register([]string{".sh", ".bash", ".zsh"}, languageSpec{name: "shell", patterns: []symbolPattern{
		pattern("function", `^\s*(?:function\s+)?([A-Za-z_]\w*)\s*(?:\(\))?\s*\{`, 0, 1),
	}})
	return registry
}

func handleCodeSymbols(ctx context.Context, params map[string]any) (any, error) {
	requested := stringParam(params, "path", ".")
	query := strings.ToLower(stringParam(params, "query", ""))
	language := strings.ToLower(stringParam(params, "language", ""))
	maxResults := intParam(params, "max_results", 30, 1, 200)
	maxChars := intParam(params, "max_chars", 2400, 200, 12000)
	symbols, err := indexCodeSymbols(ctx, requested)
	if err != nil {
		return nil, err
	}
	var rows []string
	for _, symbol := range symbols {
		if language != "" && symbol.Language != language {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(symbol.Name), query) {
			continue
		}
		row := fmt.Sprintf("%s:%d [%s] %s %s :: %s", symbol.Path, symbol.Line, symbol.Language, symbol.Kind, symbol.Name, symbol.Signature)
		if len(rows) >= maxResults || outputLength(rows)+len(row)+1 > maxChars {
			rows = append(rows, "... truncated; narrow path/query/language")
			break
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return "no symbols", nil
	}
	return strings.Join(rows, "\n"), nil
}

func handleFindCodeSymbol(ctx context.Context, params map[string]any) (any, error) {
	name := stringParam(params, "name", "")
	requested := stringParam(params, "path", ".")
	if name == "" {
		return nil, fmt.Errorf("missing symbol name")
	}
	// Go keeps its stronger AST-qualified implementation.
	if filepath.Ext(requested) == ".go" {
		return handleFindGoSymbol(ctx, params)
	}
	symbols, err := indexCodeSymbols(ctx, requested)
	if err != nil {
		return nil, err
	}
	includeBody, _ := params["include_body"].(bool)
	maxChars := intParam(params, "max_chars", 3000, 200, 12000)
	var rows []string
	for _, symbol := range symbols {
		if symbol.Name != name && !strings.EqualFold(symbol.Name, name) {
			continue
		}
		row := fmt.Sprintf("%s:%d-%d [%s] %s %s", symbol.Path, symbol.StartLine, symbol.EndLine, symbol.Language, symbol.Kind, symbol.Signature)
		if includeBody {
			body, readErr := readLineRange(symbol.Path, symbol.StartLine, symbol.EndLine, maxChars-outputLength(rows)-len(row))
			if readErr == nil {
				row += "\n" + body
			}
		}
		if outputLength(rows)+len(row)+1 > maxChars {
			rows = append(rows, "... truncated; narrow path or disable include_body")
			break
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return "symbol not found: " + name, nil
	}
	return strings.Join(rows, "\n\n"), nil
}

func indexCodeSymbols(ctx context.Context, requested string) ([]codeSymbol, error) {
	root, err := resolveWorkspacePath(requested)
	if err != nil {
		return nil, err
	}
	workspace, _ := os.Getwd()
	var symbols []codeSymbol
	indexedFiles := 0
	visit := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if indexedFiles >= maxIndexedSourceFiles {
			return filepath.SkipAll
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".go" {
			if info, statErr := entry.Info(); statErr != nil || info.Size() > maxIndexedFileBytes {
				return nil
			}
			indexedFiles++
			files, loadErr := loadGoFiles(ctx, path)
			if loadErr != nil || len(files) != 1 {
				return nil
			}
			for _, item := range files[0].symbols {
				symbols = append(symbols, codeSymbol{Name: item.Name, Kind: item.Kind, Language: "go", Path: item.Path, Line: item.Line, Signature: item.Signature, StartLine: item.Line, EndLine: item.EndLine})
			}
			return nil
		}
		spec, ok := languageByExtension[extension]
		if !ok {
			return nil
		}
		if info, statErr := entry.Info(); statErr != nil || info.Size() > maxIndexedFileBytes {
			return nil
		}
		indexedFiles++
		found, parseErr := scanLanguageFile(path, workspace, spec)
		if parseErr == nil {
			symbols = append(symbols, found...)
		}
		return nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		err = filepath.WalkDir(root, visit)
	} else {
		err = visit(root, fs.FileInfoToDirEntry(info), nil)
	}
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Path == symbols[j].Path {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].Path < symbols[j].Path
	})
	return symbols, err
}

func scanLanguageFile(path, workspace string, spec languageSpec) ([]codeSymbol, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	rel, _ := filepath.Rel(workspace, path)
	var symbols []codeSymbol
	for index, line := range lines {
		for _, pattern := range spec.patterns {
			match := pattern.regex.FindStringSubmatch(line)
			if match == nil || pattern.nameGroup >= len(match) {
				continue
			}
			kind := pattern.kind
			if pattern.kindGroup > 0 && pattern.kindGroup < len(match) {
				kind = match[pattern.kindGroup]
			}
			name := match[pattern.nameGroup]
			end := symbolEndLine(lines, index, spec.name)
			symbols = append(symbols, codeSymbol{
				Name: name, Kind: kind, Language: spec.name, Path: filepath.ToSlash(rel),
				Line: index + 1, Signature: compactLine(line, 240), StartLine: index + 1, EndLine: end,
			})
			break
		}
	}
	return symbols, nil
}

func symbolEndLine(lines []string, start int, language string) int {
	if language == "python" || language == "ruby" {
		baseIndent := leadingWhitespace(lines[start])
		for index := start + 1; index < len(lines); index++ {
			if strings.TrimSpace(lines[index]) == "" {
				continue
			}
			if leadingWhitespace(lines[index]) <= baseIndent {
				return index
			}
		}
		return len(lines)
	}
	depth, opened := 0, false
	for index := start; index < len(lines); index++ {
		depth += strings.Count(lines[index], "{")
		if depth > 0 {
			opened = true
		}
		depth -= strings.Count(lines[index], "}")
		if opened && depth <= 0 {
			return index + 1
		}
		if !opened && index > start+4 {
			return start + 1
		}
	}
	return start + 1
}

func leadingWhitespace(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func readLineRange(requested string, start, end, maxChars int) (string, error) {
	path, err := resolveWorkspacePath(requested)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	if maxChars < 200 {
		maxChars = 200
	}
	var rows []string
	used := 0
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		if lineNo < start {
			continue
		}
		if lineNo > end {
			break
		}
		row := fmt.Sprintf("%d|%s", lineNo, scanner.Text())
		if used+len(row)+1 > maxChars {
			rows = append(rows, "... truncated")
			break
		}
		rows = append(rows, row)
		used += len(row) + 1
	}
	return strings.Join(rows, "\n"), scanner.Err()
}
