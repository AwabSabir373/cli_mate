// Package doctor provides diagnostic health checks for the cli_mate environment.
// It verifies configuration, provider connectivity, and system dependencies.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cli_mate/internal/config"
)

// Status represents the result of a single diagnostic check.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check represents a single diagnostic test result.
type Check struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Status  Status   `json:"status"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// Report is the complete diagnostic report.
type Report struct {
	Timestamp time.Time `json:"timestamp"`
	OK        bool      `json:"ok"`
	Checks    []Check   `json:"checks"`
}

// Options configures the diagnostic run.
type Options struct {
	Config    *config.Config
	Workspace string
}

// Run executes all diagnostic checks and returns a Report.
func Run(opts Options) Report {
	report := Report{
		Timestamp: time.Now(),
		Checks:    make([]Check, 0),
	}

	report.Checks = append(report.Checks, checkRuntime())
	report.Checks = append(report.Checks, checkConfig(opts.Config))
	report.Checks = append(report.Checks, checkProvider(opts.Config))
	report.Checks = append(report.Checks, checkLSP())

	// Determine overall status
	report.OK = true
	for _, c := range report.Checks {
		if c.Status == StatusFail {
			report.OK = false
		}
	}

	return report
}

// checkRuntime verifies the Go runtime is available.
func checkRuntime() Check {
	version := runtime.Version()
	return Check{
		ID:      "runtime",
		Label:   "Go Runtime",
		Status:  StatusPass,
		Message: fmt.Sprintf("Go %s (%s/%s)", version, runtime.GOOS, runtime.GOARCH),
	}
}

// checkConfig validates the configuration.
func checkConfig(cfg *config.Config) Check {
	if cfg == nil {
		return Check{
			ID:      "config",
			Label:   "Configuration",
			Status:  StatusFail,
			Message: "No configuration loaded",
		}
	}

	profile, err := cfg.Active()
	if err != nil {
		return Check{
			ID:      "config",
			Label:   "Configuration",
			Status:  StatusFail,
			Message: fmt.Sprintf("Active profile: %v", err),
		}
	}

	var details []string
	details = append(details, fmt.Sprintf("Profile: %s", cfg.ActiveProfile))
	details = append(details, fmt.Sprintf("Provider: %s", profile.Provider))
	if profile.Model != "" {
		details = append(details, fmt.Sprintf("Model: %s", profile.Model))
	}
	if profile.BaseURL != "" {
		details = append(details, fmt.Sprintf("Base URL: %s", profile.BaseURL))
	}

	return Check{
		ID:      "config",
		Label:   "Configuration",
		Status:  StatusPass,
		Message: fmt.Sprintf("Profile %q configured", cfg.ActiveProfile),
		Details: details,
	}
}

// checkProvider validates the provider configuration.
func checkProvider(cfg *config.Config) Check {
	if cfg == nil {
		return Check{
			ID:      "provider",
			Label:   "Provider",
			Status:  StatusFail,
			Message: "No configuration loaded",
		}
	}

	profile, err := cfg.Active()
	if err != nil {
		return Check{
			ID:      "provider",
			Label:   "Provider",
			Status:  StatusFail,
			Message: err.Error(),
		}
	}

	if profile.Provider == "" {
		return Check{
			ID:      "provider",
			Label:   "Provider",
			Status:  StatusFail,
			Message: "No provider configured. Use /provider to set one up.",
		}
	}

	// Check if provider requires an API key
	isLocalProvider := isLocalURL(profile.BaseURL)
	if !isLocalProvider && profile.APIKey == "" {
		return Check{
			ID:      "provider",
			Label:   "Provider",
			Status:  StatusWarn,
			Message: fmt.Sprintf("Provider %q has no API key configured. Set via /api-key", profile.Provider),
			Details: []string{credentialEnvVar(profile.Provider)},
		}
	}

	if profile.Model == "" {
		return Check{
			ID:      "provider",
			Label:   "Provider",
			Status:  StatusWarn,
			Message: fmt.Sprintf("Provider %q has no model selected. Use /model", profile.Provider),
		}
	}

	return Check{
		ID:      "provider",
		Label:   "Provider",
		Status:  StatusPass,
		Message: fmt.Sprintf("%s with %s", profile.Provider, profile.Model),
	}
}

// checkLSP verifies that common LSP servers are available on the system PATH.
func checkLSP() Check {
	lspServers := []struct {
		name    string
		binary  string
		install string
	}{
		{"gopls", "gopls", "go install golang.org/x/tools/gopls@latest"},
		{"TypeScript", "typescript-language-server", "npm install -g typescript-language-server"},
		{"Rust", "rust-analyzer", "rustup component add rust-analyzer"},
		{"Python", "pyright", "npm install -g pyright"},
	}

	var missing []string
	var found []string

	for _, lsp := range lspServers {
		if _, err := exec.LookPath(lsp.binary); err == nil {
			found = append(found, lsp.name)
		} else {
			missing = append(missing, fmt.Sprintf("%s (install: %s)", lsp.name, lsp.install))
		}
	}

	if len(missing) == 0 {
		return Check{
			ID:      "lsp",
			Label:   "LSP Servers",
			Status:  StatusPass,
			Message: fmt.Sprintf("All checked LSP servers available (%s)", strings.Join(found, ", ")),
		}
	}

	var details []string
	for _, m := range missing {
		details = append(details, m)
	}

	status := StatusWarn
	msg := fmt.Sprintf("%d LSP server(s) available", len(found))
	if len(found) == 0 {
		msg = "No LSP servers found. Code editing will use text-only mode."
	} else {
		msg = fmt.Sprintf("%d of %d LSP servers available: %s", len(found), len(found)+len(missing), strings.Join(found, ", "))
	}

	return Check{
		ID:      "lsp",
		Label:   "LSP Servers",
		Status:  status,
		Message: msg,
		Details: details,
	}
}

// FormatReport produces a human-readable string from a diagnostic Report.
func FormatReport(r Report) string {
	var b strings.Builder

	b.WriteString("═══ cli_mate Health Report ═══")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Time: %s\n\n", r.Timestamp.Format(time.RFC3339)))

	for _, check := range r.Checks {
		icon := statusIcon(check.Status)
		b.WriteString(fmt.Sprintf("%s %s\n", icon, check.Label))
		b.WriteString(fmt.Sprintf("   %s\n", check.Message))
		for _, d := range check.Details {
			b.WriteString(fmt.Sprintf("   • %s\n", d))
		}
		b.WriteString("\n")
	}

	if r.OK {
		b.WriteString("✓ All checks passed\n")
	} else {
		b.WriteString("✗ Some checks failed. Run /doctor for details.\n")
	}

	return b.String()
}

func statusIcon(s Status) string {
	switch s {
	case StatusPass:
		return "✓"
	case StatusWarn:
		return "⚠"
	case StatusFail:
		return "✗"
	default:
		return "?"
	}
}

func isLocalURL(baseURL string) bool {
	if baseURL == "" {
		return false
	}
	return strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")
}

func credentialEnvVar(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "mistral":
		return "MISTRAL_API_KEY"
	case "xai":
		return "XAI_API_KEY"
	default:
		return provider + "_API_KEY"
	}
}

// defaultConfigPath returns the default config file path.
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cli_mate.yaml"
	}
	return filepath.Join(home, ".config", "cli_mate", "cli_mate.yaml")
}

// ConfigFileExists checks if the config file exists on disk.
func ConfigFileExists() bool {
	_, err := os.Stat(defaultConfigPath())
	return err == nil
}
