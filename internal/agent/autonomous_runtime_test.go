package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cli_mate/internal/agent/agentloop"
	"cli_mate/internal/providers"
	"cli_mate/internal/tools"
	"cli_mate/pkg/tokenizer"
)

type scriptedProvider struct {
	responses []providers.StreamEvent
	requests  []providers.ChatRequest
}

func (p *scriptedProvider) Name() string {
	return "scripted"
}

func (p *scriptedProvider) StreamChat(_ context.Context, req providers.ChatRequest) (<-chan providers.StreamEvent, error) {
	p.requests = append(p.requests, req)
	ch := make(chan providers.StreamEvent, len(p.responses)+1)
	if len(p.responses) == 0 {
		ch <- providers.StreamEvent{Delta: "done"}
		close(ch)
		return ch, nil
	}
	event := p.responses[0]
	p.responses = p.responses[1:]
	ch <- event
	close(ch)
	return ch, nil
}

func TestCodingRunnerInjectsAutonomousLoopState(t *testing.T) {
	provider := &scriptedProvider{responses: []providers.StreamEvent{{Delta: "All set."}}}
	runner := NewCodingRunner(provider, "", nil, t.TempDir())

	result, err := runner.Run(context.Background(), RunOptions{
		Prompt:       "inspect the project",
		MaxTokens:    8000,
		Counter:      tokenizer.NewApproxCounter(),
		DisableTools: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(provider.requests))
	}
	last := provider.requests[0].Messages[len(provider.requests[0].Messages)-1]
	if !strings.Contains(last.Content, "Autonomous loop state for CLI Mate") {
		t.Fatalf("expected autonomous loop state injection, got %q", last.Content)
	}
	if !hasEvent(result.Events, agentloop.EventPlanningCompleted) {
		t.Fatal("expected planning completed event")
	}
	if !hasEvent(result.Events, agentloop.EventCompleted) {
		t.Fatal("expected completed event")
	}
}

func TestCodingRunnerBlocksEditBeforeRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := &scriptedProvider{responses: []providers.StreamEvent{
		{Delta: "```cli_mate-tool\n{\"tool\":\"file_edit\",\"arguments\":{\"path\":\"main.go\",\"old\":\"package main\",\"new\":\"package cli_mate\"}}\n```"},
		{Delta: "Stopped after safety feedback."},
	}}
	runner := NewCodingRunner(provider, "", []tools.Tool{
		tools.NewFileReadTool(root),
		tools.NewFileEditTool(root),
	}, root)
	runner.MaxIterations = 2

	result, err := runner.Run(context.Background(), RunOptions{
		Prompt:      "edit main.go",
		MaxTokens:   8000,
		Counter:     tokenizer.NewApproxCounter(),
		ApproveTool: func(tools.Call) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package main\n" {
		t.Fatalf("file changed despite missing read: %q", string(data))
	}
	if !hasEvent(result.Events, agentloop.EventWaitingForUser) {
		t.Fatal("expected waiting/safety event")
	}
	if !hasEvent(result.Events, agentloop.EventToolCompleted) {
		t.Fatal("expected tool completed event")
	}
}

func TestCodingRunnerRunsSafeNativeToolBatchInParallel(t *testing.T) {
	first := &sleepTool{name: "first_read", delay: 150 * time.Millisecond}
	second := &sleepTool{name: "second_read", delay: 150 * time.Millisecond}
	provider := &scriptedProvider{responses: []providers.StreamEvent{
		{ToolCalls: []providers.ToolCall{
			{ID: "call-1", Name: "first_read", Arguments: "{}"},
			{ID: "call-2", Name: "second_read", Arguments: "{}"},
		}},
		{Delta: "Done."},
	}}
	runner := NewCodingRunner(provider, "", []tools.Tool{first, second}, t.TempDir())
	runner.MaxIterations = 3

	start := time.Now()
	result, err := runner.Run(context.Background(), RunOptions{
		Prompt:    "inspect two things",
		MaxTokens: 8000,
		Counter:   tokenizer.NewApproxCounter(),
	})
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed >= 280*time.Millisecond {
		t.Fatalf("expected parallel execution under 280ms, took %s", elapsed)
	}
	if first.calls() != 1 || second.calls() != 1 {
		t.Fatalf("expected both tools once, got first=%d second=%d", first.calls(), second.calls())
	}
	completed := countEvents(result.Events, agentloop.EventToolCompleted)
	if completed != 2 {
		t.Fatalf("expected two completed tool events, got %d", completed)
	}
}

