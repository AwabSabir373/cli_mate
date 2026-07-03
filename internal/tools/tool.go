package tools

import "context"

type Definition struct {
	Name        string
	Description string
	Schema      map[string]any
}

type Call struct {
	Name      string
	Argument  map[string]any
	SessionID string
}

type Result struct {
	Content        string
	Error          string
	LoadedTools    []string
	RequestedModel string
}

type Tool interface {
	Name() string
	Definition() Definition
	Execute(context.Context, Call) (Result, error)
}
