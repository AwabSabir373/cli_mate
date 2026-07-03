package openai_compat

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"

	"cli_mate/internal/providers/contracts"
)

// StreamDelta represents a single chunk from an OpenAI-compatible SSE stream.
type StreamDelta struct {
	Choices []struct {
		Delta struct {
			Content   string          `json:"content"`
			ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type ToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// ErrorResponse is the standard OpenAI-compatible error format.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ParseStream reads an OpenAI-compatible SSE stream and emits StreamEvents.
// It handles both content deltas and tool call deltas.
func ParseStream(ctx context.Context, body io.Reader) <-chan contracts.StreamEvent {
	ch := make(chan contracts.StreamEvent, 16)
	go func() {
		defer close(ch)

		// If the body supports Close (io.ReadCloser), close it when the context
		// is cancelled so that scanner.Scan() unblocks instead of hanging forever.
		if closer, ok := body.(io.ReadCloser); ok {
			go func() {
				<-ctx.Done()
				closer.Close()
			}()
		}

		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		// Track tool calls across chunks
		toolCalls := map[int]*contracts.ToolCall{}

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
			if data == "[DONE]" {
				// Emit accumulated tool calls if any
				if len(toolCalls) > 0 {
					emit := make([]contracts.ToolCall, 0, len(toolCalls))
					for i := 0; i < len(toolCalls); i++ {
						if tc, ok := toolCalls[i]; ok {
							emit = append(emit, *tc)
						}
					}
					ch <- contracts.StreamEvent{ToolCalls: emit}
				}
				return
			}

			var chunk StreamDelta
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				// Content delta
				if choice.Delta.Content != "" {
					ch <- contracts.StreamEvent{Delta: choice.Delta.Content}
				}

				// Tool call deltas
				for _, tc := range choice.Delta.ToolCalls {
					existing, ok := toolCalls[tc.Index]
					if !ok {
						existing = &contracts.ToolCall{
							ID:   tc.ID,
							Name: tc.Function.Name,
						}
						toolCalls[tc.Index] = existing
					}
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						existing.Arguments += tc.Function.Arguments
					}
				}
			}
		}
		if ctx.Err() != nil {
			ch <- contracts.StreamEvent{Err: ctx.Err()}
			return
		}
		if err := scanner.Err(); err != nil {
			ch <- contracts.StreamEvent{Err: err}
		}
	}()
	return ch
}

// FormatToolDefinitions converts tool definitions to OpenAI format.
func FormatToolDefinitions(tools []contracts.ToolDefinition) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Schema,
			},
		})
	}
	return result
}
