package ui

import (
	"fmt"
	"strings"
)

// WizardStep represents a step in the provider setup wizard.
type WizardStep string

const (
	WizardStepProvider WizardStep = "provider"
	WizardStepAPIKey   WizardStep = "api_key"
	WizardStepModel    WizardStep = "model"
	WizardStepComplete WizardStep = "complete"
)

// ProviderWizard guides users through provider setup.
type ProviderWizard struct {
	visible  bool
	step     WizardStep
	provider string
	apiKey   string
	model    string
	error    string
}

// NewProviderWizard creates a new provider wizard.
func NewProviderWizard() *ProviderWizard {
	return &ProviderWizard{
		step: WizardStepProvider,
	}
}

// Start begins the wizard flow.
func (pw *ProviderWizard) Start() {
	pw.visible = true
	pw.step = WizardStepProvider
	pw.provider = ""
	pw.apiKey = ""
	pw.model = ""
	pw.error = ""
}

// Cancel cancels the wizard.
func (pw *ProviderWizard) Cancel() {
	pw.visible = false
	pw.step = WizardStepProvider
}

// IsVisible returns whether the wizard is visible.
func (pw *ProviderWizard) IsVisible() bool {
	return pw.visible
}

// SetProvider sets the selected provider and advances to next step.
func (pw *ProviderWizard) SetProvider(provider string) {
	pw.provider = provider
	pw.step = WizardStepAPIKey
	pw.error = ""
}

// SetAPIKey sets the API key and advances to next step.
func (pw *ProviderWizard) SetAPIKey(key string) {
	pw.apiKey = key
	pw.step = WizardStepModel
	pw.error = ""
}

// SetModel sets the model and completes the wizard.
func (pw *ProviderWizard) SetModel(model string) {
	pw.model = model
	pw.step = WizardStepComplete
	pw.visible = false
}

// SetError sets an error message.
func (pw *ProviderWizard) SetError(msg string) {
	pw.error = msg
}

// GetResult returns the configured provider settings.
func (pw *ProviderWizard) GetResult() (provider, apiKey, model string) {
	return pw.provider, pw.apiKey, pw.model
}

// Render produces the wizard view.
func (pw *ProviderWizard) Render(width int, styles appStyles) string {
	if !pw.visible {
		return ""
	}

	var lines []string

	switch pw.step {
	case WizardStepProvider:
		lines = append(lines, styles.pill.Render("Step 1: Choose Provider"))
		lines = append(lines, "")
		lines = append(lines, "Available providers:")
		lines = append(lines, "")
		providers := []struct {
			name string
			desc string
		}{
			{"openai", "OpenAI (GPT-4, GPT-4.1)"},
			{"anthropic", "Anthropic (Claude)"},
			{"gemini", "Google Gemini"},
			{"ollama", "Ollama (local)"},
			{"openrouter", "OpenRouter (multi-provider)"},
			{"groq", "Groq (fast inference)"},
			{"deepseek", "DeepSeek"},
			{"mistral", "Mistral AI"},
			{"lmstudio", "LM Studio (local)"},
			{"xai", "xAI (Grok)"},
			{"custom", "Custom provider"},
		}
		for _, p := range providers {
			lines = append(lines, fmt.Sprintf("  • %s - %s", styles.accent.Render(p.name), styles.muted.Render(p.desc)))
		}
		lines = append(lines, "")
		lines = append(lines, styles.muted.Render("Type the provider name to select"))

	case WizardStepAPIKey:
		lines = append(lines, styles.pill.Render(fmt.Sprintf("Step 2: API Key for %s", pw.provider)))
		lines = append(lines, "")
		if pw.provider == "ollama" || pw.provider == "lmstudio" {
			lines = append(lines, "No API key needed for local providers.")
			lines = append(lines, "")
			lines = append(lines, styles.muted.Render("Press Enter to continue"))
		} else {
			lines = append(lines, "Enter your API key:")
			lines = append(lines, "")
			lines = append(lines, styles.muted.Render("(Key will be encrypted and stored securely)"))
		}

	case WizardStepModel:
		lines = append(lines, styles.pill.Render(fmt.Sprintf("Step 3: Choose Model for %s", pw.provider)))
		lines = append(lines, "")
		lines = append(lines, "Available models:")
		lines = append(lines, "")
		models := pw.getModelsForProvider()
		for _, m := range models {
			lines = append(lines, fmt.Sprintf("  • %s", styles.accent.Render(m)))
		}
		lines = append(lines, "")
		lines = append(lines, styles.muted.Render("Type the model name to select"))

	case WizardStepComplete:
		lines = append(lines, styles.success.Render("✓ Provider configured successfully!"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Provider: %s", pw.provider))
		lines = append(lines, fmt.Sprintf("Model: %s", pw.model))
	}

	if pw.error != "" {
		lines = append(lines, "")
		lines = append(lines, styles.error.Render("Error: "+pw.error))
	}

	return strings.Join(lines, "\n")
}

func (pw *ProviderWizard) getModelsForProvider() []string {
	switch pw.provider {
	case "openai":
		return []string{"gpt-4", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "o3-mini"}
	case "anthropic":
		return []string{"claude-sonnet-4-20250514", "claude-3-5-haiku-20241022", "claude-3-opus-20240229"}
	case "gemini":
		return []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash"}
	case "ollama":
		return []string{"llama3.1", "codellama", "mistral", "phi3"}
	case "openrouter":
		return []string{"openai/gpt-4.1-mini", "anthropic/claude-sonnet-4-20250514", "meta-llama/llama-3.1-405b"}
	case "groq":
		return []string{"llama-3.3-70b-versatile", "mixtral-8x7b-32768"}
	case "deepseek":
		return []string{"deepseek-chat", "deepseek-coder"}
	case "mistral":
		return []string{"mistral-large-latest", "mistral-medium-latest"}
	case "lmstudio":
		return []string{"local model"}
	case "xai":
		return []string{"grok-3-mini", "grok-3"}
	case "custom":
		return []string{"(enter custom model ID)"}
	default:
		return []string{}
	}
}
