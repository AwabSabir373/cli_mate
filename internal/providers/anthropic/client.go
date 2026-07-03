package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cli_mate/internal/providers/contracts"
	"cli_mate/pkg/httpclient"
)

type Client struct {
	apiKey  string
	baseURL string
	http    *httpclient.Client
}

func New(apiKey string, http *httpclient.Client) *Client {
	return &Client{apiKey: apiKey, baseURL: "https://api.anthropic.com", http: http}
}

func NewWithBaseURL(apiKey, baseURL string, http *httpclient.Client) *Client {
	return &Client{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), http: http}
}

func (c *Client) Name() string {
	return "anthropic"
}

func (c *Client) StreamChat(ctx context.Context, req contracts.ChatRequest) (<-chan contracts.StreamEvent, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}

	body := buildRequest(req)
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Stream(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("call anthropic: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError(resp)
	}
	return parseStream(ctx, resp.Body), nil
}

type anthropicRequest struct {
	Model       string          `json:"model"`
	Messages    []anthropicMsg  `json:"messages"`
	System      string          `json:"system,omitempty"`
	MaxTokens   int             `json:"max_tokens"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	Tools       []anthropicTool `json:"tools,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

func buildRequest(req contracts.ChatRequest) anthropicRequest {
	msgs := make([]anthropicMsg, 0, len(req.Messages))
	var system string

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		if m.Role == "tool" {
			msgs = append(msgs, anthropicMsg{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
					IsError:   strings.HasPrefix(m.Content, "Error:") || strings.Contains(m.Content, "\nError:"),
				}},
			})
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		content := any(m.Content)
		if len(m.ToolCalls) > 0 {
			blocks := make([]anthropicContentBlock, 0, len(m.ToolCalls)+1)
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := map[string]any{}
				if strings.TrimSpace(tc.Arguments) != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &input)
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			content = blocks
		}
		msgs = append(msgs, anthropicMsg{Role: role, Content: content})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	if maxTokens > 8192 {
		maxTokens = 8192
	}

	r := anthropicRequest{
		Model:     req.Model,
		Messages:  msgs,
		System:    system,
		MaxTokens: maxTokens,
		Stream:    true,
	}
	if req.Temperature > 0 {
		t := req.Temperature
		r.Temperature = &t
	}

	if len(req.Tools) > 0 {
		tools := make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Schema,
			})
		}
		r.Tools = tools
	}

	return r
}

// Anthropic SSE event types
type streamEvent struct {
	Type         string          `json:"type"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
	Index        int             `json:"index,omitempty"`
}

type contentDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type toolUseDelta struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input string `json:"input,omitempty"`
}

type contentBlockStart struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func parseStream(ctx context.Context, body io.Reader) <-chan contracts.StreamEvent {
	ch := make(chan contracts.StreamEvent, 16)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var currentToolID string
		var currentToolName string
		var toolInput strings.Builder

		for scanner.Scan() {
			if ctx.Err() != nil {
				ch <- contracts.StreamEvent{Err: ctx.Err()}
				return
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event streamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_start":
				var block contentBlockStart
				if err := json.Unmarshal(event.ContentBlock, &block); err == nil && block.Type == "tool_use" {
					currentToolID = block.ID
					currentToolName = block.Name
					toolInput.Reset()
				}

			case "content_block_delta":
				var delta contentDelta
				if err := json.Unmarshal(event.Delta, &delta); err == nil {
					switch delta.Type {
					case "text_delta":
						if delta.Text != "" {
							ch <- contracts.StreamEvent{Delta: delta.Text}
						}
					case "input_json_delta":
						// Tool input is streamed as JSON fragments
						var inputDelta struct {
							Type        string `json:"type"`
							PartialJSON string `json:"partial_json,omitempty"`
						}
						if err := json.Unmarshal(event.Delta, &inputDelta); err == nil {
							toolInput.WriteString(inputDelta.PartialJSON)
						}
					}
				}

			case "content_block_stop":
				if currentToolName != "" {
					ch <- contracts.StreamEvent{
						ToolCalls: []contracts.ToolCall{{
							ID:        currentToolID,
							Name:      currentToolName,
							Arguments: toolInput.String(),
						}},
					}
					currentToolID = ""
					currentToolName = ""
					toolInput.Reset()
				}

			case "message_stop":
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- contracts.StreamEvent{Err: err}
		}
	}()
	return ch
}

func responseError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("anthropic returned status %d", resp.StatusCode)
	}
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return fmt.Errorf("anthropic error (%s): %s", errResp.Error.Type, errResp.Error.Message)
	}
	return fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, truncate(string(body), 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
