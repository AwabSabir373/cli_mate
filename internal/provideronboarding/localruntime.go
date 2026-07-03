package provideronboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cli_mate/internal/config"
)

// LocalRuntime represents a potential local AI model server.
type LocalRuntime struct {
	ID      string
	Name    string
	BaseURL string
}

// DetectedLocalRuntime extends LocalRuntime with reachability and model info.
type DetectedLocalRuntime struct {
	LocalRuntime
	Reachable bool
	Models    []string
}

// LocalDetectOptions configures local runtime detection.
type LocalDetectOptions struct {
	HTTPClient *http.Client
	Timeout    time.Duration
}

// localRuntimeCandidates returns the built-in list of local runtimes to probe.
func localRuntimeCandidates() []LocalRuntime {
	return []LocalRuntime{
		{ID: "ollama", Name: "Ollama", BaseURL: "http://localhost:11434"},
		{ID: "lmstudio", Name: "LM Studio", BaseURL: "http://localhost:1234/v1"},
	}
}

// DetectLocalRuntimes probes known local endpoints and returns reachable ones.
func DetectLocalRuntimes(ctx context.Context, opts LocalDetectOptions) []DetectedLocalRuntime {
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Second
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: opts.Timeout}
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	candidates := localRuntimeCandidates()
	var results []DetectedLocalRuntime

	for _, candidate := range candidates {
		detected := probeLocalRuntime(ctx, opts.HTTPClient, candidate)
		results = append(results, detected)
	}

	return results
}

// probeLocalRuntime checks if a local runtime is reachable and fetches its models.
func probeLocalRuntime(ctx context.Context, client *http.Client, runtime LocalRuntime) DetectedLocalRuntime {
	detected := DetectedLocalRuntime{LocalRuntime: runtime}
	modelsEndpoint := fmt.Sprintf("%s%s", strings.TrimRight(runtime.BaseURL, "/"), localModelsPath(runtime.ID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		return detected
	}

	resp, err := client.Do(req)
	if err != nil {
		return detected
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return detected
	}

	detected.Reachable = true
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))

	if runtime.ID == "ollama" {
		var ollamaResp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &ollamaResp) == nil {
			for _, m := range ollamaResp.Models {
				detected.Models = append(detected.Models, m.Name)
			}
		}
	} else {
		var lmResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &lmResp) == nil {
			for _, m := range lmResp.Data {
				detected.Models = append(detected.Models, m.ID)
			}
		}
	}

	return detected
}

// localModelsPath returns the models endpoint path for a given runtime.
func localModelsPath(runtimeID string) string {
	switch runtimeID {
	case "ollama":
		return "/api/tags"
	default:
		return "/models"
	}
}

// SetupActionForRuntime generates an onboarding Action for a detected local runtime.
func SetupActionForRuntime(runtime DetectedLocalRuntime) Action {
	return Action{
		Label: fmt.Sprintf("Use %s", runtime.Name),
		Command: fmt.Sprintf("cli_mate /provider %s", runtime.ID),
		Detail: fmt.Sprintf("%s detected at %s with %d models available. No API key required.",
			runtime.Name, runtime.BaseURL, len(runtime.Models)),
	}
}

// EnsureLocalProviderProfile creates or updates a config profile for a detected local runtime.
func EnsureLocalProviderProfile(cfg *config.Config, runtime DetectedLocalRuntime) {
	profile := cfg.Profiles[cfg.ActiveProfile]
	if profile.Provider == "" {
		profile.Provider = runtime.ID
		profile.BaseURL = runtime.BaseURL
		if len(runtime.Models) > 0 {
			profile.Model = runtime.Models[0]
		} else {
			switch runtime.ID {
			case "ollama":
				profile.Model = "llama3.1"
			case "lmstudio":
				profile.Model = "local model"
			}
		}
		cfg.Profiles[cfg.ActiveProfile] = profile
	}
}
