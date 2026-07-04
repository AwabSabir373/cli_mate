package ui

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
	"cli_mate/internal/providers/registry"
	"cli_mate/pkg/crypto"
)

// setupStage represents a step in the onboarding wizard.
type setupStage int

const (
	setupStageWelcome    setupStage = iota
	setupStageProvider
	setupStageAPIKey
	setupStageBaseURL
	setupStageModel
	setupStageReview
	setupStageComplete
)

// onboardingState manages the multi-step onboarding wizard.
type onboardingState struct {
	active      bool
	stage       setupStage
	provider    string
	apiKey      string
	baseURL     string
	model       string
	cursor      int // selection cursor within the current stage
	err         string
	providers   []string // available provider names
	models      []string // available models for selected provider
	skipAPIKey  bool     // for providers that don't need API key
}

// newOnboardingState creates a new onboarding state.
func newOnboardingState() *onboardingState {
	specs := registry.Specs()
	providerNames := make([]string, 0, len(specs))
	for _, spec := range specs {
		providerNames = append(providerNames, spec.Name)
	}

	return &onboardingState{
		active:    false,
		stage:     setupStageWelcome,
		providers: providerNames,
	}
}

// start begins the onboarding wizard.
func (os *onboardingState) start() {
	os.active = true
	os.stage = setupStageWelcome
	os.cursor = 0
	os.err = ""
}

// isActive returns true if onboarding is in progress.
func (os *onboardingState) isActive() bool {
	return os.active
}

// handleKey processes a keypress and returns (shouldClose bool, errMsg string).
func (os *onboardingState) handleKey(key string) (bool, string) {
	if !os.active {
		return false, ""
	}

	switch os.stage {
	case setupStageWelcome:
		return os.handleWelcomeKey(key)
	case setupStageProvider:
		return os.handleProviderKey(key)
	case setupStageAPIKey:
		return os.handleAPIKeyKey(key)
	case setupStageBaseURL:
		return os.handleBaseURLKey(key)
	case setupStageModel:
		return os.handleModelKey(key)
	case setupStageReview:
		return os.handleReviewKey(key)
	}

	return false, ""
}

func (os *onboardingState) handleWelcomeKey(key string) (bool, string) {
	switch key {
	case "enter", " ":
		os.stage = setupStageProvider
		os.cursor = 0
		return false, ""
	case "esc":
		os.active = false
		return true, ""
	}
	return false, ""
}

func (os *onboardingState) handleProviderKey(key string) (bool, string) {
	switch key {
	case "up", "shift+tab":
		if os.cursor > 0 {
			os.cursor--
		}
	case "down", "tab":
		if os.cursor < len(os.providers)-1 {
			os.cursor++
		}
	case "enter", " ":
		if os.cursor >= 0 && os.cursor < len(os.providers) {
			os.provider = os.providers[os.cursor]
			spec, ok := registry.SpecByName(os.provider)
			if ok {
				os.baseURL = spec.DefaultBaseURL
				os.model = spec.DefaultModel

				// Determine next stage
				if spec.RequiresKey {
					os.stage = setupStageAPIKey
				} else if spec.Name == "custom" {
					os.stage = setupStageBaseURL
				} else {
					os.stage = setupStageModel
				}
				os.cursor = 0

				// Load models for this provider
				os.models = registry.Models(os.provider)
			}
		}
	case "esc":
		os.stage = setupStageWelcome
		os.cursor = 0
	}
	return false, ""
}

func (os *onboardingState) handleAPIKeyKey(key string) (bool, string) {
	switch key {
	case "esc":
		os.stage = setupStageProvider
		os.cursor = 0
	default:
		// Most keys should be handled by the input field, not the wizard
		if key == "enter" {
			if os.apiKey == "" {
				os.err = "API key is required. Paste your API key or press Esc to go back."
				return false, ""
			}
			spec, _ := registry.SpecByName(os.provider)
			if spec.Name == "custom" && os.baseURL == "" {
				os.stage = setupStageBaseURL
			} else {
				os.stage = setupStageModel
			}
			os.cursor = 0
			os.err = ""
			return false, ""
		}
	}
	return false, ""
}

