package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cli_mate/internal/providers/registry"
	"cli_mate/internal/usercommands"
)

// Suggestions are derived from the current input only. Input modes that expect
// raw text, such as API keys and custom models, intentionally disable them.
func (a App) currentSuggestions() []suggestion {
	if a.inputMode == "api_key" || a.inputMode == "custom_model" || a.inputMode == "finder" {
		return nil
	}

	input := strings.TrimSpace(a.input)
	if isProviderInput(input) {
		return providerSuggestions(providerQuery(input))
	}
	if isModelInput(input) {
		return modelSuggestions(a.activeProfile().Provider, modelQuery(input))
	}
	if isThemeInput(input) {
		return themeSuggestions(themeQuery(input))
	}
	if strings.HasPrefix(input, "/") {
		return commandSuggestions(strings.TrimPrefix(input, "/"), a.userCommands)
	}

	token, ok := activeMentionToken(a.input)
	if ok {
		return fileSuggestions(a.files, strings.TrimPrefix(token, "@"))
	}
	return nil
}

func commandSuggestions(prefix string, userCmds []usercommands.Command) []suggestion {
	commands := []suggestion{
		{Value: "/help", Label: "/help", Description: "show available commands"},
		{Value: "/open ", Label: "/open", Description: "preview a file"},
		{Value: "/setup", Label: "/setup", Description: "run the interactive setup wizard"},
		{Value: "/resume", Label: "/resume", Description: "resume a previous session"},
		{Value: "/mcp", Label: "/mcp", Description: "manage MCP servers"},
		{Value: "/mcp_server", Label: "/mcp_server", Description: "open custom mcp"},
		{Value: "/prs", Label: "/prs", Description: "show pull request status"},
		{Value: "/spec", Label: "/spec", Description: "start specification-driven development"},
		{Value: "/doctor", Label: "/doctor", Description: "run system diagnostics"},
		{Value: "/attach ", Label: "/attach", Description: "attach an image to the conversation"},
		{Value: "/session", Label: "/session", Description: "open session controls (rename/export/rewind)"},
		{Value: "/output", Label: "/output", Description: "view command output history"},
		{Value: "/provider ", Label: "/provider", Description: "choose one active provider"},
		{Value: "/model ", Label: "/model", Description: "choose model for active provider"},
		{Value: "/theme ", Label: "/theme", Description: "choose terminal theme"},
		{Value: "/api-key ", Label: "/api-key", Description: "set or update API key"},
		{Value: "/max-tokens ", Label: "/max-tokens", Description: "set custom context level limit"},
		{Value: "/base-url ", Label: "/base-url", Description: "set local provider URL"},
		{Value: "/connect", Label: "/connect", Description: "validate active provider"},
		{Value: "/approve", Label: "/approve", Description: "toggle auto-approve for tool execution"},
		{Value: "/status", Label: "/status", Description: "show configuration"},
		{Value: "/copy", Label: "/copy", Description: "copy last AI response to clipboard"},
		{Value: "/clear", Label: "/clear", Description: "clear the console"},
		{Value: "/review", Label: "/review", Description: "review code changes"},
		{Value: "/diff ", Label: "/diff", Description: "show git diff"},
		{Value: "/commit ", Label: "/commit", Description: "create a git commit"},
		{Value: "/compact", Label: "/compact", Description: "clean temporary files"},
		{Value: "/skills", Label: "/skills", Description: "list available skills"},
	}

	// Add user commands
	for _, cmd := range userCmds {
		desc := cmd.Description
		if desc == "" {
			desc = "custom command"
		}
		commands = append(commands, suggestion{
			Value:       "/" + cmd.Name + " ",
			Label:       "/" + cmd.Name,
			Description: desc,
		})
	}

	var matches []suggestion
	for _, command := range commands {
		if strings.HasPrefix(strings.TrimPrefix(command.Label, "/"), prefix) {
			matches = append(matches, command)
		}
	}
	return limitSuggestions(matches, 12)
}

