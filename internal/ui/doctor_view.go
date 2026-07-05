package ui

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"cli_mate/internal/config"
	"cli_mate/internal/providers/registry"
)

// checkStatus represents the result of a diagnostic check.
type checkStatus string

const (
	checkPass checkStatus = "pass"
	checkFail checkStatus = "fail"
	checkWarn checkStatus = "warn"
	checkSkip checkStatus = "skip"
)

// diagnosticCheck represents a single diagnostic check.
type diagnosticCheck struct {
	name     string
	status   checkStatus
	message  string
	duration time.Duration
}

// doctorView manages the system diagnostics panel.
type doctorView struct {
	visible  bool
	checks   []diagnosticCheck
	started  bool
	complete bool
	cursor   int
}

// newDoctorView creates a new diagnostic view.
func newDoctorView() *doctorView {
	return &doctorView{}
}

// show opens the diagnostics panel and runs checks.
func (dv *doctorView) show(cfg *config.Config) {
	dv.visible = true
	dv.started = true
	dv.complete = false
	dv.cursor = 0

	dv.runChecks(cfg)
	dv.complete = true
}

// hide closes the diagnostics panel.
func (dv *doctorView) hide() {
	dv.visible = false
	dv.checks = nil
	dv.started = false
	dv.complete = false
}

// isVisible returns true if the panel is visible.
func (dv *doctorView) isVisible() bool {
	return dv.visible
}

