package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"cli_mate/internal/providers"
)

// defaultCompactionPreserveLast is how many trailing messages are kept verbatim
// when the caller does not specify a preserve count.
const defaultCompactionPreserveLast = 8

// compactionTriggerRatio is the fraction of the context window at which
// proactive compaction kicks in (top of each turn).
const compactionTriggerRatio = 0.8

// summaryLabel prefixes the injected summary so it is unmistakable in the
// transcript (and so tests can assert on it).
const summaryLabel = "[Summary of earlier conversation]"

// summaryInstructions is the system prompt handed to the summarizer model.
const summaryInstructions = "You are compacting a coding-assistant conversation to save context. " +
	"Write a dense, factual summary of the conversation so far. Preserve: the user's goals and explicit constraints; " +
	"decisions made and why; files created or modified (with paths) and key code changes; commands run and their important " +
	"results; and anything still in progress or unresolved. Omit pleasantries. Use terse bullet points. Do not invent details. " +
	"If the conversation already begins with an earlier summary block, treat its facts as established context and carry them " +
	"forward into the new summary — never drop earlier information."

// CompactionOptions configure a single Compact call.
type CompactionOptions struct {
	// PreserveLast is the number of trailing messages to keep verbatim. The
	// preserved suffix is widened (never shrunk) so it begins at a safe
	// user/assistant boundary. <= 0 falls back to defaultCompactionPreserveLast.
	PreserveLast int
	// Summarize turns the to-be-elided middle into a single dense summary. It is
	// injected so Compact stays pure and testable; the agent loop wires it to a
	// real provider call.
	Summarize func(toSummarize []providers.Message) (string, error)
}

// CompactionResult is the metadata-bearing result returned by CompactMessages.
type CompactionResult struct {
	Messages       []providers.Message
	RemovedCount   int
	PreservedCount int
	SummaryText    string
	Compacted      bool
}

// imageTokenEstimate is a flat per-image token cost.
const imageTokenEstimate = 1000

// ApproxTextTokens estimates the token count of text without a real tokenizer.
// Counting non-whitespace bytes / 4 tracks the provider's actual count closely.
func ApproxTextTokens(value string) int {
	nonSpace := 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
		default:
			nonSpace++
		}
	}
	return nonSpace / 4
}

// estimateTokens is a cheap, dependency-free token estimate across message
// content plus tool call names/arguments and a flat per-image cost.
func estimateTokens(messages []providers.Message) int {
	total := 0
	for _, message := range messages {
		total += ApproxTextTokens(message.Content)
		for _, call := range message.ToolCalls {
			total += ApproxTextTokens(call.Name)
			total += ApproxTextTokens(call.Arguments)
			total += 4 // small per-call overhead
		}
		total += 4 // per-message overhead
	}
	return total
}

// estimateToolDefTokens approximates the input-token cost of the tool
// definitions sent with every request.
func estimateToolDefTokens(tools []providers.ToolDefinition) int {
	total := 0
	for _, tool := range tools {
		total += ApproxTextTokens(tool.Name)
		total += ApproxTextTokens(tool.Description)
		if len(tool.Schema) > 0 {
			if encoded, err := json.Marshal(tool.Schema); err == nil {
				total += ApproxTextTokens(string(encoded))
			}
		}
		total += 4 // per-tool overhead
	}
	return total
}

// compactionThreshold is the estimated-token level at which proactive
// compaction triggers for a given context window.
func compactionThreshold(contextWindow int) int {
	if contextWindow <= 0 {
		return 0
	}
	return int(float64(contextWindow) * compactionTriggerRatio)
}

