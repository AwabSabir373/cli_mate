package provideronboarding

import (
	"fmt"
	"strings"
)

// SetupProbeClass categorizes a provider health check failure.
type SetupProbeClass string

const (
	ProbeAuth      SetupProbeClass = "auth"
	ProbeEndpoint  SetupProbeClass = "endpoint"
	ProbeModel     SetupProbeClass = "model"
	ProbeRateLimit SetupProbeClass = "rate_limit"
	ProbeConfig    SetupProbeClass = "config"
	ProbeUnknown   SetupProbeClass = "unknown"
)

// SetupProbeError is a classified, user-facing error from a provider health check.
type SetupProbeError struct {
	Class   SetupProbeClass
	Message string
	Detail  string
}

func (e SetupProbeError) Error() string {
	return e.Message
}

// ProbeResult is a simplified health check result for onboarding.
type ProbeResult struct {
	Status     string // "pass", "warn", "fail"
	Category   string // "config", "auth", "network", "timeout", "rate_limit", "provider_error", "connectivity"
	Message    string
	StatusCode int
}

// ClassifySetupProbe maps a ProbeResult to a user-friendly SetupProbeError.
func ClassifySetupProbe(result ProbeResult) SetupProbeError {
	switch result.Category {
	case "auth":
		return SetupProbeError{
			Class:   ProbeAuth,
			Message: "Invalid or missing API key. Check your credentials and try again.",
			Detail:  result.Message,
		}
	case "network", "timeout":
		return SetupProbeError{
			Class:   ProbeEndpoint,
			Message: "Cannot reach the provider endpoint. Check your base URL and internet connection.",
			Detail:  result.Message,
		}
	case "connectivity":
		return SetupProbeError{
			Class:   ProbeEndpoint,
			Message: "Provider endpoint is unreachable. Verify the URL is correct and accessible.",
			Detail:  result.Message,
		}
	case "rate_limit":
		return SetupProbeError{
			Class:   ProbeRateLimit,
			Message: "Rate limited by provider. Wait a moment and try again.",
			Detail:  result.Message,
		}
	case "config":
		return SetupProbeError{
			Class:   ProbeConfig,
			Message: "Provider configuration is incomplete. Complete the setup and try again.",
			Detail:  result.Message,
		}
	case "provider_error":
		if isModelNotFound(result.Message, result.StatusCode) {
			return SetupProbeError{
				Class:   ProbeModel,
				Message: "Model not found. Choose a different model or check the model name.",
				Detail:  result.Message,
			}
		}
		return SetupProbeError{
			Class:   ProbeUnknown,
			Message: "Provider returned an error. Check configuration and try again.",
			Detail:  result.Message,
		}
	default:
		return SetupProbeError{
			Class:   ProbeUnknown,
			Message: "Unknown error during provider check. Verify your configuration.",
			Detail:  result.Message,
		}
	}
}

// isModelNotFound checks if a provider error is specifically about a missing model.
func isModelNotFound(message string, statusCode int) bool {
	msg := strings.ToLower(message)
	if statusCode == 404 && strings.Contains(msg, "model") {
		return true
	}
	if strings.Contains(msg, "not found") && strings.Contains(msg, "model") {
		return true
	}
	if strings.Contains(msg, "does not exist") || strings.Contains(msg, "unknown model") {
		return true
	}
	return false
}

// FormatProbeError returns a formatted string for display in the onboarding wizard.
func FormatProbeError(probeErr SetupProbeError) string {
	icon := probeIcon(probeErr.Class)
	return fmt.Sprintf("%s %s", icon, probeErr.Message)
}

func probeIcon(class SetupProbeClass) string {
	switch class {
	case ProbeAuth:
		return "🔑"
	case ProbeEndpoint:
		return "🔌"
	case ProbeModel:
		return "🤖"
	case ProbeRateLimit:
		return "⏳"
	case ProbeConfig:
		return "⚙️"
	default:
		return "❌"
	}
}
