package oauth

import (
	"fmt"
	"strings"
)

// Flow represents the OAuth flow type.
type Flow string

const (
	FlowLoopback Flow = "loopback"
	FlowDevice   Flow = "device"
)

// Registry resolves OAuth provider configurations.
type Registry struct{}

// NewRegistry creates a new OAuth provider registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// ResolveConfig builds an OAuth Config from environment variables for a provider.
func (r *Registry) ResolveConfig(name string, env map[string]string) (Config, Flow, error) {
	if err := ValidateProviderName(name); err != nil {
		return Config{}, "", err
	}

	prefix := "CLI_MATE_OAUTH_" + strings.ToUpper(strings.ReplaceAll(name, ".", "_")) + "_"
	clientID := envValue(env, prefix+"CLIENT_ID")
	tokenURL := envValue(env, prefix+"TOKEN_URL")
	authURL := envValue(env, prefix+"AUTHORIZE_URL")
	deviceURL := envValue(env, prefix+"DEVICE_URL")
	issuerURL := envValue(env, prefix+"ISSUER_URL")
	scopesStr := envValue(env, prefix+"SCOPES")
	clientSecret := envValue(env, prefix+"CLIENT_SECRET")

	// Determine flow type
	flow := FlowLoopback
	if envValue(env, prefix+"FLOW") == "device" {
		flow = FlowDevice
	}

	if clientID == "" {
		return Config{}, "", fmt.Errorf("oauth: %s_CLIENT_ID is required", prefix)
	}
	if tokenURL == "" && issuerURL == "" {
		return Config{}, "", fmt.Errorf("oauth: either %sTOKEN_URL or %sISSUER_URL is required", prefix, prefix)
	}

	var scopes []string
	if scopesStr != "" {
		scopes = strings.Fields(scopesStr)
	}

	return Config{
		ClientID:                    clientID,
		ClientSecret:                clientSecret,
		AuthorizationEndpoint:       authURL,
		TokenEndpoint:               tokenURL,
		DeviceAuthorizationEndpoint: deviceURL,
		IssuerURL:                   issuerURL,
		Scopes:                      scopes,
	}, flow, nil
}

// ValidateProviderName enforces strict naming for provider identifiers.
func ValidateProviderName(name string) error {
	if name == "" {
		return fmt.Errorf("oauth: provider name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("oauth: provider name too long (max 64 chars)")
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_') {
			return fmt.Errorf("oauth: invalid provider name %q: only alphanumeric, dot, dash, underscore allowed", name)
		}
	}
	return nil
}
