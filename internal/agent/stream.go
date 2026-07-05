package agent

import (
	"context"
	"strings"

	"cli_mate/internal/providers"
)

type StreamHandler struct {
	OnToken     func(string)
	OnError     func(error)
	OnToolCalls func([]providers.ToolCall)
	OnActivity  func()
}

func (h StreamHandler) Consume(ctx context.Context, events <-chan providers.StreamEvent) (string, []providers.ToolCall, error) {
	var builder strings.Builder
	var toolCalls []providers.ToolCall
	for {
		select {
		case <-ctx.Done():
			return builder.String(), toolCalls, ctx.Err()
		case event, ok := <-events:
			if !ok {
				return builder.String(), toolCalls, nil
			}
			if event.Err != nil {
				if h.OnError != nil {
					h.OnError(event.Err)
				}
				return builder.String(), toolCalls, event.Err
			}
			if h.OnActivity != nil {
				h.OnActivity()
			}
			if event.Delta != "" {
				builder.WriteString(event.Delta)
				if h.OnToken != nil {
					h.OnToken(event.Delta)
				}
			}
			if len(event.ToolCalls) > 0 {
				toolCalls = append(toolCalls, event.ToolCalls...)
				if h.OnToolCalls != nil {
					h.OnToolCalls(event.ToolCalls)
				}
			}
		}
	}
}
