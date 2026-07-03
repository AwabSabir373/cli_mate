package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	// pkceVerifierBytes yields a 43-character base64url verifier.
	pkceVerifierBytes = 32
	stateBytes        = 32
	// MethodS256 is the only code-challenge method this engine accepts.
	MethodS256 = "S256"
)

// ErrPKCEDowngrade is returned when a non-S256 PKCE method is used.
var ErrPKCEDowngrade = errors.New("oauth: PKCE method must be S256")

// ErrStateMismatch is returned when the OAuth state parameter does not match.
var ErrStateMismatch = errors.New("oauth: CSRF state mismatch")

// ErrInsecureTokenEndpoint is returned when a token endpoint is not HTTPS.
var ErrInsecureTokenEndpoint = errors.New("oauth: token endpoint must be HTTPS")

// ErrNoRefreshToken is returned when no refresh token is available.
var ErrNoRefreshToken = errors.New("oauth: no refresh token available")

// ErrAuthorizationPending is returned by device grant polling.
var ErrAuthorizationPending = errors.New("oauth: authorization pending")

// ErrSlowDown is returned when the device grant polling should slow down.
var ErrSlowDown = errors.New("oauth: slow down polling interval")

// Token represents an OAuth 2.0 token response.
type Token struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	TokenType    string `json:"tokenType,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // Unix timestamp
	Scopes       string `json:"scopes,omitempty"`
	IDToken      string `json:"idToken,omitempty"`
	Account      string `json:"account,omitempty"`
}

// Expired reports whether the token has expired.
func (t Token) Expired(now int64) bool {
	return t.ExpiresAt > 0 && now >= t.ExpiresAt
}

// NeedsRefresh reports whether the token should be refreshed.
func (t Token) NeedsRefresh(now int64, buffer int64) bool {
	if t.ExpiresAt == 0 {
		return false
	}
	return now+buffer >= t.ExpiresAt
}

// Config configures an OAuth 2.0 authorization server interaction.
type Config struct {
	ClientID                   string
	ClientSecret               string
	AuthorizationEndpoint      string
	TokenEndpoint              string
	DeviceAuthorizationEndpoint string
	IssuerURL                  string
	Scopes                     []string
	ExtraAuthParams            map[string]string
}

// PKCE holds a verifier/challenge pair for authorization-code flow.
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

// NewPKCE generates a high-entropy code verifier and its S256 challenge.
func NewPKCE() (PKCE, error) {
	raw := make([]byte, pkceVerifierBytes)
	if _, err := rand.Read(raw); err != nil {
		return PKCE{}, fmt.Errorf("oauth: generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge, Method: MethodS256}, nil
}

// NewState generates a high-entropy CSRF state value.
func NewState() (string, error) {
	raw := make([]byte, stateBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// trimmed returns s with whitespace trimmed.
func trimmed(s string) string {
	return strings.TrimSpace(s)
}
