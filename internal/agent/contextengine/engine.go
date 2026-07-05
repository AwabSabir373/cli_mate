package contextengine

import (
	"context"
	"sort"
	"strings"
	"time"

	"cli_mate/internal/agent/agentloop"
)

type Collector interface {
	Collect(context.Context, *agentloop.RunState) ([]agentloop.ContextItem, error)
}

type MemoryReader interface {
	Query(context.Context, agentloop.MemoryQuery) ([]agentloop.MemoryItem, error)
}

type Engine struct {
	Collectors []Collector
	Memory     MemoryReader
	Budget     agentloop.TokenBudget
	MaxItems   int
}

func New(budget agentloop.TokenBudget, collectors ...Collector) *Engine {
	if budget.Total <= 0 {
		budget.Total = 16000
	}
	if budget.ReservedOutput <= 0 {
		budget.ReservedOutput = 2000
	}
	if budget.ReservedTools <= 0 {
		budget.ReservedTools = 1500
	}
	return &Engine{Budget: budget, Collectors: collectors, MaxItems: 32}
}

func (e *Engine) Build(ctx context.Context, state *agentloop.RunState) (agentloop.ContextBundle, error) {
	var items []agentloop.ContextItem
	for _, collector := range e.Collectors {
		collected, err := collector.Collect(ctx, state)
		if err != nil {
			items = append(items, agentloop.ContextItem{
				ID:        agentloop.NewID("ctx"),
				Kind:      "collector_error",
				Source:    "context_engine",
				Content:   err.Error(),
				Score:     0.1,
				CreatedAt: time.Now().UTC(),
			})
			continue
		}
		items = append(items, collected...)
	}

	rank(state, items)
	items = compressAndEvict(items, e.Budget, e.MaxItems)

	var memories []agentloop.MemoryItem
	if e.Memory != nil {
		found, err := e.Memory.Query(ctx, agentloop.MemoryQuery{
			Text:  memoryQueryText(state),
			Limit: 12,
		})
		if err == nil {
			memories = found
		}
	}

	return agentloop.ContextBundle{
		UserRequest:  state.UserRequest,
		ActiveTask:   state.ActiveTask(),
		Items:        items,
		Memories:     memories,
		RecentEvents: recentEvents(state.Events, 20),
		TokenBudget:  usedBudget(e.Budget, items),
	}, nil
}

func memoryQueryText(state *agentloop.RunState) string {
	if state == nil {
		return ""
	}
	parts := []string{state.UserRequest}
	if active := state.ActiveTask(); active != nil {
		parts = append(parts, active.Goal)
	}
	return strings.Join(parts, " ")
}

func rank(state *agentloop.RunState, items []agentloop.ContextItem) {
	query := strings.ToLower(state.UserRequest)
	active := state.ActiveTask()
	for i := range items {
		item := &items[i]
		if item.ID == "" {
			item.ID = agentloop.NewID("ctx")
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now().UTC()
		}
		if item.Tokens <= 0 {
			item.Tokens = estimateTokens(item.Content)
		}
		score := item.Score
		lowered := strings.ToLower(item.Content + " " + item.Source + " " + item.Kind)
		for _, word := range strings.Fields(query) {
			if len(word) > 2 && strings.Contains(lowered, word) {
				score += 0.25
			}
		}
		if active != nil && strings.Contains(lowered, strings.ToLower(active.Goal)) {
			score += 1
		}
		if item.Exact {
			score += 0.5
		}
		item.Score = score
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].Tokens < items[j].Tokens
		}
		return items[i].Score > items[j].Score
	})
}

func compressAndEvict(items []agentloop.ContextItem, budget agentloop.TokenBudget, maxItems int) []agentloop.ContextItem {
	if maxItems <= 0 {
		maxItems = 32
	}
	remaining := budget.Total - budget.ReservedOutput - budget.ReservedTools
	if remaining <= 0 {
		remaining = 4000
	}
	out := make([]agentloop.ContextItem, 0, len(items))
	used := 0
	seen := map[string]bool{}
	for _, item := range items {
		key := item.Kind + "\x00" + item.Source + "\x00" + item.Content
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(out) >= maxItems {
			break
		}
		if used+item.Tokens > remaining && !item.Exact {
			item.Content = summarize(item.Content, 1200)
			item.Tokens = estimateTokens(item.Content)
		}
		if used+item.Tokens > remaining && !item.Exact {
			continue
		}
		used += item.Tokens
		out = append(out, item)
	}
	return out
}

func usedBudget(budget agentloop.TokenBudget, items []agentloop.ContextItem) agentloop.TokenBudget {
	for _, item := range items {
		budget.Used += item.Tokens
	}
	return budget
}

func recentEvents(events []agentloop.Event, limit int) []agentloop.Event {
	if limit <= 0 || len(events) <= limit {
		out := make([]agentloop.Event, len(events))
		copy(out, events)
		return out
	}
	out := make([]agentloop.Event, limit)
	copy(out, events[len(events)-limit:])
	return out
}

func estimateTokens(text string) int {
	count := 0
	for _, r := range text {
		if r > ' ' {
			count++
		}
	}
	return count/4 + 1
}

func summarize(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	head := maxBytes * 2 / 3
	tail := maxBytes - head
	return strings.TrimSpace(text[:head]) + "\n... [compressed] ...\n" + strings.TrimSpace(text[len(text)-tail:])
}

type StaticCollector struct {
	Items []agentloop.ContextItem
}

func (c StaticCollector) Collect(_ context.Context, _ *agentloop.RunState) ([]agentloop.ContextItem, error) {
	out := make([]agentloop.ContextItem, len(c.Items))
	copy(out, c.Items)
	return out, nil
}
