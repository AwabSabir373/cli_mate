package ui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"

	"cli_mate/internal/config"
	"cli_mate/internal/providers/registry"
	"cli_mate/internal/usercommands"
	"cli_mate/pkg/crypto"
)

// Command handlers mutate setup state and user settings. Chat execution stays
// in chat.go so provider calls are isolated from command parsing.
func (a *App) runCommand(raw string) {
	fields := strings.Fields(raw)
	command := strings.TrimPrefix(fields[0], "/")
	args := fields[1:]

	// Check for user-defined commands first
	argsStr := strings.Join(args, " ")
	if expanded := a.tryUserCommand(command, argsStr); expanded != "" {
		a.setUserInput(expanded)
		return
	}

	switch command {
	case "":
		a.setInput("/")
		a.appendLog("system", "Choose a command from the list, then press Tab or Enter.")
	case "help":
		a.appendLog("system", strings.Join(commandHelp(), "\n"))
	case "clear":
		a.log = nil
	case "status":
		a.appendLog("system", a.status())
	case "open":
		a.openFile(args)
	case "provider":
		a.setProvider(args)
	case "model":
		a.setModel(args)
	case "theme":
		a.setTheme(args)
	case "permissions":
		a.toggleAutoApprove()
	case "undo":
		a.undoLastEdit()
	case "copy":
		a.copyLastResponse()
	case "compact":
		a.compactConversation()
	case "setup":
		// Launch onboarding wizard
		if a.onboarding != nil {
			a.onboarding.start()
			a.appendLog("system", "Starting setup wizard. Follow the prompts to configure your AI provider.")
		}
	case "resume":
		// Open session picker
		if a.sessionPicker != nil {
			a.sessionPicker.show()
			if a.store != nil {
				ctx := context.Background()
				a.sessionPicker.loadSessions(ctx, a.store)
			}
			a.appendLog("system", "Select a session to resume.")
		}
	case "mcp":
		// Open MCP manager
		if a.mcpManager != nil {
			a.mcpManager.loadFromConfig(a.cfg)
			a.mcpManager.show()
			a.appendLog("system", "MCP server manager opened.")
		}
	case "prs":
		// Open PR status display
		if a.prStatus != nil {
			a.prStatus.show()
			a.appendLog("system", "Pull request status. Navigate with ↑/↓, Esc to close.")
		}
	case "pr":
		// Alias for prs
		if a.prStatus != nil {
			a.prStatus.show()
			a.appendLog("system", "Pull request status. Navigate with ↑/↓, Esc to close.")
		}
	case "doctor":
		// Open diagnostics panel
		if a.doctor != nil {
			a.doctor.show(a.cfg)
			a.appendLog("system", "Running system diagnostics...")
		}
	case "attach":
		// Open image attachment panel
		if a.imageAttach != nil {
			if len(args) >= 1 {
				path := args[0]
				if err := a.imageAttach.addAttachment(path); err != nil {
					a.appendLog("error", fmt.Sprintf("Could not attach image: %v", err))
				} else {
					a.imageAttach.show()
					a.appendLog("system", fmt.Sprintf("Image attached: %s", path))
				}
			} else {
				a.imageAttach.show()
				a.appendLog("system", "Use /attach <path> to add an image, or Esc to close.")
			}
		}
	case "session":
		// Open session controls
		if a.sessionCtrls != nil {
			a.sessionCtrls.show()
			a.appendLog("system", "Session controls opened.")
		}
	case "rename":
		// Rename current session
		if len(args) >= 1 {
			a.handleSessionControlAction("rename:" + strings.Join(args, " "))
		} else if a.sessionCtrls != nil {
			a.sessionCtrls.show()
			a.sessionCtrls.action = actionRename
		}
	default:
		a.appendLog("error", fmt.Sprintf("Unknown command /%s. Type /help.", command))
	}
}

func (a *App) openFile(args []string) {
	if len(args) == 0 {
		a.appendLog("error", "Usage: /open <path> or mention a file with @")
		return
	}

	path := strings.TrimPrefix(args[0], "@")
	content, err := os.ReadFile(path)
	if err != nil {
		a.appendLog("error", err.Error())
		return
	}

	text := string(content)
	if len(text) > 1200 {
		text = text[:1200] + "\n..."
	}
	a.appendLog("file", path+"\n"+text)
}

