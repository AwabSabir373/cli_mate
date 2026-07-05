package openrouter

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
	apiKey string
	http   *httpclient.Client
}

func New(apiKey string, http *httpclient.Client) *Client {
	return &Client{apiKey: apiKey, http: http}
}

func (c *Client) Name() string {
	return "openrouter"
}

func (c *Client) StreamChat(ctx context.Context, req contracts.ChatRequest) (<-chan contracts.StreamEvent, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("openrouter api key is required")
	}
	body, err := json.Marshal(chatCompletionRequest{
		Model:       req.Model,
		Messages:    openAICompatibleMessages(req.Messages),
		Stream:      true,
		Temperature: req.Temperature,
		MaxTokens:   omitZero(req.MaxTokens),
	})
	if err != nil {
		return nil, fmt.Errorf("encode openrouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create openrouter request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Stream(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("call openrouter: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError("openrouter", resp)
	}
	return streamOpenAICompatible(ctx, resp.Body), nil
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type openRouterErrorResponse struct {
	Error *struct {
		Message  string `json:"message"`
		Code     int    `json:"code"`
		Metadata struct {
			Raw          string `json:"raw"`
			ProviderName string `json:"provider_name"`
		} `json:"metadata"`
	} `json:"error"`
}

func openAICompatibleMessages(messages []contracts.Message) []chatMessage {
	out := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		role := message.Role
		switch role {
		case "system", "user", "assistant", "tool":
		default:
			role = "user"
		}
		out = append(out, chatMessage{Role: role, Content: message.Content})
	}
	return out
}

func streamOpenAICompatible(ctx context.Context, body io.ReadCloser) <-chan contracts.StreamEvent {
	events := make(chan contracts.StreamEvent, 1)
	go func() {
		defer close(events)
		defer body.Close()

		// Close the body when the context is cancelled so that scanner.Scan()
		// unblocks instead of hanging forever.
		go func() {
			<-ctx.Done()
			body.Close()
		}()

		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				sendStreamEvent(ctx, events, contracts.StreamEvent{Heartbeat: true})
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				events <- contracts.StreamEvent{Done: true}
				return
			}

			var chunk chatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				events <- contracts.StreamEvent{Err: fmt.Errorf("decode stream chunk: %w", err)}
				return
			}
			if chunk.Error != nil {
				events <- contracts.StreamEvent{Err: errors.New(chunk.Error.Message)}
				return
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					events <- contracts.StreamEvent{Delta: choice.Delta.Content}
				}
			}
		}
		if ctx.Err() != nil {
			events <- contracts.StreamEvent{Err: ctx.Err()}
			return
		}
		if err := scanner.Err(); err != nil {
			events <- contracts.StreamEvent{Err: fmt.Errorf("read stream: %w", err)}
			return
		}
		events <- contracts.StreamEvent{Done: true}
	}()
	return events
}

func sendStreamEvent(ctx context.Context, events chan<- contracts.StreamEvent, event contracts.StreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}

func omitZero(value int) *int {
	if value <= 0 || value > 32768 {
		return nil
	}
	return &value
}

func responseError(provider string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("%s returned %s", provider, resp.Status)
	}
	if parsed := openRouterError(provider, resp, detail); parsed != nil {
		return parsed
	}
	return fmt.Errorf("%s returned %s: %s", provider, resp.Status, detail)
}

func openRouterError(provider string, resp *http.Response, body string) error {
	var payload openRouterErrorResponse
	if err := json.Unmarshal([]byte(body), &payload); err != nil || payload.Error == nil {
		return nil
	}

	message := strings.TrimSpace(payload.Error.Metadata.Raw)
	if message == "" {
		message = strings.TrimSpace(payload.Error.Message)
	}
	if message == "" {
		return fmt.Errorf("%s returned %s", provider, resp.Status)
	}

	prefix := fmt.Sprintf("%s returned %s", provider, resp.Status)
	if resp.StatusCode == http.StatusTooManyRequests {
		prefix = fmt.Sprintf("%s rate limit", provider)
		if payload.Error.Metadata.ProviderName != "" {
			prefix += " from " + payload.Error.Metadata.ProviderName
		}
		return fmt.Errorf("%s: %s. Try a different model with /model, wait and retry, or configure BYOK in OpenRouter", prefix, message)
	}
	return fmt.Errorf("%s: %s", prefix, message)
}
