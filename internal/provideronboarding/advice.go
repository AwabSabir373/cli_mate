// Package provideronboarding provides setup advice, probe classification,
// and local runtime detection for the provider onboarding wizard.
package provideronboarding

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
)

// Action represents a suggested setup step for a provider.
type Action struct {
	Label   string
	Command string
	Detail  string
}

// ProviderState holds a provider profile and whether it's the active one.
type ProviderState struct {
	Profile config.Profile
	Active  bool
}

// Actions returns relevant setup actions for this provider state.
func (ps ProviderState) Actions() []Action {
	return ProviderActions(ps.Profile, ps.Active)
}

// SetupCommand generates a command string to add/setup a provider.
func SetupCommand(provider string) string {
	return fmt.Sprintf("cli_mate /provider %s", provider)
}

// UseCommand generates a command string to switch to a provider.
func UseCommand(provider string) string {
	return fmt.Sprintf("cli_mate /use %s", provider)
}

// CheckCommand generates a command string to test a provider's health.
func CheckCommand(provider string) string {
	return fmt.Sprintf("cli_mate /provider check %s", provider)
}

// ProviderActions builds a list of Actions for a provider profile.
func ProviderActions(profile config.Profile, isActive bool) []Action {
	var actions []Action

	if !isActive {
		actions = append(actions, Action{
			Label:   "Use this provider",
			Command: UseCommand(profile.Provider),
			Detail:  fmt.Sprintf("Switch to %s as the active provider", profile.Provider),
		})
	}

	actions = append(actions, Action{
		Label:   "Check provider health",
		Command: CheckCommand(profile.Provider),
		Detail:  "Verify connectivity and configuration",
	})

	if credAction := MissingCredentialAction(profile); credAction != nil {
		actions = append(actions, *credAction)
	}

	return actions
}

// MissingCredentialAction detects if a provider needs API key setup.
func MissingCredentialAction(profile config.Profile) *Action {
	if providerProfileHasCredential(profile) {
		return nil
	}

	envVar := credentialEnvVarForProfile(profile)
	if envVar == "" {
		return nil
	}

	return &Action{
		Label:   "Set API key",
		Command: fmt.Sprintf("set %s=<your-key>", envVar),
		Detail:  fmt.Sprintf("Set the %s environment variable or configure it via /provider %s", envVar, profile.Provider),
	}
}

// providerProfileHasCredential checks if a profile has an API key or auth header set.
func providerProfileHasCredential(profile config.Profile) bool {
	return strings.TrimSpace(profile.APIKey) != ""
}

// credentialEnvVarForProfile returns the expected env var name for a provider's API key.
func credentialEnvVarForProfile(profile config.Profile) string {
	switch profile.Provider {
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
	case "custom":
		return "CUSTOM_API_KEY"
	default:
		return ""
	}
}

// credentialAdvice returns human-readable advice about credential setup.
func credentialAdvice(provider string) string {
	envVar := credentialEnvVarForProfile(config.Profile{Provider: provider})
	if envVar == "" {
		return ""
	}
	return fmt.Sprintf("Set %s environment variable with your API key, or use the interactive setup.", envVar)
}
