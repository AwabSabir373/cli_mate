package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const tokenResponseLimit = 1 << 20 // 1 MiB cap on token-endpoint bodies

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope"`
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// BuildAuthorizationURL constructs the authorization request URL for an authorization-code + PKCE flow.
func BuildAuthorizationURL(cfg Config, pkce PKCE, state, redirectURI string, extraParams map[string]string) (string, error) {
	if pkce.Method != MethodS256 {
		return "", ErrPKCEDowngrade
	}
	endpoint := trimmed(cfg.AuthorizationEndpoint)
	if endpoint == "" {
		return "", errors.New("oauth: no authorization endpoint configured")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("oauth: parse authorization endpoint: %w", err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("code_challenge", pkce.Challenge)
	query.Set("code_challenge_method", pkce.Method)
	if len(cfg.Scopes) > 0 {
		query.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	for k, v := range cfg.ExtraAuthParams {
		if isReservedAuthParam(k) {
			continue
		}
		query.Set(k, v)
	}
	for k, v := range extraParams {
		if isReservedAuthParam(k) {
			continue
		}
		query.Set(k, v)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func isReservedAuthParam(key string) bool {
	switch key {
	case "response_type", "client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method":
		return true
	default:
		return false
	}
}

// validateTokenEndpoint refuses to send credentials to a non-HTTPS token endpoint
// unless it is a loopback host.
func validateTokenEndpoint(endpoint string) error {
	parsed, err := url.Parse(trimmed(endpoint))
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("oauth: invalid token endpoint %q", endpoint)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInsecureTokenEndpoint, parsed.Scheme+"://"+parsed.Host)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// ExchangeCode swaps an authorization code + PKCE verifier for tokens.
func ExchangeCode(ctx context.Context, client *http.Client, cfg Config, code, verifier, redirectURI string, now func() time.Time) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", cfg.ClientID)
	form.Set("code_verifier", verifier)
	if secret := trimmed(cfg.ClientSecret); secret != "" {
		form.Set("client_secret", secret)
	}
	return PostToken(ctx, client, cfg.TokenEndpoint, form, Token{}, now)
}

// Refresh exchanges a refresh token for a fresh access token.
func Refresh(ctx context.Context, client *http.Client, cfg Config, current Token, now func() time.Time) (Token, error) {
	refresh := trimmed(current.RefreshToken)
	if refresh == "" {
		return Token{}, ErrNoRefreshToken
	}
	if trimmed(cfg.TokenEndpoint) == "" {
		return Token{}, errors.New("oauth: no token endpoint configured for refresh")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refresh)
	form.Set("client_id", cfg.ClientID)
	if secret := trimmed(cfg.ClientSecret); secret != "" {
		form.Set("client_secret", secret)
	}
	if len(cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	base := Token{Scopes: current.Scopes, RefreshToken: refresh, Account: current.Account, IDToken: current.IDToken}
	return PostToken(ctx, client, cfg.TokenEndpoint, form, base, now)
}

// PostToken performs a token-endpoint POST and maps the response onto a Token.
func PostToken(ctx context.Context, client *http.Client, tokenEndpoint string, form url.Values, base Token, now func() time.Time) (Token, error) {
	if err := validateTokenEndpoint(tokenEndpoint); err != nil {
		return Token{}, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	if now == nil {
		now = time.Now
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimmed(tokenEndpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("oauth: token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, tokenResponseLimit))
	var parsed tokenResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &parsed)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != "" {
			if parsed.ErrorDescription != "" {
				return Token{}, fmt.Errorf("oauth: token endpoint error %q: %s", parsed.Error, parsed.ErrorDescription)
			}
			return Token{}, fmt.Errorf("oauth: token endpoint error %q", parsed.Error)
		}
		return Token{}, fmt.Errorf("oauth: token endpoint returned HTTP %d", resp.StatusCode)
	}

	if trimmed(parsed.AccessToken) == "" {
		return Token{}, errors.New("oauth: token endpoint returned no access token")
	}

	token := base
	token.AccessToken = parsed.AccessToken
	if trimmed(parsed.RefreshToken) != "" {
		token.RefreshToken = parsed.RefreshToken
	}
	if trimmed(parsed.TokenType) != "" {
		token.TokenType = parsed.TokenType
	}
	if parsed.ExpiresIn > 0 {
		token.ExpiresAt = now().Add(time.Duration(parsed.ExpiresIn) * time.Second).Unix()
	}
	if scope := trimmed(parsed.Scope); scope != "" {
		token.Scopes = scope
	}
	if trimmed(parsed.IDToken) != "" {
		token.IDToken = parsed.IDToken
	}
	return token, nil
}
