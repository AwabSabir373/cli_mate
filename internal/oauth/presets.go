package oauth

// providerPreset defines the default OAuth configuration for a provider.
type providerPreset struct {
	ClientID  string
	AuthURL   string
	TokenURL  string
	DeviceURL string
	IssuerURL string
	Scopes    []string
	Flow      Flow
}

// builtinOAuthPresets contains pre-configured OAuth settings for known providers.
// These are disabled by default unless the user opts in via environment variable.
var builtinOAuthPresets = map[string]providerPreset{
	"chatgpt": {
		ClientID:  "oidc",
		AuthURL:   "https://auth.openai.com/authorize",
		TokenURL:  "https://auth.openai.com/token",
		IssuerURL: "https://auth.openai.com",
		Scopes:    []string{"openid", "email", "profile"},
		Flow:      FlowLoopback,
	},
}

// lookupOAuthPreset returns a preset configuration for a provider.
func lookupOAuthPreset(name string) (providerPreset, bool) {
	preset, ok := builtinOAuthPresets[name]
	return preset, ok
}

// envWithPresetsAllowed creates an environment map that enables presets.
func envWithPresetsAllowed(env map[string]string) map[string]string {
	m := make(map[string]string, len(env)+1)
	for k, v := range env {
		m[k] = v
	}
	m["CLI_MATE_OAUTH_ALLOW_PRESETS"] = "1"
	return m
}