func (os *onboardingState) handleBaseURLKey(key string) (bool, string) {
	switch key {
	case "esc":
		spec, ok := registry.SpecByName(os.provider)
		if ok && spec.RequiresKey {
			os.stage = setupStageAPIKey
		} else {
			os.stage = setupStageProvider
		}
		os.cursor = 0
	default:
		if key == "enter" {
			if os.baseURL == "" {
				os.err = "Base URL is required for custom providers."
				return false, ""
			}
			os.stage = setupStageModel
			os.cursor = 0
			os.err = ""
			return false, ""
		}
	}
	return false, ""
}

func (os *onboardingState) handleModelKey(key string) (bool, string) {
	switch key {
	case "up", "shift+tab":
		if os.cursor > 0 {
			os.cursor--
		}
	case "down", "tab":
		if os.cursor < len(os.models)-1 {
			os.cursor++
		}
	case "enter", " ":
		if os.cursor >= 0 && os.cursor < len(os.models) {
			os.model = os.models[os.cursor]
			os.stage = setupStageReview
			os.cursor = 0
		}
	case "esc":
		spec, ok := registry.SpecByName(os.provider)
		if ok && spec.RequiresKey {
			os.stage = setupStageAPIKey
		} else {
			os.stage = setupStageProvider
		}
		os.cursor = 0
	}
	return false, ""
}

func (os *onboardingState) handleReviewKey(key string) (bool, string) {
	switch key {
	case "up", "shift+tab":
		if os.cursor > 0 {
			os.cursor--
		}
	case "down", "tab":
		if os.cursor < 2 {
			os.cursor++
		}
	case "enter", " ":
		switch os.cursor {
		case 0:
			// Confirm and save
			os.stage = setupStageComplete
			os.cursor = 0
			return false, ""
		case 1:
			// Go back to provider selection
			os.stage = setupStageProvider
			os.cursor = 0
		case 2:
			// Cancel
			os.active = false
			return true, ""
		}
	case "esc":
		os.stage = setupStageModel
		os.cursor = 0
	}
	return false, ""
}

// applyConfig applies the onboarding configuration to the app's config.
func (os *onboardingState) applyConfig(a *App) {
	_ = a.cfg.UpdateActive(func(profile *config.Profile) {
		profile.Provider = os.provider
		if os.apiKey != "" {
			// Encrypt the API key before saving
			if !crypto.IsEncrypted(os.apiKey) {
				if enc, err := crypto.Encrypt(os.apiKey); err == nil {
					profile.APIKey = enc
				}
			} else {
				profile.APIKey = os.apiKey
			}
		}
		if os.baseURL != "" {
			profile.BaseURL = os.baseURL
		}
		if os.model != "" {
			profile.Model = os.model
		}
	})
	a.saveSettings()
	a.disconnect()
}

// render renders the current onboarding stage.
func (os *onboardingState) render(styles appStyles, width int) string {
	if !os.active {
		return ""
	}

	switch os.stage {
	case setupStageWelcome:
		return os.renderWelcome(styles, width)
	case setupStageProvider:
		return os.renderProvider(styles, width)
	case setupStageAPIKey:
		return os.renderAPIKey(styles, width)
	case setupStageBaseURL:
		return os.renderBaseURL(styles, width)
	case setupStageModel:
		return os.renderModel(styles, width)
	case setupStageReview:
		return os.renderReview(styles, width)
	case setupStageComplete:
		return os.renderComplete(styles, width)
	}
	return ""
}