func TestParseToolCallAcceptsUnclosedToolFenceWithCompleteJSON(t *testing.T) {
	call, ok, err := parseToolCall("thinking\n```cli_mate-tool\n{\"tool\":\"file_read\",\"arguments\":{\"path\":\"src/components/card.tsx\"}}")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected tool call")
	}
	if call.Name != "file_read" {
		t.Fatalf("expected file_read, got %q", call.Name)
	}
	if call.Argument["path"] != "src/components/card.tsx" {
		t.Fatalf("expected path argument, got %#v", call.Argument)
	}
}

func TestParseToolCallRejectsIncompleteUnclosedToolFence(t *testing.T) {
	_, ok, err := parseToolCall("thinking\n```cli_mate-tool\n{\"tool\":\"file_read\",\"arguments\":{\"path\":\"src/components")
	if !ok {
		t.Fatal("expected malformed tool call to be recognized")
	}
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "incomplete JSON") {
		t.Fatalf("expected incomplete JSON error, got %v", err)
	}
}

func TestAcceptFinalAnswerRejectsMutationClaimWithoutEdit(t *testing.T) {
	root := t.TempDir()
	runner := NewCodingRunner(&scriptedProvider{}, "", []tools.Tool{
		tools.NewGlobTool(root),
		tools.NewFileEditTool(root),
	}, root)
	rt := runner.newAutonomousRuntime(RunOptions{
		Prompt:    "fix the product card slug overflow",
		MaxTokens: 8000,
		Counter:   tokenizer.NewApproxCounter(),
	}, 3)
	rt.emit(agentloop.Event{
		Type:    agentloop.EventToolCompleted,
		Summary: "glob completed",
		Data: map[string]any{
			"tool":             "glob",
			"mutates":          false,
			"mutation_applied": false,
			"success":          true,
		},
	})

	accepted, feedback := rt.acceptFinalAnswer(context.Background(), "Task complete. File modified: app/dashboard/products/components/product-card.tsx")
	if accepted {
		t.Fatal("expected false completion claim to be rejected")
	}
	if !strings.Contains(feedback, "no successful file edit/write/patch tool has run") {
		t.Fatalf("expected mutation evidence feedback, got %q", feedback)
	}
}

func TestAcceptFinalAnswerAllowsCompletionAfterSuccessfulEdit(t *testing.T) {
	root := t.TempDir()
	runner := NewCodingRunner(&scriptedProvider{}, "", []tools.Tool{
		tools.NewFileEditTool(root),
	}, root)
	rt := runner.newAutonomousRuntime(RunOptions{
		Prompt:    "fix the product card slug overflow",
		MaxTokens: 8000,
		Counter:   tokenizer.NewApproxCounter(),
	}, 3)
	rt.emit(agentloop.Event{
		Type:    agentloop.EventToolCompleted,
		Summary: "file_edit completed",
		Data: map[string]any{
			"tool":             "file_edit",
			"mutates":          true,
			"mutation_applied": true,
			"success":          true,
		},
	})
	rt.emit(agentloop.Event{
		Type:    agentloop.EventVerificationPassed,
		Summary: "verification passed",
	})

	accepted, feedback := rt.acceptFinalAnswer(context.Background(), "Updated product-card.tsx and verified the change.")
	if !accepted {
		t.Fatalf("expected successful edit evidence to allow final answer, got %q", feedback)
	}
}

func hasEvent(events []agentloop.Event, kind agentloop.EventType) bool {
	for _, event := range events {
		if event.Type == kind {
			return true
		}
	}
	return false
}

func countEvents(events []agentloop.Event, kind agentloop.EventType) int {
	count := 0
	for _, event := range events {
		if event.Type == kind {
			count++
		}
	}
	return count
}

type sleepTool struct {
	name  string
	delay time.Duration
	mu    sync.Mutex
	count int
}

func (t *sleepTool) Name() string {
	return t.name
}

func (t *sleepTool) Definition() tools.Definition {
	return tools.Definition{
		Name:        t.name,
		Description: "read test data",
	}
}

func (t *sleepTool) Execute(ctx context.Context, _ tools.Call) (tools.Result, error) {
	select {
	case <-ctx.Done():
		return tools.Result{Error: ctx.Err().Error()}, ctx.Err()
	case <-time.After(t.delay):
	}
	t.mu.Lock()
	t.count++
	t.mu.Unlock()
	return tools.Result{Content: t.name + " ok"}, nil
}

func (t *sleepTool) calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}
