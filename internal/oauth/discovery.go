package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const oauthWellKnownPath = "/.well-known/oauth-authorization-server"

// ServerMetadata is a subset of RFC 8414 authorization-server metadata.
type ServerMetadata struct {
	Issuer                      string   `json:"issuer"`
	AuthorizationEndpoint       string   `json:"authorization_endpoint"`
	TokenEndpoint               string   `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint"`
	RegistrationEndpoint        string   `json:"registration_endpoint"`
	ScopesSupported             []string `json:"scopes_supported"`
}

// DiscoverAuthorizationServer fetches metadata from the RFC 8414 well-known path.
func DiscoverAuthorizationServer(ctx context.Context, client *http.Client, issuerURL string) (ServerMetadata, error) {
	if client == nil {
		client = http.DefaultClient
	}
	discoveryURL := joinWellKnown(issuerURL, oauthWellKnownPath)
	return fetchMetadata(ctx, client, discoveryURL)
}

// fetchMetadata performs the HTTP GET for OAuth/OIDC metadata.
func fetchMetadata(ctx context.Context, client *http.Client, url string) (ServerMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ServerMetadata{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return ServerMetadata{}, fmt.Errorf("oauth: metadata fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, tokenResponseLimit))
	var meta ServerMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return ServerMetadata{}, fmt.Errorf("oauth: parse metadata: %w", err)
	}
	return meta, nil
}

// joinWellKnown inserts the well-known path into the issuer URL.
func joinWellKnown(baseURL, wellKnown string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	// If there's already a path, insert before it, otherwise append.
	if strings.Contains(base, "://") {
		// Count slashes after the scheme
		parts := strings.SplitN(base, "://", 2)
		if len(parts) == 2 {
			rest := parts[1]
			if idx := strings.Index(rest, "/"); idx >= 0 {
				host := rest[:idx]
				path := rest[idx:]
				return parts[0] + "://" + host + wellKnown + path
			}
			return base + wellKnown
		}
	}
	return base + wellKnown
}
