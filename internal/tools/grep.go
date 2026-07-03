package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type GrepTool struct {
	Root string
}

func NewGrepTool(root string) *GrepTool {
	return &GrepTool{Root: root}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Search file contents using a regular expression or fixed string. Returns matching lines with file:line:content format.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"pattern"},
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Search pattern. Uses Go regexp syntax by default, or literal search when fixed_string=true. Example: 'func [A-Z]' to find exported functions.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory or file to search within (default: workspace root)",
				},
				"glob": map[string]any{
					"type":        "string",
					"description": "Glob pattern to include files, e.g. '*.go', '*.{ts,js}'",
				},
				"exclude_glob": map[string]any{
					"type":        "string",
					"description": "Glob pattern to exclude files, e.g. '*_test.go' to exclude test files",
				},
				"ignore_case": map[string]any{
					"type":        "boolean",
					"description": "If true, perform case-insensitive matching. Default: false",
				},
				"fixed_string": map[string]any{
					"type":        "boolean",
					"description": "If true, treat pattern as a literal string instead of regex. Default: false",
				},
				"whole_word": map[string]any{
					"type":        "boolean",
					"description": "If true, only match whole words (pattern must be surrounded by non-word characters). Default: false",
				},
				"context_lines": map[string]any{
					"type":        "integer",
					"description": "Number of context lines to show before and after each match (default: 0, max: 10)",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of matches to return (default: 200, max: 1000)",
				},
			},
			"description": "Search file contents using a regular expression or fixed string. Returns matching lines in 'file:line:content' format with optional context lines.",
		},
	}
}

func (t *GrepTool) Execute(_ context.Context, call Call) (Result, error) {
	pattern, _ := call.Argument["pattern"].(string)
	if strings.TrimSpace(pattern) == "" {
		return Result{Error: "pattern is required"}, fmt.Errorf("pattern is required")
	}

	ignoreCase, _ := call.Argument["ignore_case"].(bool)
	fixedString, _ := call.Argument["fixed_string"].(bool)
	wholeWord, _ := call.Argument["whole_word"].(bool)
	contextLines := 0
	if cl, ok := call.Argument["context_lines"].(float64); ok {
		contextLines = int(cl)
		if contextLines < 0 {
			contextLines = 0
		}
		if contextLines > 10 {
			contextLines = 10
		}
	}
	maxResults := 200
	if mr, ok := call.Argument["max_results"].(float64); ok {
		maxResults = int(mr)
		if maxResults < 1 {
			maxResults = 1
		}
		if maxResults > 1000 {
			maxResults = 1000
		}
	}

	// Build the regex pattern
	searchPattern := pattern
	if fixedString {
		searchPattern = regexp.QuoteMeta(pattern)
	}
	if wholeWord {
		searchPattern = `\b` + searchPattern + `\b`
	}
	if ignoreCase {
		searchPattern = "(?i)" + searchPattern
	}

	re, err := regexp.Compile(searchPattern)
	if err != nil {
		return Result{Error: "invalid pattern: " + err.Error()}, err
	}

	searchRoot := t.Root
	if path, ok := call.Argument["path"].(string); ok && strings.TrimSpace(path) != "" {
		resolved, err := resolveWorkspacePath(t.Root, path)
		if err != nil {
			return Result{Error: err.Error()}, err
		}
		if err := ensureExistingPathInWorkspace(t.Root, resolved); err != nil {
			return Result{Error: err.Error()}, err
		}
		searchRoot = resolved
	}

	globFilter, _ := call.Argument["glob"].(string)
	excludeGlob, _ := call.Argument["exclude_glob"].(string)

	var results []grepResult
	skipDirs := map[string]bool{".git": true, ".idea": true, "node_modules": true, "vendor": true, ".openclaude": true}

	err = filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply include glob filter
		if globFilter != "" {
			matched, _ := filepath.Match(globFilter, d.Name())
			if !matched {
				return nil
			}
		}

		// Apply exclude glob filter
		if excludeGlob != "" {
			excluded, _ := filepath.Match(excludeGlob, d.Name())
			if excluded {
				return nil
			}
		}

		// Skip binary files
		if isBinary(d.Name()) {
			return nil
		}

		matches := grepFileWithContext(path, re, contextLines)
		results = append(results, matches...)

		if countGrepMatches(results) >= maxResults {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return Result{Error: err.Error()}, err
	}

	// Sort by file then line
	sort.Slice(results, func(i, j int) bool {
		if results[i].file != results[j].file {
			return results[i].file < results[j].file
		}
		return results[i].line < results[j].line
	})

	if len(results) == 0 {
		return Result{Content: "No matches found."}, nil
	}

	// Deduplicate adjacent context lines from overlapping matches
	results = dedupGrepResults(results)

	// Format output
	var b strings.Builder
	currentFile := ""
	matchCount := 0
	for _, r := range results {
		if r.isMatch {
			if matchCount >= maxResults {
				break
			}
			matchCount++
		}
		rel, _ := filepath.Rel(searchRoot, r.file)
		if rel != currentFile {
			fmt.Fprintf(&b, "%s:\n", filepath.ToSlash(rel))
			currentFile = rel
		}
		if r.isMatch {
			fmt.Fprintf(&b, "  >%d:%s\n", r.line, r.text)
		} else {
			fmt.Fprintf(&b, "   %s\n", r.text)
		}
	}
	totalMatches := countGrepMatches(results)
	if totalMatches > maxResults {
		fmt.Fprintf(&b, "... and %d more matches\n", totalMatches-maxResults)
	}

	return Result{Content: b.String()}, nil
}

