package agent

import "cli_mate/pkg/tokenizer"

type ContextWindow struct {
	MaxTokens     int
	ReserveTokens int
	counter       tokenizer.Counter
}

func NewContextWindow(maxTokens, reserveTokens int, counter tokenizer.Counter) *ContextWindow {
	return &ContextWindow{
		MaxTokens:     maxTokens,
		ReserveTokens: reserveTokens,
		counter:       counter,
	}
}

func (w *ContextWindow) Trim(messages []Message) []Message {
	if len(messages) == 0 || w == nil || w.MaxTokens <= 0 {
		return messages
	}

	counter := w.counter
	if counter == nil {
		counter = tokenizer.NewApproxCounter()
	}

	budget := w.MaxTokens - w.ReserveTokens
	if budget <= 0 {
		budget = w.MaxTokens
	}

	selected := make([]Message, 0, len(messages))
	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		cost := counter.Count(messages[i].Content)
		if used+cost > budget && len(selected) > 0 {
			break
		}
		used += cost
		selected = append(selected, messages[i])
	}

	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}
	return selected
}
