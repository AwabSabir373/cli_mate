package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"cli_mate/internal/agent/agentloop"
)

type Store interface {
	Put(context.Context, agentloop.MemoryItem) error
	Query(context.Context, agentloop.MemoryQuery) ([]agentloop.MemoryItem, error)
	UpdateFromEvents(context.Context, string, []agentloop.Event) error
}

type InMemoryStore struct {
	mu    sync.RWMutex
	items map[string]agentloop.MemoryItem
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{items: map[string]agentloop.MemoryItem{}}
}

func (s *InMemoryStore) Put(_ context.Context, item agentloop.MemoryItem) error {
	now := time.Now().UTC()
	if item.ID == "" {
		item.ID = agentloop.NewID("mem")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *InMemoryStore) Query(_ context.Context, q agentloop.MemoryQuery) ([]agentloop.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []agentloop.MemoryItem
	for _, item := range s.items {
		if q.Scope != "" && item.Scope != "" && item.Scope != q.Scope {
			continue
		}
		if q.Type != "" && item.Type != q.Type {
			continue
		}
		if q.Text != "" && !matches(item, q.Text) {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].Confidence > out[j].Confidence
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (s *InMemoryStore) UpdateFromEvents(ctx context.Context, scope string, events []agentloop.Event) error {
	for _, event := range events {
		switch event.Type {
		case agentloop.EventToolFinished, agentloop.EventToolCompleted:
			item := agentloop.MemoryItem{
				Type:       agentloop.ToolHistory,
				Scope:      scope,
				Key:        event.TaskID + ":" + event.Summary,
				Value:      event.Summary,
				Confidence: 0.8,
				Evidence:   event.Evidence,
			}
			if err := s.Put(ctx, item); err != nil {
				return err
			}
		case agentloop.EventVerificationFailed:
			item := agentloop.MemoryItem{
				Type:       agentloop.FailedAttempt,
				Scope:      scope,
				Key:        event.TaskID + ":verification_failed",
				Value:      event.Summary,
				Confidence: 0.9,
				Evidence:   event.Evidence,
			}
			if err := s.Put(ctx, item); err != nil {
				return err
			}
		case agentloop.EventVerificationPassed, agentloop.EventCompleted:
			item := agentloop.MemoryItem{
				Type:       agentloop.StrategyMemory,
				Scope:      scope,
				Key:        event.TaskID + ":success",
				Value:      event.Summary,
				Confidence: 0.85,
				Evidence:   event.Evidence,
			}
			if err := s.Put(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func matches(item agentloop.MemoryItem, query string) bool {
	haystack := strings.ToLower(item.Key + " " + item.Value)
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if len(term) > 2 && strings.Contains(haystack, term) {
			return true
		}
	}
	return query == ""
}
