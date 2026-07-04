package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cli_mate/internal/providers/registry"
	"cli_mate/internal/usercommands"
)

// suggestionCategory groups suggestions by type.
type suggestionCategory string

const (
	catCommand    suggestionCategory = "commands"
	catFile       suggestionCategory = "files"
	catProvider   suggestionCategory = "providers"
	catModel      suggestionCategory = "models"
	catTheme      suggestionCategory = "themes"
	catSpecialist suggestionCategory = "specialists"
	catUserCmd    suggestionCategory = "user-commands"
)

// autocompleteSuggestion extends the base suggestion with a category.
type autocompleteSuggestion struct {
	Value       string
	Label       string
	Description string
	Category    suggestionCategory
	Score       int // higher = better match
}

// autocompleteState manages the full autocomplete system.
type autocompleteState struct {
	mu            sync.RWMutex
	active        bool
	category      suggestionCategory
	suggestions   []autocompleteSuggestion
	cursor        int
	query         string
	fileIndex     *fileSuggestionIndex
	lastActive    time.Time
}

// fileSuggestionIndex caches workspace file listings with a TTL.
type fileSuggestionIndex struct {
	files     []string
	builtAt   time.Time
	ttl       time.Duration
	cwd       string
	mu        sync.RWMutex
}

// newFileSuggestionIndex creates a new file suggestion index.
func newFileSuggestionIndex(ttl time.Duration) *fileSuggestionIndex {
	return &fileSuggestionIndex{
		ttl: ttl,
	}
}

// refresh rebuilds the file index if the cache is stale.
func (idx *fileSuggestionIndex) refresh(cwd string, limit int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if time.Since(idx.builtAt) < idx.ttl && idx.cwd == cwd && len(idx.files) > 0 {
		return
	}

	idx.cwd = cwd
	idx.builtAt = time.Now()
	idx.files = buildFileIndex(cwd, limit)
}

// search performs a fuzzy search against the cached file index.
func (idx *fileSuggestionIndex) search(query string, maxResults int) []autocompleteSuggestion {
	idx.mu.RLock()
	files := idx.files
	idx.mu.RUnlock()

	if len(files) == 0 || query == "" {
		return nil
	}

	query = strings.ToLower(query)
	var results []autocompleteSuggestion

	for _, f := range files {
		lower := strings.ToLower(f)
		score := fuzzyScore(query, lower)
		if score > 0 {
			kind := "file"
			if strings.HasSuffix(f, "/") {
				kind = "dir"
			}
			results = append(results, autocompleteSuggestion{
				Value:       f,
				Label:       "@" + f,
				Description: kind,
				Category:    catFile,
				Score:       score,
			})
		}
	}

	// Sort by score descending, then by label ascending
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Label < results[j].Label
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// buildFileIndex walks the workspace and builds a flat file list.
func buildFileIndex(root string, limit int) []string {
	skipDirs := map[string]bool{
		"vendor": true, "node_modules": true, "build": true,
		"dist": true, ".git": true, ".dart_tool": true,
	}

	var entries []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		name := d.Name()
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
			rel, _ := filepath.Rel(root, path)
			entries = append(entries, filepath.ToSlash(rel)+"/")
			return nil
		}
		if len(entries) >= limit {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(root, path)
		entries = append(entries, filepath.ToSlash(rel))
		return nil
	})
	return entries
}

// fuzzyScore computes a simple fuzzy match score.
// Returns 0 if no match, higher positive integer for better matches.
func fuzzyScore(query, target string) int {
	if query == "" {
		return 0
	}

	qi := 0
	score := 0
	consecutive := 0
	firstMatch := -1

	for ti := 0; ti < len(target) && qi < len(query); ti++ {
		if target[ti] == query[qi] {
			if qi == 0 {
				firstMatch = ti
				if ti == 0 {
					consecutive = 2 // bonus for prefix match
				}
			}
			consecutive++
			score += consecutive * 10
			// Bonus for word boundaries
			if ti > 0 && (target[ti-1] == '/' || target[ti-1] == '_' || target[ti-1] == '-' || target[ti-1] == '.') {
				score += 15
			}
			qi++
		} else {
			consecutive = 0
		}
	}

	if qi < len(query) {
		return 0 // not all query chars matched
	}

	// Bonus for matching at word boundaries
	if firstMatch == 0 {
		score += 20
	}

	// Bonus for shorter matches (closer to the start)
	score += (len(target) - firstMatch) * 2

	return score
}

// newAutocompleteState creates a new autocomplete state.
func newAutocompleteState() *autocompleteState {
	return &autocompleteState{
		fileIndex: newFileSuggestionIndex(30 * time.Second),
	}
}

// isActive returns true if the autocomplete overlay is visible.
func (ac *autocompleteState) isActive() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.active && len(ac.suggestions) > 0
}

// open opens the autocomplete with suggestions for the given query context.
func (ac *autocompleteState) open(input string, cwd string, userCmds []usercommands.Command) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.query = input
	ac.active = true
	ac.cursor = 0
	ac.lastActive = time.Now()

	// Refresh file index
	if cwd != "" {
		ac.fileIndex.refresh(cwd, 2000)
	}

	ac.suggestions = ac.computeSuggestions(input, userCmds)
}

