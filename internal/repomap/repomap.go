// Package repomap builds a deterministic repository characterization for the
// agent's system prompt. It analyzes file extensions, entry points, and project
// structure to give the model a high-level understanding of the codebase.
package repomap

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// languageExtensions maps file extensions to language names.
var languageExtensions = map[string]string{
	".go":    "Go",
	".py":    "Python",
	".js":    "JavaScript",
	".ts":    "TypeScript",
	".tsx":   "TypeScript",
	".jsx":   "JavaScript",
	".rs":    "Rust",
	".java":  "Java",
	".rb":    "Ruby",
	".php":   "PHP",
	".c":     "C",
	".cpp":   "C++",
	".h":     "C/C++ Header",
	".cs":    "C#",
	".swift": "Swift",
	".kt":    "Kotlin",
	".scala": "Scala",
	".sh":    "Shell",
	".bash":  "Shell",
	".yaml":  "YAML",
	".yml":   "YAML",
	".json":  "JSON",
	".toml":  "TOML",
	".xml":   "XML",
	".sql":   "SQL",
	".md":    "Markdown",
	".html":  "HTML",
	".css":   "CSS",
	".scss":  "SCSS",
}

// entryPointPatterns are common entry point file patterns.
var entryPointPatterns = []string{
	"main.go",
	"main.py",
	"index.js",
	"index.ts",
	"app.js",
	"app.ts",
	"server.js",
	"server.ts",
	"index.html",
	"cmd/*/main.go",
}

// Map is a structured characterization of a repository.
type Map struct {
	Root        string
	Languages   map[string]int // language -> file count
	EntryPoints []string       // detected entry points
	TestFiles   []string       // test file patterns
	ConfigFiles []string       // configuration files
	KeyDirs     []string       // important directories
}

// Build constructs a repo map by scanning the directory tree.
func Build(root string, maxDepth int) Map {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	m := Map{
		Root:      root,
		Languages: make(map[string]int),
	}

	seen := map[string]bool{}
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > maxDepth {
			return filepath.SkipDir
		}

		// Skip common non-project directories
		dirParts := strings.Split(rel, string(os.PathSeparator))
		for _, part := range dirParts[:len(dirParts)-1] {
			switch part {
			case ".git", "node_modules", "vendor", ".idea", ".vscode",
				".openclaude", ".cli_mate", ".mimocode", ".claude", "__pycache__":
				return nil
			}
		}

		ext := strings.ToLower(filepath.Ext(path))
		if lang, ok := languageExtensions[ext]; ok {
			m.Languages[lang]++
		}

		name := d.Name()
		if isTestFile(name) && !seen["test:"+ext] {
			m.TestFiles = append(m.TestFiles, "*"+ext)
			seen["test:"+ext] = true
		}

		if isConfigFile(name) && !seen["config:"+name] {
			m.ConfigFiles = append(m.ConfigFiles, name)
			seen["config:"+name] = true
		}

		for _, pattern := range entryPointPatterns {
			if matched, _ := filepath.Match(pattern, name); matched && !seen["entry:"+rel] {
				m.EntryPoints = append(m.EntryPoints, rel)
				seen["entry:"+rel] = true
			}
		}

		return nil
	})

	// Detect key directories
	m.KeyDirs = detectKeyDirs(root)

	return m
}

// Render produces a compact text summary for the system prompt.
func (m Map) Render(maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}

	var b strings.Builder
	b.WriteString("Repository Analysis:\n")

	// Languages sorted by count
	type langCount struct {
		lang  string
		count int
	}
	var langs []langCount
	for lang, count := range m.Languages {
		langs = append(langs, langCount{lang, count})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].count > langs[j].count
	})
	if len(langs) > 0 {
		var parts []string
		shown := langs
		if len(shown) > 5 {
			shown = shown[:5]
		}
		for _, l := range shown {
			parts = append(parts, l.lang)
		}
		b.WriteString("  Languages: " + strings.Join(parts, ", ") + "\n")
	}

	// Entry points
	if len(m.EntryPoints) > 0 {
		shown := m.EntryPoints
		if len(shown) > 3 {
			shown = shown[:3]
		}
		b.WriteString("  Entry points: " + strings.Join(shown, ", ") + "\n")
	}

	// Test files
	if len(m.TestFiles) > 0 {
		b.WriteString("  Test patterns: " + strings.Join(m.TestFiles, ", ") + "\n")
	}

	// Config files
	if len(m.ConfigFiles) > 0 {
		shown := m.ConfigFiles
		if len(shown) > 5 {
			shown = shown[:5]
		}
		b.WriteString("  Config: " + strings.Join(shown, ", ") + "\n")
	}

	// Key directories
	if len(m.KeyDirs) > 0 {
		shown := m.KeyDirs
		if len(shown) > maxLines {
			shown = shown[:maxLines]
		}
		b.WriteString("  Key dirs: " + strings.Join(shown, ", ") + "\n")
	}

	return b.String()
}

func isTestFile(name string) bool {
	return strings.HasSuffix(name, "_test.go") ||
		strings.HasSuffix(name, ".test.js") ||
		strings.HasSuffix(name, ".test.ts") ||
		strings.HasSuffix(name, ".spec.js") ||
		strings.HasSuffix(name, ".spec.ts") ||
		strings.HasSuffix(name, "_test.py") ||
		strings.HasSuffix(name, "_test.rs")
}

func isConfigFile(name string) bool {
	configs := map[string]bool{
		"go.mod": true, "go.sum": true,
		"package.json": true, "tsconfig.json": true,
		"Cargo.toml": true, "pyproject.toml": true,
		"Makefile": true, "Dockerfile": true,
		".goreleaser.yml": true, ".golangci.yml": true,
		"docker-compose.yml": true, "docker-compose.yaml": true,
		".env": true, ".env.example": true,
	}
	return configs[name]
}

func detectKeyDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var dirs []string
	important := map[string]bool{
		"cmd": true, "internal": true, "pkg": true,
		"src": true, "lib": true, "app": true, "api": true,
		"test": true, "tests": true, "spec": true,
		"docs": true, "doc": true,
		"scripts": true, "tools": true,
		"migrations": true, "db": true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if important[name] || strings.HasPrefix(name, "cmd") || strings.HasPrefix(name, "internal") {
				dirs = append(dirs, name+"/")
			}
		}
	}
	return dirs
}
