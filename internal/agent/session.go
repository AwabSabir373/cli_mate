package agent

import (
	"context"
	"fmt"
	"time"

	"cli_mate/internal/providers"
	"cli_mate/internal/tools"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	ID         string
	Role       Role
	Content    string
	ToolCalls  []providers.ToolCall
	ToolCallID string
	Name       string
	CreatedAt  time.Time
}

type Session struct {
	id       string
	model    providers.Provider
	tools    map[string]tools.Tool
	window   *ContextWindow
	messages []Message
}

func NewSession(id string, model providers.Provider, window *ContextWindow, toolset []tools.Tool) *Session {
	indexedTools := make(map[string]tools.Tool, len(toolset))
	for _, tool := range toolset {
		indexedTools[tool.Name()] = tool
	}

	return &Session{
		id:     id,
		model:  model,
		tools:  indexedTools,
		window: window,
	}
}

func (s *Session) ID() string {
	return s.id
}

func (s *Session) Messages() []Message {
	out := make([]Message, len(s.messages))
	copy(out, s.messages)
	return out
}

func (s *Session) AddMessage(role Role, content string) {
	s.messages = append(s.messages, Message{
		Role:      role,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	})
}

func (s *Session) Complete(ctx context.Context, prompt string) (<-chan providers.StreamEvent, error) {
	if s.model == nil {
		return nil, fmt.Errorf("agent session has no provider")
	}

	s.AddMessage(RoleUser, prompt)
	messages := s.messages
	if s.window != nil {
		messages = s.window.Trim(messages)
	}

	request := providers.ChatRequest{
		Messages: providerMessages(messages),
		Tools:    providerTools(s.toolDefinitions()),
	}
	return s.model.StreamChat(ctx, request)
}

func (s *Session) toolDefinitions() []tools.Definition {
	definitions := make([]tools.Definition, 0, len(s.tools))
	for _, tool := range s.tools {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

func providerMessages(messages []Message) []providers.Message {
	out := make([]providers.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, providers.Message{
			Role:       string(message.Role),
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallID: message.ToolCallID,
			Name:       message.Name,
		})
	}
	return out
}

func providerTools(definitions []tools.Definition) []providers.ToolDefinition {
	out := make([]providers.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, providers.ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			Schema:      definition.Schema,
		})
	}
	return out
}