// runChecks runs all diagnostic checks.
func (dv *doctorView) runChecks(cfg *config.Config) {
	dv.checks = nil

	// 1. Go runtime check
	dv.addCheck("Go Runtime", func() (checkStatus, string) {
		return checkPass, fmt.Sprintf("Go %s on %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	})

	// 2. Config file check
	dv.addCheck("Configuration", func() (checkStatus, string) {
		if cfg == nil {
			return checkFail, "No configuration loaded"
		}
		if cfg.ActiveProfile == "" {
			return checkWarn, "No active profile set"
		}
		return checkPass, fmt.Sprintf("Active profile: %s", cfg.ActiveProfile)
	})

	// 3. Config writable check
	dv.addCheck("Config File", func() (checkStatus, string) {
		if cfg == nil {
			return checkSkip, "No config available"
		}
		// Determine config path
		home, _ := os.UserHomeDir()
		configDir := home + "/.config/cli_mate"
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			return checkWarn, fmt.Sprintf("Config directory does not exist: %s", configDir)
		}
		return checkPass, fmt.Sprintf("Config directory: %s", configDir)
	})

	// 4. Provider check
	dv.addCheck("Provider", func() (checkStatus, string) {
		if cfg == nil {
			return checkSkip, "No config"
		}
		profile, err := cfg.Active()
		if err != nil {
			return checkFail, fmt.Sprintf("Could not load profile: %v", err)
		}
		if profile.Provider == "" {
			return checkFail, "No provider configured"
		}
		_, ok := registry.SpecByName(profile.Provider)
		if !ok {
			return checkWarn, fmt.Sprintf("Unknown provider: %s", profile.Provider)
		}
		return checkPass, fmt.Sprintf("Provider: %s", profile.Provider)
	})

	// 5. API key check
	dv.addCheck("API Key", func() (checkStatus, string) {
		if cfg == nil {
			return checkSkip, "No config"
		}
		profile, err := cfg.Active()
		if err != nil {
			return checkSkip, ""
		}
		spec, ok := registry.SpecByName(profile.Provider)
		if !ok || !spec.RequiresKey {
			return checkSkip, "Not required for this provider"
		}
		if profile.APIKey == "" {
			return checkFail, "API key not set"
		}
		keyLen := len(profile.APIKey)
		if keyLen < 10 {
			return checkWarn, fmt.Sprintf("API key seems too short (%d chars)", keyLen)
		}
		return checkPass, fmt.Sprintf("API key set (%d chars)", keyLen)
	})

	// 6. Base URL check
	dv.addCheck("Base URL", func() (checkStatus, string) {
		if cfg == nil {
			return checkSkip, "No config"
		}
		profile, err := cfg.Active()
		if err != nil {
			return checkSkip, ""
		}
		if profile.BaseURL == "" {
			if profile.Provider == "ollama" {
				return checkWarn, "Using default Ollama URL (http://localhost:11434)"
			}
			return checkWarn, "No base URL configured"
		}
		return checkPass, fmt.Sprintf("Endpoint: %s", profile.BaseURL)
	})

	// 7. Model check
	dv.addCheck("Model", func() (checkStatus, string) {
		if cfg == nil {
			return checkSkip, "No config"
		}
		profile, err := cfg.Active()
		if err != nil {
			return checkSkip, ""
		}
		if profile.Model == "" {
			return checkFail, "No model configured"
		}
		return checkPass, fmt.Sprintf("Model: %s", profile.Model)
	})

	// 8. Session store check
	dv.addCheck("Session Store", func() (checkStatus, string) {
		storePath := ".cli_mate/cli_mate.db"
		if cfg != nil && cfg.Storage.Path != "" {
			storePath = cfg.Storage.Path
		}
		return checkPass, fmt.Sprintf("Storage: %s", storePath)
	})

	// 9. Workspace check
	dv.addCheck("Workspace", func() (checkStatus, string) {
		cwd, err := os.Getwd()
		if err != nil {
			return checkWarn, "Could not determine working directory"
		}
		// Check if workspace has go.mod or package.json
		hasGoMod := false
		hasPackageJSON := false
		if _, err := os.Stat(cwd + "/go.mod"); err == nil {
			hasGoMod = true
		}
		if _, err := os.Stat(cwd + "/package.json"); err == nil {
			hasPackageJSON = true
		}
		desc := cwd
		if hasGoMod {
			desc += " (Go project)"
		} else if hasPackageJSON {
			desc += " (Node.js project)"
		}
		return checkPass, desc
	})

	// 10. MCP servers check
	dv.addCheck("MCP Servers", func() (checkStatus, string) {
		if cfg == nil || len(cfg.MCP) == 0 {
			return checkSkip, "No MCP servers configured"
		}
		return checkPass, fmt.Sprintf("%d MCP server(s) configured", len(cfg.MCP))
	})
}

// addCheck runs a check function and appends the result.
func (dv *doctorView) addCheck(name string, check func() (checkStatus, string)) {
	start := time.Now()
	status, message := check()
	duration := time.Since(start)
	dv.checks = append(dv.checks, diagnosticCheck{
		name:     name,
		status:   status,
		message:  message,
		duration: duration,
	})
}

// handleKey processes key events.
func (dv *doctorView) handleKey(key string) string {
	if !dv.visible {
		return ""
	}

	switch key {
	case "up", "shift+tab":
		if dv.cursor > 0 {
			dv.cursor--
		}
	case "down", "tab":
		if dv.cursor < len(dv.checks)-1 {
			dv.cursor++
		}
	case "r", "R":
		// Re-run checks (handled by caller)
		return "rerun"
	case "esc":
		dv.hide()
		return "close"
	}

	return ""
}

// statusIcon returns an icon for the check status.
func statusIcon(status checkStatus) string {
	switch status {
	case checkPass:
		return "✓"
	case checkFail:
		return "✗"
	case checkWarn:
		return "⚠"
	case checkSkip:
		return "–"
	default:
		return "?"
	}
}

// statusColor returns a lipgloss color for the check status.
func statusColor(status checkStatus) lipgloss.Color {
	switch status {
	case checkPass:
		return lipgloss.Color("42") // green
	case checkFail:
		return lipgloss.Color("196") // red
	case checkWarn:
		return lipgloss.Color("208") // orange
	case checkSkip:
		return lipgloss.Color("243") // gray
	default:
		return lipgloss.Color("252") // default
	}
}

// renderDoctorView renders the diagnostics panel.
func renderDoctorView(dv *doctorView, styles appStyles, width int) string {
	if !dv.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" System Diagnostics "))
	b.WriteString("\n\n")

	if !dv.complete {
		b.WriteString(styles.muted.Render("  Running diagnostics..."))
		b.WriteString("\n")
		return b.String()
	}

	if len(dv.checks) == 0 {
		b.WriteString(styles.muted.Render("  No diagnostic results."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	// Summary
	passCount := 0
	failCount := 0
	warnCount := 0
	for _, c := range dv.checks {
		switch c.status {
		case checkPass:
			passCount++
		case checkFail:
			failCount++
		case checkWarn:
			warnCount++
		}
	}

	summary := fmt.Sprintf("  %s %d passed", styles.success.Render("✓"), passCount)
	if warnCount > 0 {
		summary += fmt.Sprintf("  %s %d warnings", styles.accent.Render("⚠"), warnCount)
	}
	if failCount > 0 {
		summary += fmt.Sprintf("  %s %d failed", styles.error.Render("✗"), failCount)
	}
	b.WriteString(summary)
	b.WriteString("\n\n")

	// Individual checks
	for i, check := range dv.checks {
		icon := statusIcon(check.status)
		color := statusColor(check.status)

		line := fmt.Sprintf("  %s %s", icon, check.name)
		if i == dv.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", line)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", line))
			b.WriteString("\n")
		}

		// Show message
		if check.message != "" {
			styledMsg := lipgloss.NewStyle().Foreground(color).Render(check.message)
			b.WriteString(fmt.Sprintf("     %s", styledMsg))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · R rerun · Esc close"))
	b.WriteString("\n")

	return styles.panel.Width(width - 4).Render(b.String())
}

// renderDoctorSummary renders a compact diagnostic summary for display.
func renderDoctorSummary(dv *doctorView, styles appStyles) string {
	if !dv.complete {
		return ""
	}

	passCount := 0
	failCount := 0
	for _, c := range dv.checks {
		switch c.status {
		case checkPass:
			passCount++
		case checkFail:
			failCount++
		}
	}

	if failCount > 0 {
		return styles.error.Render(fmt.Sprintf("%d issues", failCount))
	}
	return styles.success.Render(fmt.Sprintf("%d checks passed", passCount))
}
