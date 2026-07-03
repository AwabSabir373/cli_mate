package contracts

import "context"

type Provider interface {
	Name() string
	StreamChat(context.Context, ChatRequest) (<-chan StreamEvent, error)
}

type ChatRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDefinition
	Temperature float64
	MaxTokens   int
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type StreamEvent struct {
	Delta     string
	ToolCalls []ToolCall
	Err       error
	Done      bool
	Usage     Usage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ToolDefinition struct {
	Name        string
	Description string
	Schema      map[string]any
}
