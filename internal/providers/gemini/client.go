package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	return "gemini"
}

func (c *Client) StreamChat(ctx context.Context, req contracts.ChatRequest) (<-chan contracts.StreamEvent, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("gemini api key is required")
	}
	body, err := json.Marshal(geminiRequest(req))
	if err != nil {
		return nil, fmt.Errorf("encode gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", url.PathEscape(req.Model), url.QueryEscape(c.apiKey))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Stream(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("call gemini: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, responseError("gemini", resp)
	}
	return streamGemini(ctx, resp.Body), nil
}

type generateContentRequest struct {
	Contents          []geminiContent      `json:"contents"`
	SystemInstruction *geminiContent       `json:"system_instruction,omitempty"`
	GenerationConfig  geminiGenerateConfig `json:"generationConfig,omitempty"`
}

type geminiGenerateConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiChunk struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func geminiRequest(req contracts.ChatRequest) generateContentRequest {
	out := generateContentRequest{
		Contents: make([]geminiContent, 0, len(req.Messages)),
		GenerationConfig: geminiGenerateConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: outputTokenLimit(req.MaxTokens),
		},
	}
	var systemParts []geminiPart
	for _, message := range req.Messages {
		if message.Role == "system" {
			systemParts = append(systemParts, geminiPart{Text: message.Content})
			continue
		}

		role := "user"
		if message.Role == "assistant" {
			role = "model"
		}

		if len(out.Contents) == 0 && role == "model" {
			// Gemini requires the first message to be user
			out.Contents = append(out.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: "(context trimmed)"}},
			})
		}

		if len(out.Contents) > 0 && out.Contents[len(out.Contents)-1].Role == role {
			// Collapse consecutive messages with the same role
			lastIdx := len(out.Contents) - 1
			out.Contents[lastIdx].Parts = append(out.Contents[lastIdx].Parts, geminiPart{Text: "\n\n" + message.Content})
		} else {
			out.Contents = append(out.Contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: message.Content}},
			})
		}
	}
	if len(systemParts) > 0 {
		out.SystemInstruction = &geminiContent{Parts: systemParts}
	}
	return out
}

func streamGemini(ctx context.Context, body io.ReadCloser) <-chan contracts.StreamEvent {
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

			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var chunk geminiChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				events <- contracts.StreamEvent{Err: fmt.Errorf("decode gemini stream chunk: %w", err)}
				return
			}
			if chunk.Error != nil {
				events <- contracts.StreamEvent{Err: errors.New(chunk.Error.Message)}
				return
			}
			for _, candidate := range chunk.Candidates {
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						events <- contracts.StreamEvent{Delta: part.Text}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			events <- contracts.StreamEvent{Err: fmt.Errorf("read gemini stream: %w", err)}
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