func (a *App) setProvider(args []string) {
	if len(args) == 0 {
		a.appendLog("system", "Choose one provider with /provider, then press Tab or Enter.")
		return
	}

	name := strings.ToLower(args[0])
	spec, ok := registry.SpecByName(name)
	if !ok {
		a.appendLog("error", fmt.Sprintf("Unknown provider %q. Type /provider to choose one.", name))
		return
	}

	_ = a.cfg.UpdateActive(func(profile *config.Profile) {
		changed := profile.Provider != spec.Name
		profile.Provider = spec.Name
		if changed && spec.RequiresKey {
			profile.APIKey = ""
		}
		if changed || profile.Model == "" {
			profile.Model = spec.DefaultModel
		}
		if profile.BaseURL == "" {
			profile.BaseURL = spec.DefaultBaseURL
		}
	})
	a.saveSettings()
	a.disconnect()

	a.appendLog("system", fmt.Sprintf("Provider set to %s.", spec.Name))
	profile := a.activeProfile()

	// Custom provider: base URL is required, API key is optional
	if spec.Name == "custom" && profile.BaseURL == "" {
		if a.onboarding != nil {
			a.onboarding.start()
		}
		a.appendLog("system", "Custom providers require guided setup. Enter the endpoint in the setup wizard.")
		return
	}

	if spec.RequiresKey && profile.APIKey == "" {
		a.setInput("")
		a.inputMode = "api_key"
		a.appendLog("system", fmt.Sprintf("Paste your %s API key, then press Enter. It is saved in your config.", spec.Name))
		return
	}

	a.setInput("/model ")
	a.inputMode = ""
	a.appendLog("system", fmt.Sprintf("Choose a %s model, then press Tab or Enter.", spec.Name))
}

func (a *App) setModel(args []string) {
	if len(args) == 0 {
		a.setInput("/model ")
		a.appendLog("system", "Choose a model from the selector, or type a custom model id.")
		return
	}

	model := strings.Join(args, " ")
	_ = a.cfg.UpdateActive(func(profile *config.Profile) {
		profile.Model = model
	})
	a.saveSettings()
	a.disconnect()

	profile := a.activeProfile()
	spec, ok := registry.SpecByName(profile.Provider)
	if ok && spec.RequiresKey && profile.APIKey == "" {
		a.setInput("")
		a.inputMode = "api_key"
		a.appendLog("system", fmt.Sprintf("Model set to %s. Paste your %s API key, then press Enter.", model, spec.Name))
		return
	}

	a.appendLog("system", "Model set to "+model+". Send a message when ready.")
}

func (a *App) saveCustomModel(text string) {
	args := strings.Fields(strings.TrimPrefix(text, "/model"))
	if len(args) == 0 {
		a.input = ""
		a.appendLog("error", "Type a custom model id.")
		return
	}

	a.inputMode = ""
	a.setModel(args)
}

func (a *App) setTheme(args []string) {
	if len(args) == 0 {
		a.input = "/theme "
		a.appendLog("system", "Choose a theme from the selector.")
		return
	}

	theme := strings.ToLower(args[0])
	if !validTheme(theme) {
		a.appendLog("error", fmt.Sprintf("Unknown theme %q. Use /theme.", theme))
		return
	}
	a.theme = theme
	a.styles = buildStyles(themeFor(theme))
	a.appendLog("system", "Theme set to "+theme+".")
}

func (a *App) toggleAutoApprove() {
	if a.cfg == nil {
		a.appendLog("error", "No configuration loaded.")
		return
	}
	_ = a.cfg.UpdateActive(func(profile *config.Profile) {
		profile.AutoApprove = !profile.AutoApprove
	})
	profile := a.activeProfile()
	if profile.AutoApprove {
		a.appendLog("system", "Auto-approve enabled. Tools will run without confirmation.")
	} else {
		a.appendLog("system", "Auto-approve disabled. Destructive tools will require confirmation.")
	}
}

func (a *App) saveAPIKey(value string) {
	_ = a.cfg.UpdateActive(func(profile *config.Profile) {
		profile.APIKey = value
	})
	a.saveSettings()
	a.disconnect()
	a.inputMode = ""

	// Clear the raw key from the input buffer immediately.
	a.input = ""
	a.cursorPos = 0

	// Zero the key material from the local variable.
	crypto.ZeroString(&value)

	profile := a.activeProfile()
	spec, _ := registry.SpecByName(profile.Provider)
	if profile.Model == spec.DefaultModel || profile.Model == "" {
		a.input = "/model "
		a.appendLog("system", "API key saved. Choose a model, then press Tab or Enter.")
	} else {
		a.input = ""
		a.appendLog("system", "API key saved. Send a message when ready.")
	}
}