func (os *onboardingState) renderWelcome(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Setup Wizard "))
	b.WriteString("\n\n")
	b.WriteString(styles.title.Render("Welcome to cli_mate!"))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("This wizard will help you configure your AI provider in 4 easy steps."))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("You'll need:"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  • An AI provider (OpenAI, Anthropic, etc.)"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  • An API key (if required by the provider)"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  • A model choice"))
	b.WriteString("\n\n")
	b.WriteString(styles.success.Render("  Press Enter to begin"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Press Esc to cancel"))
	b.WriteString("\n")
	return b.String()
}

func (os *onboardingState) renderProvider(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Step 1: Choose Provider "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Select your AI provider:"))
	b.WriteString("\n\n")

	for i, provider := range os.providers {
		spec, ok := registry.SpecByName(provider)
		desc := ""
		if ok {
			desc = fmt.Sprintf("default: %s", spec.DefaultModel)
			if spec.RequiresKey {
				desc += " · API key required"
			}
		}
		if i == os.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", provider)))
			b.WriteString("\n")
			if desc != "" {
				b.WriteString(styles.muted.Render(fmt.Sprintf("   %s", desc)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(fmt.Sprintf("   %s", provider))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (os *onboardingState) renderAPIKey(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Step 2: API Key "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render(fmt.Sprintf("Enter your %s API key:", os.provider)))
	b.WriteString("\n\n")

	masked := ""
	if os.apiKey != "" {
		if len(os.apiKey) > 8 {
			masked = os.apiKey[:4] + strings.Repeat("*", len(os.apiKey)-8) + os.apiKey[len(os.apiKey)-4:]
		} else {
			masked = strings.Repeat("*", len(os.apiKey))
		}
	}

	b.WriteString(styles.prompt.Render("  API Key: "))
	b.WriteString(styles.input.Render(masked))
	b.WriteString("\n")

	if os.err != "" {
		b.WriteString("\n")
		b.WriteString(styles.error.Render(os.err))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if os.apiKey == "" {
		b.WriteString(styles.muted.Render("  Type or paste your API key · Enter to confirm · Esc back"))
	} else {
		b.WriteString(styles.muted.Render("  Enter to confirm · Esc back"))
	}
	b.WriteString("\n")
	return b.String()
}

func (os *onboardingState) renderBaseURL(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Step 2: Base URL "))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Enter the provider's API base URL:"))
	b.WriteString("\n\n")

	b.WriteString(styles.prompt.Render("  Base URL: "))
	b.WriteString(styles.input.Render(os.baseURL))
	b.WriteString("\n")

	if os.err != "" {
		b.WriteString("\n")
		b.WriteString(styles.error.Render(os.err))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Type the URL · Enter to confirm · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (os *onboardingState) renderModel(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(fmt.Sprintf(" Step 3: Choose Model (%s) ", os.provider)))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render("Select a model:"))
	b.WriteString("\n\n")

	models := os.models
	if len(models) == 0 {
		// Fallback models if registry doesn't provide any
		models = []string{"gpt-4.1-mini", "gpt-4.1", "gpt-4o", "claude-sonnet-4-20250514", "gemini-2.5-flash"}
	}

	for i, model := range models {
		if i == os.cursor {
			if i < len(models) {
				b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", model)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(fmt.Sprintf("   %s", model))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (os *onboardingState) renderReview(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Step 4: Review "))
	b.WriteString("\n\n")
	b.WriteString(styles.title.Render("Review your configuration:"))
	b.WriteString("\n\n")

	b.WriteString(styles.accent.Render("  Provider:"))
	b.WriteString(fmt.Sprintf("  %s", os.provider))
	b.WriteString("\n")

	if os.apiKey != "" {
		masked := os.apiKey[:4] + strings.Repeat("*", len(os.apiKey)-8) + os.apiKey[len(os.apiKey)-4:]
		b.WriteString(styles.accent.Render("  API Key:"))
		b.WriteString(fmt.Sprintf("  %s", masked))
		b.WriteString("\n")
	}

	if os.baseURL != "" {
		b.WriteString(styles.accent.Render("  Base URL:"))
		b.WriteString(fmt.Sprintf("  %s", os.baseURL))
		b.WriteString("\n")
	}

	b.WriteString(styles.accent.Render("  Model:"))
	b.WriteString(fmt.Sprintf("     %s", os.model))
	b.WriteString("\n\n")

	options := []string{"✓ Save & Connect", "← Change Provider", "✕ Cancel"}
	for i, opt := range options {
		if i == os.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf("  %s", opt)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s", opt))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")
	return b.String()
}

func (os *onboardingState) renderComplete(styles appStyles, _ int) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(" Setup Complete "))
	b.WriteString("\n\n")
	b.WriteString(styles.success.Render("✓ Configuration saved!"))
	b.WriteString("\n\n")
	b.WriteString(styles.muted.Render(fmt.Sprintf("Provider: %s", os.provider)))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render(fmt.Sprintf("Model:    %s", os.model)))
	b.WriteString("\n\n")
	b.WriteString(styles.accent.Render("  Press Enter to start chatting"))
	b.WriteString("\n")
	return b.String()
}

// isComplete returns true if the onboarding wizard has finished.
func (os *onboardingState) isComplete() bool {
	return os.stage == setupStageComplete
}

// reset clears the onboarding state.
func (os *onboardingState) reset() {
	os.active = false
	os.stage = setupStageWelcome
	os.provider = ""
	os.apiKey = ""
	os.baseURL = ""
	os.model = ""
	os.cursor = 0
	os.err = ""
}
