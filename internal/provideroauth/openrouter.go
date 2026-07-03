package provideroauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cli_mate/internal/oauth"
)

const (
	openRouterDefaultBaseURL = "https://openrouter.ai"
	openRouterAuthPath       = "/auth"
	openRouterKeyExchangeAPI = "/api/v1/auth/keys"
)

// OpenRouterOptions configures the OpenRouter OAuth login flow.
type OpenRouterOptions struct {
	BaseURL     string
	HTTPClient  *http.Client
	OpenBrowser func(url string) error
	Out         io.Writer
	Timeout     time.Duration
}

// openRouterKeyResponse is the response from the OpenRouter key exchange API.
type openRouterKeyResponse struct {
	Key string `json:"key"`
}

// OpenRouterLogin runs the OpenRouter OAuth loopback login flow,
// which exchanges OAuth authorization for an API key.
func OpenRouterLogin(ctx context.Context, opts OpenRouterOptions) (string, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = openRouterDefaultBaseURL
	}

	loginCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Generate PKCE and state using oauth package
	state, err := oauth.NewState()
	if err != nil {
		return "", fmt.Errorf("openrouter: generate state: %w", err)
	}
	pkce, err := oauth.NewPKCE()
	if err != nil {
		return "", fmt.Errorf("openrouter: generate PKCE: %w", err)
	}

	// Start loopback listener
	listener, err := oauth.NewLoopbackListener(state)
	if err != nil {
		return "", fmt.Errorf("openrouter: start listener: %w", err)
	}
	defer listener.Close()

	redirectURI := listener.RedirectURI()
	authURL := fmt.Sprintf("%s%s?response_type=code&client_id=cli_mate&redirect_uri=%s&state=%s&code_challenge=%s&code_challenge_method=S256&scope=openid+profile+email",
		baseURL, openRouterAuthPath, url.QueryEscape(redirectURI), url.QueryEscape(state), url.QueryEscape(pkce.Challenge))

	fmt.Fprintf(opts.Out, "Open this URL to authorize with OpenRouter:\n  %s\n", authURL)
	if opts.OpenBrowser != nil {
		_ = opts.OpenBrowser(authURL)
	}

	// Wait for the callback code
	code, err := listener.Wait(loginCtx)
	if err != nil {
		return "", fmt.Errorf("openrouter: wait for callback: %w", err)
	}

	// Exchange the code for an API key
	apiKey, err := openRouterExchange(loginCtx, opts.HTTPClient, baseURL, code, pkce.Verifier)
	if err != nil {
		return "", fmt.Errorf("openrouter: exchange code: %w", err)
	}

	return apiKey, nil
}

// openRouterExchange exchanges the authorization code for an API key.
func openRouterExchange(ctx context.Context, client *http.Client, baseURL, code, codeVerifier string) (string, error) {
	exchangeURL := baseURL + openRouterKeyExchangeAPI

	form := url.Values{}
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("code_challenge_method", "S256")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exchangeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter: key exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openrouter: key exchange returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var keyResp openRouterKeyResponse
	if err := json.Unmarshal(body, &keyResp); err != nil {
		return "", fmt.Errorf("openrouter: parse key response: %w", err)
	}
	if keyResp.Key == "" {
		return "", fmt.Errorf("openrouter: key exchange returned empty key")
	}

	return keyResp.Key, nil
}


