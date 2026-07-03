package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cli_mate/internal/providers/contracts"
	"cli_mate/pkg/httpclient"
)

type Client struct {
	baseURL string
	http    *httpclient.Client
}

func New(baseURL string, http *httpclient.Client) *Client {
	return &Client{baseURL: baseURL, http: http}
}

func (c *Client) Name() string {
	return "ollama"
}

func (c *Client) StreamChat(ctx context.Context, req contracts.ChatRequest) (<-chan contracts.StreamEvent, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("ollama base url is required")
	}
	body, err := json.Marshal(ollamaChatRequest{
		Model:    req.Model,
		Messages: ollamaMessages(req.Messages),
		Stream:   true,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  outputTokenLimit(req.MaxTokens),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode ollama request: %w", err)
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/x-ndjson")

	resp, err := c.http.Stream(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("call ollama: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError("ollama", resp)
	}
	return streamOllama(ctx, resp.Body), nil
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChunk struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
	Error   string        `json:"error,omitempty"`
}

func ollamaMessages(messages []contracts.Message) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(messages))
	for _, message := range messages {
		role := message.Role
		switch role {
		case "system", "user", "assistant", "tool":
		default:
			role = "user"
		}
		out = append(out, ollamaMessage{Role: role, Content: message.Content})
	}
	return out
}

func streamOllama(ctx context.Context, body io.ReadCloser) <-chan contracts.StreamEvent {
	events := make(chan contracts.StreamEvent, 1)
	go func() {
		defer close(events)
		defer body.Close()

		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				events <- contracts.StreamEvent{Err: ctx.Err()}
				return
			default:
			}

			var chunk ollamaChunk
			if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
				events <- contracts.StreamEvent{Err: fmt.Errorf("decode ollama stream chunk: %w", err)}
				return
			}
			if chunk.Error != "" {
				events <- contracts.StreamEvent{Err: errors.New(chunk.Error)}
				return
			}
			if chunk.Message.Content != "" {
				events <- contracts.StreamEvent{Delta: chunk.Message.Content}
			}
			if chunk.Done {
				events <- contracts.StreamEvent{Done: true}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			events <- contracts.StreamEvent{Err: fmt.Errorf("read ollama stream: %w", err)}
			return
		}
		events <- contracts.StreamEvent{Done: true}
	}()
	return events
}

func responseError(provider string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("%s returned %s", provider, resp.Status)
	}
	return fmt.Errorf("%s returned %s: %s", provider, resp.Status, detail)
}

func outputTokenLimit(value int) int {
	if value <= 0 || value > 32768 {
		return 0
	}
	return value
}
