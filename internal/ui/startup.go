package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cli_mate/internal/providers/registry"
)

// startPhase represents a phase of the startup process.
type startPhase int

const (
	phaseWelcome startPhase = iota
	phaseWorkspace
	phaseConfig
	phaseProvider
	phaseComplete
)

// startupState manages the first-run and workspace initialization flow.
type startupState struct {
	visible    bool
	phase      startPhase
	workspace  string
	configPath string
	hasConfig  bool
	hasProfile bool
	cursor     int
	err        string
	isFirstRun bool
}

// newStartupState creates a new startup state.
func newStartupState() *startupState {
	return &startupState{}
}

// show opens the startup flow.
func (ss *startupState) show() {
	ss.visible = true
	ss.phase = phaseWelcome
	ss.cursor = 0
	ss.err = ""

	// Detect workspace
	cwd, _ := os.Getwd()
	ss.workspace = filepath.Base(cwd)

	// Check for existing config
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "cli_mate", "cli_mate.yaml")
	if _, err := os.Stat(configPath); err == nil {
		ss.hasConfig = true
		ss.configPath = configPath
	} else {
		ss.hasConfig = false
	}
}

// hide closes the startup flow.
func (ss *startupState) hide() {
	ss.visible = false
}

// isVisible returns true if the startup flow is active.
func (ss *startupState) isVisible() bool {
	return ss.visible
}

// isComplete returns true if startup has finished.
func (ss *startupState) isComplete() bool {
	return ss.phase == phaseComplete
}

// handleKey processes a keypress and returns (shouldClose bool).
func (ss *startupState) handleKey(key string) bool {
	if !ss.visible {
		return false
	}

	switch ss.phase {
	case phaseWelcome:
		return ss.handleWelcomeKey(key)
	case phaseWorkspace:
		return ss.handleWorkspaceKey(key)
	case phaseConfig:
		return ss.handleConfigKey(key)
	case phaseProvider:
		return ss.handleProviderKey(key)
	case phaseComplete:
		if key == "enter" || key == " " || key == "esc" {
			ss.hide()
			return true
		}
	}

	return false
}

func (ss *startupState) handleWelcomeKey(key string) bool {
	switch key {
	case "enter", " ":
		ss.phase = phaseWorkspace
		ss.cursor = 0
	case "esc":
		ss.hide()
		return true
	}
	return false
}

func (ss *startupState) handleWorkspaceKey(key string) bool {
	switch key {
	case "enter", " ":
		ss.phase = phaseConfig
		ss.cursor = 0
	case "esc":
		ss.phase = phaseWelcome
	}
	return false
}

func (ss *startupState) handleConfigKey(key string) bool {
	switch key {
	case "up", "shift+tab":
		if ss.cursor > 0 {
			ss.cursor--
		}
	case "down", "tab":
		if ss.cursor < 1 {
			ss.cursor++
		}
	case "enter", " ":
		if ss.cursor == 0 {
			// Continue with existing config
			ss.phase = phaseComplete
		} else {
			// Start fresh
			ss.phase = phaseProvider
			ss.cursor = 0
		}
	case "esc":
		ss.phase = phaseWorkspace
	}
	return false
}

func (ss *startupState) handleProviderKey(key string) bool {
	providers := registry.Specs()
	switch key {
	case "up", "shift+tab":
		if ss.cursor > 0 {
			ss.cursor--
		}
	case "down", "tab":
		if ss.cursor < len(providers)-1 {
			ss.cursor++
		}
	case "enter", " ":
		if ss.cursor >= 0 && ss.cursor < len(providers) {
			ss.phase = phaseComplete
		}
	case "esc":
		ss.phase = phaseConfig
	}
	return false
}

// renderStartup renders the startup flow.
func renderStartup(ss *startupState, styles appStyles, width int) string {
	if !ss.visible {
		return ""
	}

	var b strings.Builder

	switch ss.phase {
	case phaseWelcome:
		ss.renderWelcome(&b, styles)
	case phaseWorkspace:
		ss.renderWorkspace(&b, styles)
	case phaseConfig:
		ss.renderConfig(&b, styles)
	case phaseProvider:
		ss.renderProvider(&b, styles)
	case phaseComplete:
		ss.renderComplete(&b, styles)
	}

	return styles.panel.Width(width - 4).Render(b.String())
}

