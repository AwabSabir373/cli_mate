package registry

import (
	"fmt"

	"cli_mate/internal/config"
	"cli_mate/internal/providers/anthropic"
	"cli_mate/internal/providers/contracts"
	"cli_mate/internal/providers/custom"
	"cli_mate/internal/providers/deepseek"
	"cli_mate/internal/providers/gemini"
	"cli_mate/internal/providers/groq"
	"cli_mate/internal/providers/lmstudio"
	"cli_mate/internal/providers/mistral"
	"cli_mate/internal/providers/ollama"
	"cli_mate/internal/providers/openai"
	"cli_mate/internal/providers/openrouter"
	"cli_mate/internal/providers/xai"
	"cli_mate/pkg/httpclient"
)

type Spec struct {
	Name           string
	DefaultModel   string
	Models         []string
	RequiresKey    bool
	DefaultBaseURL string
}

var specs = []Spec{
	{
		Name:         "openai",
		DefaultModel: "gpt-4.1-mini",
		Models:       []string{"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "gpt-4o", "gpt-4o-mini", "o3-mini"},
		RequiresKey:  true,
	},
	{
		Name:         "anthropic",
		DefaultModel: "claude-sonnet-4-20250514",
		Models:       []string{"claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022", "claude-3-opus-20240229"},
		RequiresKey:  true,
	},
	{
		Name:         "openrouter",
		DefaultModel: "openai/gpt-4.1-mini",
		Models:       []string{"openai/gpt-4.1-mini", "anthropic/claude-3.5-sonnet", "google/gemini-2.5-flash", "meta-llama/llama-3.3-70b-instruct"},
		RequiresKey:  true,
	},
	{
		Name:         "gemini",
		DefaultModel: "gemini-2.5-flash",
		Models:       []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-1.5-flash", "gemini-1.5-pro"},
		RequiresKey:  true,
	},
	{
		Name:         "groq",
		DefaultModel: "llama-3.3-70b-versatile",
		Models:       []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "deepseek-r1-distill-llama-70b", "mixtral-8x7b-32768"},
		RequiresKey:  true,
	},
	{
		Name:           "ollama",
		DefaultModel:   "llama3.1",
		Models:         []string{"llama3.1", "llama3.2", "qwen2.5-coder", "deepseek-coder", "codellama"},
		DefaultBaseURL: "http://localhost:11434",
	},
	{
		Name:         "deepseek",
		DefaultModel: "deepseek-chat",
		Models:       []string{"deepseek-chat", "deepseek-coder", "deepseek-reasoner"},
		RequiresKey:  true,
	},
	{
		Name:         "mistral",
		DefaultModel: "mistral-large-latest",
		Models:       []string{"mistral-large-latest", "mistral-small-latest", "codestral-latest"},
		RequiresKey:  true,
	},
	{
		Name:           "lmstudio",
		DefaultModel:   "local model",
		Models:         []string{"local model"},
		RequiresKey:    false,
		DefaultBaseURL: "http://localhost:1234/v1",
	},
	{
		Name:         "xai",
		DefaultModel: "grok-3-mini",
		Models:       []string{"grok-2", "grok-3", "grok-3-mini"},
		RequiresKey:  true,
	},
	{
		Name:         "custom",
		DefaultModel: "gpt-4.1-mini",
		Models:       []string{"custom"},
		RequiresKey:  false,
	},
}

func Specs() []Spec {
	out := make([]Spec, len(specs))
	copy(out, specs)
	return out
}

func SpecByName(name string) (Spec, bool) {
	for _, spec := range specs {
		if spec.Name == name {
			return spec, true
		}
	}
	return Spec{}, false
}

func Models(provider string) []string {
	spec, ok := SpecByName(provider)
	if !ok {
		return nil
	}
	out := make([]string, len(spec.Models))
	copy(out, spec.Models)
	return out
}

func New(profile config.Profile, http *httpclient.Client) (contracts.Provider, error) {
	switch profile.Provider {
	case "openai":
		return openai.New(profile.APIKey, http), nil
	case "anthropic":
		return anthropic.New(profile.APIKey, http), nil
	case "openrouter":
		return openrouter.New(profile.APIKey, http), nil
	case "gemini":
		return gemini.New(profile.APIKey, http), nil
	case "groq":
		return groq.New(profile.APIKey, http), nil
	case "deepseek":
		return deepseek.New(profile.APIKey, http), nil
	case "mistral":
		return mistral.New(profile.APIKey, http), nil
	case "lmstudio":
		baseURL := profile.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:1234/v1"
		}
		return lmstudio.New(baseURL, http), nil
	case "xai":
		return xai.New(profile.APIKey, http), nil
	case "custom":
		baseURL := profile.BaseURL
		if baseURL == "" {
			return nil, fmt.Errorf("custom provider requires a base URL; use /base-url <url>")
		}
		return custom.New(baseURL, profile.APIKey, http), nil
	case "ollama":
		baseURL := profile.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return ollama.New(baseURL, http), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", profile.Provider)
	}
}

func Validate(profile config.Profile) error {
	spec, ok := SpecByName(profile.Provider)
	if !ok {
		return fmt.Errorf("unknown provider %q", profile.Provider)
	}
	if profile.Model == "" {
		return fmt.Errorf("model is required for provider %q", profile.Provider)
	}
	if spec.RequiresKey && profile.APIKey == "" {
		return fmt.Errorf("%s api key is required; choose /provider %s and paste the key during setup", profile.Provider, profile.Provider)
	}
	if profile.Provider == "ollama" && profile.BaseURL == "" {
		return fmt.Errorf("ollama base url is required; use /base-url http://localhost:11434")
	}
	if profile.Provider == "custom" && profile.BaseURL == "" {
		return fmt.Errorf("custom provider requires a base URL; use /base-url <url>")
	}
	return nil
}
