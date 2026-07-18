package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"cli_mate/internal/providers/registry"
)

// Navigation and submission live together because Enter, Tab, and Esc share
// the same suggestion and setup-state rules.
func (a *App) back() {
	a.selected = 0

	if a.inputMode == "finder" {
		a.inputMode = ""
		a.input = ""
		a.cursorPos = 0
		return
	}

	if a.inputMode == "api_key" {
		a.inputMode = ""
		a.input = "/provider "
		a.cursorPos = len(a.input)
		a.appendLog("system", "Back to provider selection.")
		return
	}
	if a.inputMode == "custom_model" {
		a.inputMode = ""
		a.input = "/model "
		a.cursorPos = len(a.input)
		a.appendLog("system", "Back to model selection.")
		return
	}

	input := strings.TrimSpace(a.input)
	switch {
	case isModelInput(input):
		profile := a.activeProfile()
		spec, ok := registry.SpecByName(profile.Provider)
		if ok && spec.RequiresKey {
			a.inputMode = "api_key"
			a.input = ""
			a.cursorPos = 0
			a.appendLog("system", "Back to API key input.")
			return
		}
		a.input = "/provider "
		a.cursorPos = len(a.input)
		a.appendLog("system", "Back to provider selection.")
	case isThemeInput(input), isProviderInput(input), input == "/":
		a.input = ""
		a.cursorPos = 0
	case input != "":
		a.input = ""
		a.cursorPos = 0
	default:
		a.appendLog("system", "Press Ctrl+C to quit.")
	}
}

func (a *App) submit() tea.Cmd {
	text := strings.TrimSpace(a.input)
	a.input = ""
	a.cursorPos = 0
	if text == "" {
		return nil
	}

	if a.inputMode == "finder" {
		a.openFinderSelection()
		return nil
	}

	if a.inputMode == "api_key" {
		a.saveAPIKey(text)
		return nil
	}
	if a.inputMode == "custom_model" {
		a.saveCustomModel(text)
		return nil
	}

	if strings.HasPrefix(text, "/") {
		if text == "/exit" || text == "/quit" {
			return tea.Quit
		}
		a.runCommand(text)
		return nil
	}

	// Shell escape: !command runs outside the agent
	if strings.HasPrefix(text, "!") {
		cmdText := strings.TrimSpace(text[1:])
		if cmdText == "" {
			a.appendLog("system", "Usage: !<shell command>")
			return nil
		}
		a.appendLog("system", "$ "+cmdText)
		return runBashEscape(a.workspaceRoot, cmdText)
	}

	if a.loading {
		a.appendLog("error", "Wait for the current response to finish before sending another message.")
		return nil
	}

	// Check for MCP setup intent
	if intent, ok := detectMCPSetupIntent(text); ok {
		a.appendLog("system", "Detected MCP setup request: "+intent.ServerName)
		if intent.Endpoint != "" {
			a.appendLog("system", "Endpoint: "+intent.Endpoint)
		}
		if a.mcpManager != nil {
			a.mcpManager.show()
		}
		return nil
	}

	// Save to history
	a.history = append(a.history, text)
	if len(a.history) > 50 {
		a.history = a.history[1:]
	}
	a.historyIndex = len(a.history)

	a.appendLog("user", text)
	return a.startChat(text)
}

// navigateHistory loads previous/next prompt from command history.
func (a *App) navigateHistory(delta int) {
	// If suggestions are active, navigate suggestions instead of history
	if len(a.currentSuggestions()) > 0 {
		a.moveSelection(delta)
		return
	}
	// In finder mode, navigate file matches
	if a.inputMode == "finder" {
		a.selected = clamp(a.selected+delta, 0, 19)
		return
	}

	if len(a.history) == 0 {
		return
	}
	newIndex := a.historyIndex + delta
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex >= len(a.history) {
		newIndex = len(a.history)
	}
	a.historyIndex = newIndex

	if a.historyIndex >= len(a.history) {
		// Past the end — clear input
		a.input = ""
	} else {
		a.input = a.history[a.historyIndex]
	}
	a.cursorPos = len(a.input)
	a.selected = 0
}

func (a *App) openFinderSelection() {
	query := strings.ToLower(a.input)
	var matches []string
	for _, f := range a.files {
		if query == "" || strings.Contains(strings.ToLower(f), query) {
			matches = append(matches, f)
		}
		if len(matches) >= 20 {
			break
		}
	}
	if len(matches) == 0 {
		a.inputMode = ""
		return
	}
	idx := a.selected % len(matches)
	selected := matches[idx]
	a.inputMode = ""
	a.input = ""
	a.cursorPos = 0
	a.selected = 0
	// Open the selected file by inserting as @mention
	a.input = "@" + selected + " "
	a.cursorPos = len(a.input)
}