func providerSuggestions(prefix string) []suggestion {
	prefix = strings.ToLower(prefix)
	var matches []suggestion
	for _, spec := range registry.Specs() {
		if prefix != "" && !strings.HasPrefix(spec.Name, prefix) {
			continue
		}

		auth := "local"
		if spec.RequiresKey {
			auth = "api key required"
		}
		matches = append(matches, suggestion{
			Value:       "/provider " + spec.Name,
			Label:       spec.Name,
			Description: fmt.Sprintf("model %s, %s", spec.DefaultModel, auth),
		})
	}
	return matches
}

func modelSuggestions(provider string, prefix string) []suggestion {
	prefix = strings.ToLower(prefix)
	models := registry.Models(provider)
	if len(models) == 0 {
		models = []string{"openai/gpt-4.1-mini", "gemini-2.5-flash", "llama-3.3-70b-versatile", "llama3.1"}
	}

	var matches []suggestion
	for _, model := range models {
		if prefix != "" && !strings.Contains(strings.ToLower(model), prefix) {
			continue
		}
		matches = append(matches, suggestion{
			Value:       "/model " + model,
			Label:       model,
			Description: provider + " model",
		})
	}

	if prefix == "" || strings.HasPrefix("custom", prefix) {
		matches = append(matches, suggestion{
			Value:       "/model custom",
			Label:       "custom",
			Description: "type a custom model id",
		})
	}
	return limitSuggestions(matches, 8)
}

func isCustomModelChoice(args []string) bool {
	return len(args) == 1 && strings.EqualFold(args[0], "custom")
}

func themeSuggestions(prefix string) []suggestion {
	themes := []string{"midnight", "matrix", "paper", "mono", "catppuccin", "dracula", "nord", "gruvbox", "tokyonight", "rosepine", "solarized", "onedark"}
	var matches []suggestion
	for _, theme := range themes {
		if prefix != "" && !strings.HasPrefix(theme, strings.ToLower(prefix)) {
			continue
		}
		matches = append(matches, suggestion{
			Value:       "/theme " + theme,
			Label:       theme,
			Description: "terminal look",
		})
	}
	return matches
}

func isProviderInput(input string) bool {
	return input == "/provider" || strings.HasPrefix(input, "/provider ")
}

func providerQuery(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(input, "/provider"))
}

func isModelInput(input string) bool {
	return input == "/model" || strings.HasPrefix(input, "/model ")
}

func modelQuery(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(input, "/model"))
}

func isThemeInput(input string) bool {
	return input == "/theme" || strings.HasPrefix(input, "/theme ")
}

func themeQuery(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(input, "/theme"))
}

func validTheme(theme string) bool {
	for _, candidate := range []string{"midnight", "matrix", "paper", "mono", "catppuccin", "dracula", "nord", "gruvbox", "tokyonight", "rosepine", "solarized", "onedark"} {
		if theme == candidate {
			return true
		}
	}
	return false
}

func fileSuggestions(files []string, prefix string) []suggestion {
	prefix = filepath.ToSlash(strings.ToLower(prefix))
	var matches []suggestion
	for _, file := range files {
		lower := strings.ToLower(file)
		if prefix == "" || strings.HasPrefix(lower, prefix) || strings.Contains(lower, prefix) {
			kind := "file"
			if strings.HasSuffix(file, "/") {
				kind = "dir"
			}
			matches = append(matches, suggestion{
				Value:       file,
				Label:       "@" + file,
				Description: kind,
			})
		}
	}
	return limitSuggestions(matches, 15)
}

func activeMentionToken(input string) (string, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", false
	}
	last := parts[len(parts)-1]
	return last, strings.HasPrefix(last, "@")
}

func workspaceFiles(root string, limit int) []string {
	// Hardcoded skip list for known large non-code directories.
	skipDirs := map[string]bool{
		"vendor": true, "node_modules": true, "build": true,
		"dist": true, ".dart_tool": true,
	}
	var entries []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		name := d.Name()
		// Skip all hidden dot-files and dot-dirs (e.g. .git, .env, .idea)
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDirs[name] {
				return filepath.SkipDir
			}
			// Include the directory itself so users can type @internal/ to see it
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			entries = append(entries, filepath.ToSlash(rel)+"/")
			return nil
		}
		if len(entries) >= limit {
			return filepath.SkipAll
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		entries = append(entries, filepath.ToSlash(rel))
		return nil
	})
	return entries
}

func limitSuggestions(items []suggestion, limit int) []suggestion {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
