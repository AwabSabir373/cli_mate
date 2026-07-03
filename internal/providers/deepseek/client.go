package deepseek

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
	"cli_mate/pkg/httpclient"
)

type Client struct {
	apiKey  string
	baseURL string
	http    *httpclient.Client
}

func New(apiKey string, http *httpclient.Client) *Client {
	return &Client{apiKey: apiKey, baseURL: "https://api.deepseek.com/v1", http: http}
}

func NewWithBaseURL(apiKey, baseURL string, http *httpclient.Client) *Client {
	return &Client{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), http: http}
}

func (c *Client) Name() string {
	return "deepseek"
}

func (c *Client) StreamChat(ctx context.Context, req contracts.ChatRequest) (<-chan contracts.StreamEvent, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("deepseek api key is required")
	}

	msgs := formatMessages(req.Messages)
	body := map[string]any{
		"model":       req.Model,
		"messages":    msgs,
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
		return nil, fmt.Errorf("encode deepseek request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create deepseek request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Stream(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("call deepseek: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError(resp)
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

func responseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("deepseek returned status %d", resp.StatusCode)
	}
	var errResp openai_compat.ErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("deepseek error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}
	return fmt.Errorf("deepseek returned status %d: %s", resp.StatusCode, truncate(string(body), 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