// close closes the autocomplete overlay.
func (ac *autocompleteState) close() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.active = false
	ac.suggestions = nil
	ac.cursor = 0
}

// move moves the cursor by delta and returns the selected suggestion value.
func (ac *autocompleteState) move(delta int) string {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if !ac.active || len(ac.suggestions) == 0 {
		return ""
	}

	ac.cursor += delta
	if ac.cursor < 0 {
		ac.cursor = 0
	}
	if ac.cursor >= len(ac.suggestions) {
		ac.cursor = len(ac.suggestions) - 1
	}

	return ac.suggestions[ac.cursor].Value
}

// selected returns the currently selected suggestion value.
func (ac *autocompleteState) selected() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	if !ac.active || ac.cursor < 0 || ac.cursor >= len(ac.suggestions) {
		return ""
	}
	return ac.suggestions[ac.cursor].Value
}

// computeSuggestions builds the suggestion list based on input context.
func (ac *autocompleteState) computeSuggestions(input string, userCmds []usercommands.Command) []autocompleteSuggestion {
	trimmed := strings.TrimSpace(input)

	// Command mode: input starts with /
	if strings.HasPrefix(trimmed, "/") {
		prefix := strings.TrimPrefix(trimmed, "/")
		if !strings.Contains(prefix, " ") {
			return ac.commandSuggestions(prefix, userCmds)
		}
		// After command prefix + space, check for sub-arguments
		parts := strings.SplitN(prefix, " ", 2)
		cmdName := parts[0]
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}
		_ = arg

		switch cmdName {
		case "provider":
			return ac.providerSuggestions(arg)
		case "model":
			return ac.modelSuggestions(arg)
		case "theme":
			return ac.themeSuggestions(arg)
		}
		return nil
	}

	// File mention mode: last token starts with @
	fields := strings.Fields(trimmed)
	if len(fields) > 0 {
		last := fields[len(fields)-1]
		if strings.HasPrefix(last, "@") {
			query := strings.TrimPrefix(last, "@")
			return ac.fileIndex.search(query, 15)
		}
	}

	return nil
}