type grepResult struct {
	file      string
	line      int
	text      string
	isMatch   bool
	isContext bool
}

// grepFileWithContext searches a file and returns matches with surrounding context lines.
func grepFileWithContext(path string, re *regexp.Regexp, contextLines int) []grepResult {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []grepResult
	var before []grepResult
	seen := make(map[int]bool)
	afterRemaining := 0
	lineNum := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if re.MatchString(line) {
			for _, ctxLine := range before {
				if !seen[ctxLine.line] {
					results = append(results, ctxLine)
					seen[ctxLine.line] = true
				}
			}
			if !seen[lineNum] {
				results = append(results, grepResult{file: path, line: lineNum, text: line, isMatch: true})
				seen[lineNum] = true
			}
			before = before[:0]
			afterRemaining = contextLines
			continue
		}

		contextResult := grepResult{file: path, line: lineNum, text: line, isContext: true}
		if afterRemaining > 0 {
			if !seen[lineNum] {
				results = append(results, contextResult)
				seen[lineNum] = true
			}
			afterRemaining--
			continue
		}
		if contextLines > 0 {
			before = append(before, contextResult)
			if len(before) > contextLines {
				before = before[1:]
			}
		}
	}
	if scanner.Err() != nil {
		return results
	}

	return results
}

func countGrepMatches(results []grepResult) int {
	count := 0
	for _, result := range results {
		if result.isMatch {
			count++
		}
	}
	return count
}

// dedupGrepResults removes duplicate adjacent context lines from overlapping matches.
func dedupGrepResults(results []grepResult) []grepResult {
	if len(results) == 0 {
		return results
	}
	deduped := make([]grepResult, 0, len(results))
	seen := make(map[string]map[int]bool)
	for _, r := range results {
		if seen[r.file] == nil {
			seen[r.file] = make(map[int]bool)
		}
		if !seen[r.file][r.line] {
			deduped = append(deduped, r)
			seen[r.file][r.line] = true
		}
	}
	return deduped
}

func isBinary(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".bin": true, ".obj": true, ".o": true, ".a": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".bmp": true, ".ico": true, ".svg": true, ".webp": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".sqlite": true, ".db": true,
	}
	return binaryExts[ext]
}

// grepFile is kept for backward compatibility (plain line matching without context).
func grepFile(path string, re *regexp.Regexp) []grepResult {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []grepResult
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			results = append(results, grepResult{
				file:    path,
				line:    lineNum,
				text:    strings.TrimRight(line, "\r\n"),
				isMatch: true,
			})
		}
	}
	if scanner.Err() != nil {
		return results
	}
	return results
}