func (ss *startupState) renderWelcome(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.pill.Render(" Welcome "))
	b.WriteString("\n\n")
	b.WriteString(styles.title.Render(fmt.Sprintf("Welcome to %s", ss.workspace)))
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("cli_mate is a terminal-first AI coding agent."))
	b.WriteString("\n\n")

	b.WriteString(styles.accent.Render("  ✦ "))
	b.WriteString(styles.muted.Render("Code with AI in your terminal"))
	b.WriteString("\n")
	b.WriteString(styles.accent.Render("  ✦ "))
	b.WriteString(styles.muted.Render("Get context-aware suggestions"))
	b.WriteString("\n")
	b.WriteString(styles.accent.Render("  ✦ "))
	b.WriteString(styles.muted.Render("Edit files, run commands, review code"))
	b.WriteString("\n\n")

	b.WriteString(styles.success.Render("  Press Enter to continue"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Press Esc to skip"))
	b.WriteString("\n")
}

func (ss *startupState) renderWorkspace(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.pill.Render(" Workspace "))
	b.WriteString("\n\n")

	b.WriteString(styles.accent.Render(fmt.Sprintf("  Working directory: %s", ss.workspace)))
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("cli_mate will index your files and provide"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("context-aware code assistance for this project."))
	b.WriteString("\n\n")

	if ss.hasConfig {
		b.WriteString(styles.muted.Render("  Existing configuration detected"))
		b.WriteString("\n\n")
	}

	b.WriteString(styles.success.Render("  Press Enter to continue"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Press Esc to go back"))
	b.WriteString("\n")
}

func (ss *startupState) renderConfig(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.pill.Render(" Configuration "))
	b.WriteString("\n\n")

	if ss.hasConfig {
		b.WriteString(styles.muted.Render("An existing configuration was found:"))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %s", ss.configPath)))
		b.WriteString("\n\n")

		options := []string{"✓ Use existing configuration", "✕ Start fresh"}
		for i, opt := range options {
			if i == ss.cursor {
				b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", opt)))
				b.WriteString("\n")
			} else {
				b.WriteString(fmt.Sprintf("   %s", opt))
				b.WriteString("\n")
			}
		}
	} else {
		b.WriteString(styles.muted.Render("No configuration found. Let's set up your AI provider."))
		b.WriteString("\n\n")
		b.WriteString(styles.success.Render("  Press Enter to continue"))
	}
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
}

func (ss *startupState) renderProvider(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.pill.Render(" Choose Provider "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Select your AI provider:"))
	b.WriteString("\n\n")

	specs := registry.Specs()
	for i, spec := range specs {
		auth := "local"
		if spec.RequiresKey {
			auth = "api key required"
		}
		desc := fmt.Sprintf("model: %s · %s", spec.DefaultModel, auth)
		if i == ss.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", spec.Name)))
			b.WriteString("\n")
			b.WriteString(styles.muted.Render(fmt.Sprintf("   %s", desc)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", spec.Name))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
}

func (ss *startupState) renderComplete(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.pill.Render(" Ready "))
	b.WriteString("\n\n")
	b.WriteString(styles.success.Render("✓ Setup complete!"))
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("  Type /help to see available commands"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Type /setup to configure your provider"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Start typing to chat with the AI agent"))
	b.WriteString("\n\n")

	b.WriteString(styles.accent.Render("  Press Enter to start"))
	b.WriteString("\n")
}

// detectWorkspaceConfig checks if the workspace has cli_mate-specific config.
func detectWorkspaceConfig(root string) (hasAGENTS, hasMakefile bool) {
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err == nil {
		hasAGENTS = true
	}
	makefilePath := filepath.Join(root, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		hasMakefile = true
	}
	return
}