// commandSuggestions returns matching command suggestions.
func (ac *autocompleteState) commandSuggestions(prefix string, userCmds []usercommands.Command) []autocompleteSuggestion {
	allCommands := []struct {
		value, label, desc string
		cat                 suggestionCategory
	}{
		{"/help", "/help", "show available commands", catCommand},
		{"/open ", "/open", "preview a file", catCommand},
		{"/setup", "/setup", "run the interactive setup wizard", catCommand},
		{"/resume", "/resume", "resume a previous session", catCommand},
		{"/mcp", "/mcp", "manage MCP servers", catCommand},
		{"/prs", "/prs", "show pull request status", catCommand},
		{"/spec", "/spec", "start specification-driven development", catCommand},
		{"/provider ", "/provider", "choose AI provider", catCommand},
		{"/model ", "/model", "choose model", catCommand},
		{"/theme ", "/theme", "choose terminal theme", catCommand},
		{"/api-key ", "/api-key", "set API key", catCommand},
		{"/max-tokens ", "/max-tokens", "set context limit", catCommand},
		{"/base-url ", "/base-url", "set provider URL", catCommand},
		{"/connect", "/connect", "validate and connect", catCommand},
		{"/approve", "/approve", "toggle auto-approve", catCommand},
		{"/status", "/status", "show configuration", catCommand},
		{"/copy", "/copy", "copy last response", catCommand},
		{"/clear", "/clear", "clear console", catCommand},
		{"/review", "/review", "review code changes", catCommand},
		{"/diff ", "/diff", "show git diff", catCommand},
		{"/commit ", "/commit", "create a git commit", catCommand},
		{"/compact", "/compact", "summarize older messages", catCommand},
		{"/undo", "/undo", "undo last file edit", catCommand},
		{"/skills", "/skills", "list available skills", catCommand},
		{"/update", "/update", "check for updates", catCommand},
		{"/style ", "/style", "set response style", catCommand},
	}

	var results []autocompleteSuggestion

	for _, cmd := range allCommands {
		label := strings.TrimPrefix(cmd.label, "/")
		if prefix == "" || strings.HasPrefix(label, prefix) || fuzzyScore(prefix, label) > 0 {
			results = append(results, autocompleteSuggestion{
				Value:       cmd.value,
				Label:       cmd.label,
				Description: cmd.desc,
				Category:    cmd.cat,
				Score:       fuzzyScore(prefix, label),
			})
		}
	}

	// Add user commands
	for _, cmd := range userCmds {
		if prefix == "" || strings.HasPrefix(cmd.Name, prefix) {
			desc := cmd.Description
			if desc == "" {
				desc = "custom command"
			}
			results = append(results, autocompleteSuggestion{
				Value:       "/" + cmd.Name + " ",
				Label:       "/" + cmd.Name,
				Description: desc,
				Category:    catUserCmd,
				Score:       fuzzyScore(prefix, cmd.Name),
			})
		}
	}

	// Sort: prefix matches first, then fuzzy score
	sort.Slice(results, func(i, j int) bool {
		iPrefix := strings.HasPrefix(strings.TrimPrefix(results[i].Label, "/"), prefix)
		jPrefix := strings.HasPrefix(strings.TrimPrefix(results[j].Label, "/"), prefix)
		if iPrefix != jPrefix {
			return iPrefix
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > 12 {
		results = results[:12]
	}

	return results
}

// providerSuggestions returns matching provider suggestions.
func (ac *autocompleteState) providerSuggestions(prefix string) []autocompleteSuggestion {
	prefix = strings.ToLower(prefix)
	var results []autocompleteSuggestion

	for _, spec := range registry.Specs() {
		if prefix != "" && !strings.HasPrefix(spec.Name, prefix) {
			continue
		}
		auth := "local"
		if spec.RequiresKey {
			auth = "api key required"
		}
		results = append(results, autocompleteSuggestion{
			Value:       "/provider " + spec.Name,
			Label:       spec.Name,
			Description: fmt.Sprintf("model %s, %s", spec.DefaultModel, auth),
			Category:    catProvider,
			Score:       fuzzyScore(prefix, spec.Name),
		})
	}

	return results
}

// modelSuggestions returns matching model suggestions.
func (ac *autocompleteState) modelSuggestions(prefix string) []autocompleteSuggestion {
	prefix = strings.ToLower(prefix)
	models := []string{"gpt-4.1-mini", "gpt-4.1", "gpt-4o", "claude-sonnet-4-20250514", "gemini-2.5-flash", "llama-3.3-70b-versatile", "llama3.1"}

	var results []autocompleteSuggestion
	for _, model := range models {
		if prefix != "" && !strings.Contains(strings.ToLower(model), prefix) && fuzzyScore(prefix, model) == 0 {
			continue
		}
		results = append(results, autocompleteSuggestion{
			Value:       "/model " + model,
			Label:       model,
			Description: "AI model",
			Category:    catModel,
			Score:       fuzzyScore(prefix, model),
		})
	}

	if prefix == "" || strings.HasPrefix("custom", prefix) {
		results = append(results, autocompleteSuggestion{
			Value:       "/model custom",
			Label:       "custom",
			Description: "type a custom model id",
			Category:    catModel,
		})
	}

	return results
}

// themeSuggestions returns matching theme suggestions.
func (ac *autocompleteState) themeSuggestions(prefix string) []autocompleteSuggestion {
	themes := []string{"midnight", "matrix", "paper", "mono", "catppuccin", "dracula", "nord", "gruvbox", "tokyonight", "rosepine", "solarized", "onedark"}
	var results []autocompleteSuggestion

	for _, theme := range themes {
		if prefix != "" && !strings.HasPrefix(theme, strings.ToLower(prefix)) {
			continue
		}
		results = append(results, autocompleteSuggestion{
			Value:       "/theme " + theme,
			Label:       theme,
			Description: "terminal theme",
			Category:    catTheme,
		})
	}

	return results
}

// renderAutocomplete renders the autocomplete overlay.
func renderAutocomplete(ac *autocompleteState, styles appStyles, width int) string {
	ac.mu.RLock()
	if !ac.active || len(ac.suggestions) == 0 {
		ac.mu.RUnlock()
		return ""
	}
	suggestions := make([]autocompleteSuggestion, len(ac.suggestions))
	copy(suggestions, ac.suggestions)
	cursor := ac.cursor
	ac.mu.RUnlock()

	var b strings.Builder

	// Group by category for display
	type categoryGroup struct {
		name  string
		items []autocompleteSuggestion
	}
	groups := make(map[suggestionCategory]*categoryGroup)
	var order []suggestionCategory

	for _, s := range suggestions {
		if _, ok := groups[s.Category]; !ok {
			groups[s.Category] = &categoryGroup{name: string(s.Category)}
			order = append(order, s.Category)
		}
		groups[s.Category].items = append(groups[s.Category].items, s)
	}

	// Show flat list with category badges
	b.WriteString(styles.muted.Render(" Suggestions:"))
	b.WriteString("\n")

	maxVisible := 10
	start := 0
	if cursor > maxVisible/2 {
		start = cursor - maxVisible/2
	}
	if start+maxVisible > len(suggestions) {
		start = len(suggestions) - maxVisible
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < len(suggestions) && i < start+maxVisible; i++ {
		s := suggestions[i]
		catBadge := styles.badge.Render(string(s.Category[:3]))
		label := s.Label
		if len(label) > 35 {
			label = label[:35]
		}
		desc := s.Description
		if desc != "" {
			desc = " " + styles.muted.Render(desc)
		}

		if i == cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s %s%s", catBadge, label, "")))
			b.WriteString(desc)
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s %s%s", catBadge, label, desc))
			b.WriteString("\n")
		}
	}

	if len(suggestions) > maxVisible {
		b.WriteString(styles.muted.Render(fmt.Sprintf("   ... %d more ...", len(suggestions)-maxVisible)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Tab/Enter select · Esc close"))
	b.WriteString("\n")

	return styles.softPanel.Width(width - 6).Render(b.String())
}
