package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cli_mate/internal/providers/contracts"
	"cli_mate/internal/providers/openai_compat"
	"cli_mate/pkg/crypto"
	"cli_mate/pkg/httpclient"
)

// Client is a generic OpenAI-compatible provider that accepts any base URL,
// model, and optional API key. This lets users connect to any service that
// exposes an OpenAI-compatible chat completions API (e.g. LiteLLM, Together
// AI, Fireworks, local proxies, etc.).
type Client struct {
	apiKey  string
	baseURL string
	http    *httpclient.Client
}

// New creates a new custom provider client with the given base URL, API key,
// and HTTP client. The API key may be empty for local/unauthenticated endpoints.
// If the base URL already ends with /chat/completions, the suffix is stripped
// so that the client doesn't double-append it when making requests.
func New(baseURL, apiKey string, http *httpclient.Client) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    http,
	}
}

func (c *Client) Name() string {
	return "custom"
}

func (c *Client) StreamChat(ctx context.Context, req contracts.ChatRequest) (<-chan contracts.StreamEvent, error) {
	body := map[string]any{
		"model":       req.Model,
		"messages":    formatMessages(req.Messages),
		"stream":      true,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		body["tools"] = openai_compat.FormatToolDefinitions(req.Tools)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode custom request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create custom request: %w", err)
	}

	// Decrypt API key JIT, immediately before network dispatch
	if c.apiKey != "" {
		keyBytes, keyErr := crypto.DecryptIfNeededBytes(c.apiKey)
		if keyErr != nil {
			return nil, fmt.Errorf("decrypt custom api key: %w", keyErr)
		}
		httpReq.Header.Set("Authorization", "Bearer "+string(keyBytes))
		crypto.ZeroBytes(keyBytes)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Stream(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("call custom endpoint: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError(c.baseURL, resp)
	}
	return openai_compat.ParseStream(ctx, resp.Body), nil
}

type chatMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []toolCallMessage `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Name       string            `json:"name,omitempty"`
}

type toolCallMessage struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func formatMessages(msgs []contracts.Message) []chatMessage {
	result := make([]chatMessage, 0, len(msgs))
	for _, m := range msgs {
		msg := chatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.Role == "tool" {
			msg.ToolCallID = m.ToolCallID
			msg.Name = m.Name
		}
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]toolCallMessage, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				call := toolCallMessage{ID: tc.ID, Type: "function"}
				call.Function.Name = tc.Name
				call.Function.Arguments = tc.Arguments
				msg.ToolCalls = append(msg.ToolCalls, call)
			}
		}
		result = append(result, msg)
	}
	return result
}

func responseError(baseURL string, resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("custom endpoint %s returned status %d", baseURL, resp.StatusCode)
	}
	var errResp openai_compat.ErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("custom endpoint %s error (%d): %s", baseURL, resp.StatusCode, errResp.Error.Message)
	}
	return fmt.Errorf("custom endpoint %s returned status %d: %s", baseURL, resp.StatusCode, truncate(string(body), 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