func (a App) status() string {
	profile := a.activeProfile()
	permissions := "ask"
	if profile.AutoApprove {
		permissions = "auto"
	}
	return fmt.Sprintf("provider=%s model=%s workspace=%s permissions=%s", fallback(profile.Provider, "not set"), fallback(profile.Model, "not set"), fallback(a.workspaceRoot, "not set"), permissions)
}

func (a *App) copyLastResponse() {
	for i := len(a.log) - 1; i >= 0; i-- {
		if a.log[i].Kind == "assistant" {
			if err := clipboard.WriteAll(a.log[i].Text); err != nil {
				a.appendLog("error", fmt.Sprintf("Copy failed: %s", err))
				return
			}
			a.appendLog("system", "Copied last response to clipboard.")
			return
		}
	}
	a.appendLog("system", "No AI response to copy.")
}

func (a *App) undoLastEdit() {
	if len(a.editHistory) == 0 {
		a.appendLog("system", "Nothing to undo.")
		return
	}
	last := a.editHistory[len(a.editHistory)-1]
	a.editHistory = a.editHistory[:len(a.editHistory)-1]
	if !last.existed {
		if err := os.Remove(last.path); err != nil && !os.IsNotExist(err) {
			a.appendLog("error", fmt.Sprintf("Undo failed: %s", err))
			return
		}
	} else {
		if err := os.WriteFile(last.path, []byte(last.content), 0o600); err != nil {
			a.appendLog("error", fmt.Sprintf("Undo failed: %s", err))
			return
		}
	}
	a.appendLog("system", fmt.Sprintf("Undid last edit to %s", last.path))
}

func (a *App) compactConversation() {
	if len(a.messages) == 0 {
		a.appendLog("system", "No conversation to compact.")
		return
	}
	a.appendLog("system", "Compacting conversation... summarizing older messages.")
	// Compaction happens automatically during the next agent turn via the
	// proactive compaction in the tool loop. This command marks that the user
	// wants to force compaction now by clearing the low water mark.
	a.compactPending = true
}

func commandHelp() []string {
	return []string{
		"/open <path>            preview a file in the terminal",
		"/copy                   copy last AI response to clipboard",
		"/undo                   undo the last file_edit",
		"/setup                  run the interactive setup wizard",
		"/resume                 resume a previous session",
		"/mcp                    manage MCP servers",
		"/prs                    show pull request status",
		"/doctor                 run system diagnostics",
		"/attach <path>          attach an image to the conversation",
		"/session                open session controls (rename/export/rewind)",
		"/rename <name>          rename the current session",
		"/provider               choose one active provider",
		"/model                  choose model for active provider",
		"/theme                  choose terminal theme",
		"/permissions            toggle tool auto-approval",
		"/status                 show active provider configuration",
		"/clear                  clear the console",
		"/compact                summarize older messages to save context",
		"/exit                   quit cli_mate",
		"",
		"Keyboard shortcuts:",
		"  Up/Down       cycle through prompt history",
		"  Alt+Up/Down   scroll console",
		"  Ctrl+P        fuzzy file finder",
		"  Ctrl+C        quit",
	}
}

func providerList() string {
	var rows []string
	for _, spec := range registry.Specs() {
		key := "no key"
		if spec.RequiresKey {
			key = "api key"
		}
		rows = append(rows, fmt.Sprintf("%s  default=%s  auth=%s", spec.Name, spec.DefaultModel, key))
	}
	return strings.Join(rows, "\n")
}

func (a *App) matchingCommands(prefix string) []string {
	commands := []string{"/help", "/open", "/copy", "/undo", "/setup", "/resume", "/mcp", "/prs", "/doctor", "/attach", "/session", "/rename", "/provider", "/model", "/theme", "/permissions", "/status", "/clear", "/compact", "/exit"}
	// Add user commands to autocomplete
	for _, cmd := range a.userCommands {
		commands = append(commands, "/"+cmd.Name)
	}
	var matches []string
	for _, command := range commands {
		if strings.HasPrefix(strings.TrimPrefix(command, "/"), prefix) {
			matches = append(matches, command)
		}
	}
	return matches
}

// tryUserCommand checks if the command name matches a user-defined command
// and returns the expanded template, or empty string if not found.
func (a *App) tryUserCommand(name string, args string) string {
	for _, cmd := range a.userCommands {
		if cmd.Name == name {
			return usercommands.Expand(cmd.Template, args)
		}
	}
	return ""
}

// setUserInput sets the input field to the given text, ready to be submitted.
func (a *App) setUserInput(text string) {
	a.input = text
	a.cursorPos = len(text)
	a.appendLog("system", "Custom command expanded. Press Enter to submit.")
}