// Compact summarizes the oldest middle of a conversation, keeping the leading
// system message(s) and the most recent turns verbatim.
func Compact(messages []providers.Message, opts CompactionOptions) ([]providers.Message, error) {
	result, err := CompactMessages(messages, opts)
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// CompactMessages summarizes the oldest middle of a conversation and returns
// both the replacement messages and metadata about what changed.
func CompactMessages(messages []providers.Message, opts CompactionOptions) (CompactionResult, error) {
	preserveLast := opts.PreserveLast
	if preserveLast <= 0 {
		preserveLast = defaultCompactionPreserveLast
	}
	if opts.Summarize == nil {
		return CompactionResult{}, errors.New("compaction requires a Summarize function")
	}

	// Leading system messages are kept verbatim at the head.
	systemEnd := 0
	for systemEnd < len(messages) && messages[systemEnd].Role == "system" {
		systemEnd++
	}

	// Naive boundary: keep the last preserveLast messages. Then widen the suffix
	// backward to a safe boundary so it never starts on a tool result.
	boundary := len(messages) - preserveLast
	if boundary < systemEnd {
		boundary = systemEnd
	}
	boundary = safeSuffixBoundary(messages, systemEnd, boundary)

	middle := messages[systemEnd:boundary]
	if len(middle) == 0 {
		return CompactionResult{
			Messages:       messages,
			PreservedCount: len(messages),
		}, nil
	}

	summary, err := opts.Summarize(middle)
	if err != nil {
		return CompactionResult{}, err
	}
	summary = strings.TrimSpace(summary)

	content := summaryLabel + "\n" + summary

	compacted := make([]providers.Message, 0, systemEnd+1+(len(messages)-boundary))
	compacted = append(compacted, messages[:systemEnd]...)
	compacted = append(compacted, providers.Message{
		Role:    "user",
		Content: content,
	})
	compacted = append(compacted, messages[boundary:]...)
	return CompactionResult{
		Messages:       compacted,
		RemovedCount:   len(middle),
		PreservedCount: len(messages) - len(middle),
		SummaryText:    summary,
		Compacted:      true,
	}, nil
}

// safeSuffixBoundary walks the preserve boundary backward so the preserved
// suffix begins on an assistant message rather than a tool result.
func safeSuffixBoundary(messages []providers.Message, systemEnd int, boundary int) int {
	for boundary > systemEnd && messages[boundary].Role != "assistant" {
		boundary--
	}
	return boundary
}

// isContextLimitError reports whether a provider error string looks like a
// context-window / prompt-too-long error.
func isContextLimitError(message string) bool {
	lowered := strings.ToLower(strings.TrimSpace(message))
	if lowered == "" {
		return false
	}
	needles := []string{
		"context length",
		"context window",
		"context_length_exceeded",
		"maximum context",
		"context limit",
		"prompt is too long",
		"too many tokens",
		"reduce the length of the messages",
		"exceeds the maximum",
		"input is too long",
	}
	for _, needle := range needles {
		if strings.Contains(lowered, needle) {
			return true
		}
	}
	return false
}

// compactionState carries the per-run state the agent loop needs to compact a
// conversation safely.
type compactionState struct {
	enabled      bool
	threshold    int
	preserveLast int
	lowWaterMark int
	// reactiveAttempted guards the reactive path so it fires at most once per
	// run.
	reactiveAttempted bool
	onUsage           func(providers.Usage)
}

func newCompactionState(contextWindow, preserveLast int, onUsage func(providers.Usage)) *compactionState {
	return &compactionState{
		enabled:      contextWindow > 0,
		threshold:    compactionThreshold(contextWindow),
		preserveLast: preserveLast,
		onUsage:      onUsage,
	}
}

// maybeCompact runs proactive compaction at the top of a turn. It returns the
// (possibly compacted) message slice.
func (state *compactionState) maybeCompact(
	ctx context.Context,
	provider providers.Provider,
	messages []providers.Message,
	tools []providers.ToolDefinition,
) []providers.Message {
	if !state.enabled {
		return messages
	}
	toolTokens := estimateToolDefTokens(tools)
	size := estimateTokens(messages) + toolTokens
	if size <= state.threshold {
		return messages
	}
	if state.lowWaterMark > 0 && size <= state.lowWaterMark {
		return messages
	}

	// Prune stale tool output first (free reclaim).
	if pruned, reclaimed := pruneStaleToolOutput(messages, state.preserveLast); reclaimed > 0 {
		messages = pruned
		size = estimateTokens(messages) + toolTokens
		if size <= state.threshold {
			state.lowWaterMark = size
			return messages
		}
	}

	compacted, err := Compact(messages, CompactionOptions{
		PreserveLast: state.preserveLast,
		Summarize:    summarizeClosure(ctx, provider, state.onUsage),
	})
	if err != nil {
		return messages
	}
	newSize := estimateTokens(compacted) + toolTokens
	if newSize >= size {
		state.lowWaterMark = size
		return messages
	}
	state.lowWaterMark = newSize
	return compacted
}

// recover runs reactive compaction after a provider/stream error. It compacts
// at most once per run when the error looks like a context-limit error.
func (state *compactionState) recover(
	ctx context.Context,
	provider providers.Provider,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	errorMessage string,
) (compacted []providers.Message, retried bool, err error) {
	if !state.enabled {
		return messages, false, nil
	}
	if state.reactiveAttempted {
		return messages, false, nil
	}
	if !isContextLimitError(errorMessage) {
		return messages, false, nil
	}

	result, compactErr := Compact(messages, CompactionOptions{
		PreserveLast: state.preserveLast,
		Summarize:    summarizeClosure(ctx, provider, state.onUsage),
	})
	if compactErr != nil {
		state.reactiveAttempted = true
		return messages, true, compactErr
	}
	if estimateTokens(result) >= estimateTokens(messages) {
		return messages, false, nil
	}
	state.reactiveAttempted = true
	state.lowWaterMark = estimateTokens(result) + estimateToolDefTokens(tools)
	return result, true, nil
}

// summarizeClosure builds a Summarize function backed by a focused, tool-less
// provider call.
func summarizeClosure(ctx context.Context, provider providers.Provider, onUsage func(providers.Usage)) func([]providers.Message) (string, error) {
	return func(toSummarize []providers.Message) (string, error) {
		return summarizeWithFallback(ctx, provider, toSummarize, onUsage)
	}
}

// summarizeWithFallback summarizes messages in a single provider call. If that
// call fails with a context-limit error, it splits the slice in half,
// summarizes each half recursively, and joins the partial summaries.
func summarizeWithFallback(ctx context.Context, provider providers.Provider, messages []providers.Message, onUsage func(providers.Usage)) (string, error) {
	summary, err := summarizeMessagesOnce(ctx, provider, messages, onUsage)
	if err == nil {
		return summary, nil
	}
	if len(messages) < 2 || !isContextLimitError(err.Error()) {
		return "", err
	}

	mid := len(messages) / 2
	left, leftErr := summarizeWithFallback(ctx, provider, messages[:mid], onUsage)
	if leftErr != nil {
		return "", leftErr
	}
	right, rightErr := summarizeWithFallback(ctx, provider, messages[mid:], onUsage)
	if rightErr != nil {
		return "", rightErr
	}

	combined := strings.TrimSpace(left + "\n\n" + right)
	reduced, reduceErr := summarizeMessagesOnce(ctx, provider, []providers.Message{
		{Role: "user", Content: combined},
	}, onUsage)
	if reduceErr != nil {
		if isContextLimitError(reduceErr.Error()) {
			return combined, nil
		}
		return "", reduceErr
	}
	return reduced, nil
}

// summarizeMessagesOnce performs a single tool-less summarization call.
func summarizeMessagesOnce(ctx context.Context, provider providers.Provider, messages []providers.Message, onUsage func(providers.Usage)) (string, error) {
	request := providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: summaryInstructions},
			{Role: "user", Content: "Summarize this conversation:\n\n" + renderTranscript(messages)},
		},
	}
	stream, err := provider.StreamChat(ctx, request)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for event := range stream {
		if event.Err != nil {
			return "", event.Err
		}
		if event.Usage.TotalTokens > 0 && onUsage != nil {
			onUsage(event.Usage)
		}
		b.WriteString(event.Delta)
	}
	summary := strings.TrimSpace(b.String())
	if summary == "" {
		return "", errors.New("summarizer returned no text")
	}
	return summary, nil
}

