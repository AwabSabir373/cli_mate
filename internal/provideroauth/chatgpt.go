package provideroauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cli_mate/internal/oauth"
)

const chatgptLoopbackPort = 1455

// ChatGPTOptions configures the ChatGPT OAuth login flow.
type ChatGPTOptions struct {
	HTTPClient  *http.Client
	OpenBrowser func(url string) error
	Env         map[string]string
	Out         io.Writer
	Timeout     time.Duration
}

// ChatGPTLogin runs the ChatGPT OAuth loopback login flow.
// It returns the OAuth token status on success.
func ChatGPTLogin(ctx context.Context, opts ChatGPTOptions) (oauth.Status, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}

	// Set up the presets-enabled environment
	env := opts.Env
	if env == nil {
		env = map[string]string{}
	}
	env["CLI_MATE_OAUTH_ALLOW_PRESETS"] = "1"

	// Build OAuth config for ChatGPT preset
	cfg := oauth.Config{
		ClientID:              "oidc",
		AuthorizationEndpoint: "https://auth.openai.com/authorize",
		TokenEndpoint:         "https://auth.openai.com/token",
		IssuerURL:             "https://auth.openai.com",
		Scopes:                []string{"openid", "email", "profile"},
		ExtraAuthParams: map[string]string{
			"id_token_add_organizations": "true",
			"codex_cli_simplified_flow": "true",
			"originator":                "cli_mate",
		},
	}

	loginCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	state, err := oauth.NewState()
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: generate state: %w", err)
	}

	pkce, err := oauth.NewPKCE()
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: generate PKCE: %w", err)
	}

	listener, err := oauth.NewLoopbackListenerOnPort(state, chatgptLoopbackPort)
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: start listener: %w", err)
	}
	defer listener.Close()

	redirectURI := listener.RedirectURIWithHost("localhost", "/auth/callback")
	authURL, err := oauth.BuildAuthorizationURL(cfg, pkce, state, redirectURI, nil)
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: build auth URL: %w", err)
	}

	fmt.Fprintf(opts.Out, "Open this URL to authorize with ChatGPT:\n  %s\n", authURL)
	if opts.OpenBrowser != nil {
		_ = opts.OpenBrowser(authURL)
	}

	code, err := listener.Wait(loginCtx)
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: wait for callback: %w", err)
	}

	token, err := oauth.ExchangeCode(loginCtx, opts.HTTPClient, cfg, code, pkce.Verifier, redirectURI, func() time.Time { return time.Now() })
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: exchange code: %w", err)
	}

	// Extract account ID from ID token
	accountID := extractChatGPTAccountID(token.IDToken)
	if accountID != "" {
		token.Account = accountID
	}

	// Store the token
	store, err := oauth.NewStore(oauth.StoreOptions{
		Env: env,
		Now: func() time.Time { return time.Now() },
	})
	if err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: create store: %w", err)
	}

	key := oauth.ProviderKey("chatgpt")
	if err := store.Save(key, token); err != nil {
		return oauth.Status{}, fmt.Errorf("chatgpt oauth: save token: %w", err)
	}

	statuses, err := store.Status(oauth.KeyPrefixProvider)
	if err != nil {
		return oauth.Status{}, err
	}
	for _, st := range statuses {
		if st.Key == key {
			return st, nil
		}
	}
	return oauth.Status{Key: key}, nil
}

// extractChatGPTAccountID parses the ID token JWS to extract the chatgpt_account_id claim.
func extractChatGPTAccountID(idToken string) string {
	if idToken == "" {
		return ""
	}

	// JWS format: header.payload.signature
	parts := strings.SplitN(idToken, ".", 3)
	if len(parts) < 2 {
		return ""
	}

	// Decode the payload (part 1)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try standard base64
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}

	var claims struct {
		ChatGPTAccountID string `json:"https://api.openai.com/auth/chatgpt_account_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}

	// Fallback: try top-level
	var fallback struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	}
	if err := json.Unmarshal(payload, &fallback); err != nil {
		return ""
	}
	return fallback.ChatGPTAccountID
}
