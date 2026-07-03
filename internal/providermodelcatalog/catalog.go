// Package providermodelcatalog provides a curated registry of AI models
// and remote catalog fetching capabilities.
package providermodelcatalog

import "strings"

// Model represents an AI model with its metadata.
type Model struct {
	ID             string
	Description    string
	ContextWindow  int
	MaxOutput      int
	InputCost      float64
	OutputCost     float64
	SupportsTools  bool
	SupportsImages bool
	Source         string // "curated", "remote", "live"
}

// curatedModels is the central registry of recommended models per provider.
var curatedModels = map[string][]Model{
	"openai": {
		{ID: "gpt-4.1", Description: "GPT-4.1, best-in-class coding", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
		{ID: "gpt-4.1-mini", Description: "GPT-4.1 Mini, fast & efficient", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
		{ID: "gpt-4.1-nano", Description: "GPT-4.1 Nano, fastest & cheapest", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
		{ID: "gpt-4o", Description: "GPT-4o, multimodal flagship", ContextWindow: 128000, SupportsTools: true, SupportsImages: true},
		{ID: "o3-mini", Description: "o3-mini, reasoning model", ContextWindow: 200000, SupportsTools: true, SupportsImages: false},
	},
	"anthropic": {
		{ID: "claude-sonnet-4-20250514", Description: "Claude Sonnet 4, best balance", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
		{ID: "claude-3-5-sonnet-20241022", Description: "Claude 3.5 Sonnet, previous gen", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
		{ID: "claude-3-5-haiku-20241022", Description: "Claude 3.5 Haiku, fast & cheap", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
		{ID: "claude-3-opus-20240229", Description: "Claude 3 Opus, most capable", ContextWindow: 200000, SupportsTools: true, SupportsImages: true},
	},
	"gemini": {
		{ID: "gemini-2.5-flash", Description: "Gemini 2.5 Flash, fast", ContextWindow: 1048576, SupportsTools: true, SupportsImages: true},
		{ID: "gemini-2.5-pro", Description: "Gemini 2.5 Pro, most capable", ContextWindow: 1048576, SupportsTools: true, SupportsImages: true},
		{ID: "gemini-2.0-flash", Description: "Gemini 2.0 Flash", ContextWindow: 1048576, SupportsTools: true, SupportsImages: true},
	},
	"groq": {
		{ID: "llama-3.3-70b-versatile", Description: "Llama 3.3 70B, versatile", ContextWindow: 131072, SupportsTools: true},
		{ID: "llama-3.1-8b-instant", Description: "Llama 3.1 8B, fast", ContextWindow: 131072, SupportsTools: true},
		{ID: "mixtral-8x7b-32768", Description: "Mixtral 8x7B MoE", ContextWindow: 32768, SupportsTools: true},
		{ID: "deepseek-r1-distill-llama-70b", Description: "DeepSeek R1 distilled", ContextWindow: 131072, SupportsTools: false},
	},
	"ollama": {
		{ID: "llama3.1", Description: "Llama 3.1 8B", ContextWindow: 8192},
		{ID: "llama3.2", Description: "Llama 3.2 3B", ContextWindow: 8192},
		{ID: "qwen2.5-coder", Description: "Qwen 2.5 Coder", ContextWindow: 32768, SupportsTools: true},
		{ID: "deepseek-coder", Description: "DeepSeek Coder", ContextWindow: 16384, SupportsTools: true},
		{ID: "codellama", Description: "Code Llama", ContextWindow: 16384},
	},
	"openrouter": {
		{ID: "openai/gpt-4.1-mini", ContextWindow: 200000, SupportsTools: true},
		{ID: "anthropic/claude-sonnet-4-20250514", ContextWindow: 200000, SupportsTools: true},
		{ID: "google/gemini-2.5-flash", ContextWindow: 1048576, SupportsTools: true},
		{ID: "meta-llama/llama-3.3-70b-instruct", ContextWindow: 131072, SupportsTools: true},
	},
	"deepseek": {
		{ID: "deepseek-chat", Description: "DeepSeek Chat", ContextWindow: 64000, SupportsTools: true},
		{ID: "deepseek-coder", Description: "DeepSeek Coder", ContextWindow: 128000, SupportsTools: true},
		{ID: "deepseek-reasoner", Description: "DeepSeek Reasoner", ContextWindow: 64000, SupportsTools: false},
	},
	"mistral": {
		{ID: "mistral-large-latest", Description: "Mistral Large", ContextWindow: 128000, SupportsTools: true},
		{ID: "mistral-small-latest", Description: "Mistral Small", ContextWindow: 32000, SupportsTools: true},
		{ID: "codestral-latest", Description: "Codestral", ContextWindow: 32000, SupportsTools: true},
	},
	"xai": {
		{ID: "grok-3", Description: "Grok 3", ContextWindow: 131072, SupportsTools: true},
		{ID: "grok-3-mini", Description: "Grok 3 Mini", ContextWindow: 131072, SupportsTools: true},
	},
	"lmstudio": {
		{ID: "local model", Description: "Locally loaded model", ContextWindow: 8192},
	},
}

// Models returns the list of curated models for a provider.
func Models(provider string) []Model {
	models, ok := curatedModels[provider]
	if !ok {
		return nil
	}
	result := make([]Model, len(models))
	copy(result, models)
	return result
}

// DefaultModel returns the recommended default model for a provider.
func DefaultModel(provider string) string {
	models := Models(provider)
	if len(models) == 0 {
		return ""
	}
	return models[0].ID
}

// IsCodingModel checks if a model ID looks like a coding-capable model.
func IsCodingModel(id string) bool {
	// Filter out known non-coding models
	if isKnownNonCoding(id) {
		return false
	}
	return looksLikeCodingModel(id)
}

func isKnownNonCoding(id string) bool {
	nonCoding := []string{
		"dall-e", "sora", "whisper", "tts", "embedding",
		"davinci", "babbage", "curie", "audio", "moderation",
		"stable-diffusion", "midjourney",
	}
	for _, s := range nonCoding {
		if containsFold(id, s) {
			return true
		}
	}
	return false
}

func looksLikeCodingModel(id string) bool {
	codingKeywords := []string{
		"gpt", "claude", "gemini", "llama", "coder", "code",
		"deepseek", "mistral", "mixtral", "qwen", "phi",
		"codestral", "grok", "o1", "o3", "reasoning",
		"instruct", "sonnet", "haiku", "opus", "flash",
	}
	for _, kw := range codingKeywords {
		if containsFold(id, kw) {
			return true
		}
	}
	return false
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