// renderTranscript flattens messages into a plain-text transcript for the
// summarizer.
func renderTranscript(messages []providers.Message) string {
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case "assistant":
			line := "assistant: " + message.Content
			if len(message.ToolCalls) > 0 {
				calls := make([]string, 0, len(message.ToolCalls))
				for _, call := range message.ToolCalls {
					calls = append(calls, call.Name+"("+call.Arguments+")")
				}
				line += "\n[tool calls: " + strings.Join(calls, "; ") + "]"
			}
			lines = append(lines, line)
		case "tool":
			lines = append(lines, "tool result: "+message.Content)
		default:
			lines = append(lines, message.Role+": "+message.Content)
		}
	}
	return strings.Join(lines, "\n\n")
}

// pruneStaleToolOutput reduces the size of old tool results to reclaim context
// without calling the summarizer. Returns the pruned messages and how many
// tokens were reclaimed.
func pruneStaleToolOutput(messages []providers.Message, preserveLast int) ([]providers.Message, int) {
	if preserveLast <= 0 {
		preserveLast = defaultCompactionPreserveLast
	}
	cutoff := len(messages) - preserveLast
	if cutoff <= 0 {
		return messages, 0
	}

	pruned := make([]providers.Message, len(messages))
	tokensReclaimed := 0
	for i, msg := range messages {
		if i < cutoff && msg.Role == "tool" && len(msg.Content) > 500 {
			trimmed := msg.Content[:200] + "\n... [pruned to save context] ...\n" + msg.Content[len(msg.Content)-100:]
			tokensReclaimed += ApproxTextTokens(msg.Content) - ApproxTextTokens(trimmed)
			pruned[i] = providers.Message{
				Role:       msg.Role,
				Content:    trimmed,
				ToolCalls:  msg.ToolCalls,
				ToolCallID: msg.ToolCallID,
				Name:       msg.Name,
			}
		} else {
			pruned[i] = msg
		}
	}
	return pruned, tokensReclaimed
}
