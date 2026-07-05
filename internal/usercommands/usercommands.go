// Package usercommands loads user-defined slash commands from markdown files.
//
// A user drops a file at `.cli_mate/commands/<name>.md` (project) or
// `<userConfigDir>/cli_mate/commands/<name>.md` (personal), with optional
// YAML-style frontmatter:
//
//	---
//	description: Open a PR for the current branch
//	model: gpt-4.1
//	---
//	Create a pull request for the current branch. Title: $1. Summarize: $ARGUMENTS
//
// Typing `/<name> some args` expands the body template ($ARGUMENTS, $1..$N) and
// submits it as a normal prompt.
package usercommands

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Command is a user-defined slash command parsed from a markdown file.
type Command struct {
	Name        string // the `/name` (file basename, lowercased, sans .md)
	Description string // frontmatter `description:`, for help + autocomplete
	Model       string // optional frontmatter `model:` override
	Template    string // the markdown body, expanded on invocation
	Path        string // source file, for diagnostics
	Project     bool   // true if from the project `.cli_mate/commands` dir
}

// Paths are the directories scanned for command files, project first.
type Paths struct {
	ProjectDir string
	UserDir    string
}

// DefaultPaths returns the project and user command directories.
func DefaultPaths(workspaceRoot, userConfigDir string) Paths {
	p := Paths{}
	if strings.TrimSpace(workspaceRoot) != "" {
		p.ProjectDir = filepath.Join(workspaceRoot, ".cli_mate", "commands")
	}
	if strings.TrimSpace(userConfigDir) != "" {
		p.UserDir = filepath.Join(userConfigDir, "cli_mate", "commands")
	}
	return p
}

// Load reads every `*.md` command file under the given paths and returns them
// keyed by lowercased name, sorted by name. A project command shadows a user
// command of the same name.
func Load(paths Paths) []Command {
	commands := map[string]Command{}

	// Load user commands first (lower priority)
	if paths.UserDir != "" {
		loadDir(paths.UserDir, false, commands)
	}

	// Load project commands (higher priority, shadows user)
	if paths.ProjectDir != "" {
		loadDir(paths.ProjectDir, true, commands)
	}

	// Sort by name for deterministic order
	result := make([]Command, 0, len(commands))
	for _, cmd := range commands {
		result = append(result, cmd)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Expand takes a command template and user arguments, replacing $ARGUMENTS
// and $1..$N placeholders.
func Expand(template string, args string) string {
	result := template
	result = strings.ReplaceAll(result, "$ARGUMENTS", args)

	parts := strings.Fields(args)
	for i, part := range parts {
		placeholder := "$" + string(rune('1'+i))
		result = strings.ReplaceAll(result, placeholder, part)
	}

	// Clear remaining unexpanded placeholders
	for i := len(parts) + 1; i <= 9; i++ {
		placeholder := "$" + string(rune('0'+i))
		result = strings.ReplaceAll(result, placeholder, "")
	}

	return strings.TrimSpace(result)
}

func loadDir(dir string, project bool, commands map[string]Command) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		name = strings.ToLower(name)
		if name == "" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		cmd, err := parseCommandFile(path, name, project)
		if err != nil {
			continue
		}

		commands[name] = cmd
	}
}

func parseCommandFile(path, name string, project bool) (Command, error) {
	f, err := os.Open(path)
	if err != nil {
		return Command{}, err
	}
	defer f.Close()

	var (
		description     string
		model           string
		body            strings.Builder
		inFrontmatter   bool
		frontmatterDone bool
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if !frontmatterDone {
			if strings.TrimSpace(line) == "---" {
				if !inFrontmatter {
					inFrontmatter = true
					continue
				}
				frontmatterDone = true
				continue
			}
			if inFrontmatter {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					switch strings.ToLower(key) {
					case "description":
						description = value
					case "model":
						model = value
					}
				}
				continue
			}
		}

		body.WriteString(line)
		body.WriteString("\n")
	}

	return Command{
		Name:        name,
		Description: description,
		Model:       model,
		Template:    strings.TrimSpace(body.String()),
		Path:        path,
		Project:     project,
	}, nil
}