// insertText inserts text at the current cursor position.
func (a *App) insertText(text string) {
	if a.cursorPos >= len(a.input) {
		a.input += text
		a.cursorPos = len(a.input)
	} else {
		a.input = a.input[:a.cursorPos] + text + a.input[a.cursorPos:]
		a.cursorPos += len(text)
	}
}

// insertChar inserts a single character at the cursor position.
func (a *App) insertChar(ch rune) {
	a.insertText(string(ch))
}

// deleteCharBackward deletes the character before the cursor (Backspace).
func (a *App) deleteCharBackward() {
	if a.cursorPos == 0 {
		return
	}
	a.input = a.input[:a.cursorPos-1] + a.input[a.cursorPos:]
	a.cursorPos--
}

// deleteCharForward deletes the character after the cursor (Delete).
func (a *App) deleteCharForward() {
	if a.cursorPos >= len(a.input) {
		return
	}
	a.input = a.input[:a.cursorPos] + a.input[a.cursorPos+1:]
}

// moveCursorLeft moves cursor one position left.
func (a *App) moveCursorLeft() {
	if a.cursorPos > 0 {
		a.cursorPos--
	}
}

// moveCursorRight moves cursor one position right.
func (a *App) moveCursorRight() {
	if a.cursorPos < len(a.input) {
		a.cursorPos++
	}
}

// deleteWordBackward deletes the word before the cursor (Alt+Backspace / Ctrl+W).
func (a *App) deleteWordBackward() {
	if a.cursorPos == 0 {
		return
	}
	pos := a.cursorPos - 1
	// Skip any trailing spaces
	for pos > 0 && a.input[pos] == ' ' {
		pos--
	}
	// Skip word characters
	for pos > 0 && a.input[pos-1] != ' ' {
		pos--
	}
	a.input = a.input[:pos] + a.input[a.cursorPos:]
	a.cursorPos = pos
}

// deleteToLineStart deletes from cursor to the beginning of the line.
func (a *App) deleteToLineStart() {
	if a.cursorPos == 0 {
		return
	}
	a.input = a.input[a.cursorPos:]
	a.cursorPos = 0
}

// deleteToLineEnd deletes from cursor to the end of the line.
func (a *App) deleteToLineEnd() {
	a.input = a.input[:a.cursorPos]
}

// pasteFromClipboard reads the system clipboard and inserts at cursor.
func (a *App) pasteFromClipboard() {
	text, err := clipboard.ReadAll()
	if err != nil {
		return
	}
	// Normalize line endings and trim trailing newlines from paste
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n\r")
	a.insertText(text)
}

func (a *App) moveSelection(delta int) {
	items := a.currentSuggestions()
	if len(items) == 0 {
		a.selected = 0
		return
	}
	a.selected = (a.selected + delta + len(items)) % len(items)
}

func (a *App) acceptSuggestion() {
	items := a.currentSuggestions()
	if len(items) == 0 {
		return
	}

	item := items[clamp(a.selected, 0, len(items)-1)]
	if strings.HasPrefix(strings.TrimSpace(a.input), "/") {
		a.input = item.Value
		if strings.HasPrefix(item.Value, "/") && !strings.HasSuffix(item.Value, " ") {
			a.input += " "
		}
		return
	}

	token, ok := activeMentionToken(a.input)
	if !ok {
		return
	}
	a.input = strings.TrimSuffix(a.input, token) + "@" + item.Value + " "
}

func (a *App) acceptSelectionOrSubmit() bool {
	items := a.currentSuggestions()
	if len(items) == 0 {
		return false
	}

	before := strings.TrimSpace(a.input)
	item := items[clamp(a.selected, 0, len(items)-1)]

	if isProviderInput(before) || isModelInput(before) || isThemeInput(before) {
		return a.commitSetupChoice(item)
	}

	if strings.HasPrefix(before, "/") {
		a.acceptSuggestion()
		if strings.HasSuffix(item.Value, " ") {
			a.selected = 0
			return true
		}
		_ = a.submit()
		return true
	}

	a.acceptSuggestion()
	return true
}

func (a *App) commitSetupChoice(item suggestion) bool {
	fields := strings.Fields(item.Value)
	if len(fields) < 2 {
		a.acceptSuggestion()
		return true
	}

	command := strings.TrimPrefix(fields[0], "/")
	args := fields[1:]

	switch command {
	case "provider":
		a.setProvider(args)
	case "model":
		if isCustomModelChoice(args) {
			a.inputMode = "custom_model"
			a.input = ""
			a.appendLog("system", "Type your custom model id, then press Enter.")
			return true
		}
		a.setModel(args)
	case "theme":
		a.setTheme(args)
	default:
		a.acceptSuggestion()
	}
	a.selected = 0
	return true
}
