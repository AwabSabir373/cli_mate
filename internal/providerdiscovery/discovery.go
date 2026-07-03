// Package providerdiscovery auto-detects local AI runtimes like Ollama and LM Studio.
package providerdiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var (
	ollamaDefaultURL = "http://localhost:11434"
	lmstudioURL      = "http://localhost:1234/v1"
	discoveryTimeout = 2 * time.Second
)

// DiscoveredProvider represents a locally running AI provider.
type DiscoveredProvider struct {
	Name        string   `json:"name"`
	BaseURL     string   `json:"baseURL"`
	Models      []string `json:"models"`
	RequiresKey bool     `json:"requiresKey"`
}

// Discover scans common local ports for running AI providers.
func Discover(ctx context.Context) []DiscoveredProvider {
	var providers []DiscoveredProvider

	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	// Check Ollama
	if p, ok := checkOllama(ctx); ok {
		providers = append(providers, p)
	}

	// Check LM Studio
	if p, ok := checkLMStudio(ctx); ok {
		providers = append(providers, p)
	}

	return providers
}

func checkOllama(ctx context.Context) (DiscoveredProvider, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaDefaultURL+"/api/tags", nil)
	if err != nil {
		return DiscoveredProvider{}, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DiscoveredProvider{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DiscoveredProvider{}, false
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DiscoveredProvider{}, false
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}

	return DiscoveredProvider{
		Name:        "ollama",
		BaseURL:     ollamaDefaultURL,
		Models:      models,
		RequiresKey: false,
	}, true
}

func checkLMStudio(ctx context.Context) (DiscoveredProvider, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lmstudioURL+"/models", nil)
	if err != nil {
		return DiscoveredProvider{}, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DiscoveredProvider{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DiscoveredProvider{}, false
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DiscoveredProvider{}, false
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return DiscoveredProvider{
		Name:        "lmstudio",
		BaseURL:     lmstudioURL,
		Models:      models,
		RequiresKey: false,
	}, true
}

// FormatProviders returns a human-readable list of discovered providers.
func FormatProviders(providers []DiscoveredProvider) string {
	if len(providers) == 0 {
		return "No local providers detected."
	}

	var result string
	for _, p := range providers {
		result += fmt.Sprintf("%s at %s (%d models available)\n", p.Name, p.BaseURL, len(p.Models))
	}
	return result
}
