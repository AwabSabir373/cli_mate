package providermodelcatalog

// This file provides remote catalog fetching from models.dev and other sources.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	modelsDevEndpoint = "https://models.dev/api/v1"
	fetchTimeout      = 10 * time.Second
	maxResponseBytes  = 1 << 20 // 1 MiB
)

// FetchRemote attempts to fetch a remote model catalog for a provider.
// Returns nil if the remote is unavailable or the provider is not found.
func FetchRemote(ctx context.Context, providerID string) ([]Model, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/providers/%s/models", modelsDevEndpoint, providerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote catalog returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}

	var remoteModels []struct {
		ID           string  `json:"id"`
		Description  string  `json:"description,omitempty"`
		ContextWindow int    `json:"contextWindow,omitempty"`
		InputCost    float64 `json:"inputCost,omitempty"`
		OutputCost   float64 `json:"outputCost,omitempty"`
	}

	if err := json.Unmarshal(body, &remoteModels); err != nil {
		return nil, err
	}

	var models []Model
	for _, rm := range remoteModels {
		if !IsCodingModel(rm.ID) && !IsCodingModel(rm.Description) {
			continue
		}
		models = append(models, Model{
			ID:            rm.ID,
			Description:   rm.Description,
			ContextWindow: rm.ContextWindow,
			InputCost:     rm.InputCost,
			OutputCost:    rm.OutputCost,
			Source:        "remote",
		})
	}

	return models, nil
}
