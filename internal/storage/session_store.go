package storage

import (
	"context"
	"time"

	"cli_mate/internal/agent"
)

type SessionRecord struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SessionStore interface {
	CreateSession(context.Context, SessionRecord) error
	AppendMessage(context.Context, string, agent.Message) error
	Messages(context.Context, string) ([]agent.Message, error)
	ListSessions(context.Context) ([]SessionRecord, error)
	DeleteSession(context.Context, string) error
}
