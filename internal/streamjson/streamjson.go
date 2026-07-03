// Package streamjson implements a JSONL protocol for programmatic I/O.
// It provides structured event streaming for headless mode, IDE integration,
// and piping agent output to other tools.
package streamjson

import (
	"encoding/json"
	"io"
	"sync"
)

const SchemaVersion = 1

type EventType string

const (
	EventRunStart    EventType = "run_start"
	EventText        EventType = "text"
	EventToolCall    EventType = "tool_call"
	EventToolResult  EventType = "tool_result"
	EventUsage       EventType = "usage"
	EventFinal       EventType = "final"
	EventError       EventType = "error"
	EventRunEnd      EventType = "run_end"
	EventPermission  EventType = "permission"
	EventCheckpoint  EventType = "checkpoint"
	EventWarning     EventType = "warning"
)

// Event is a single JSONL event in the stream.
type Event struct {
	SchemaVersion    int              `json:"schemaVersion"`
	Type             EventType        `json:"type"`
	RunID            string           `json:"runId"`
	SessionID        string           `json:"sessionId,omitempty"`
	Cwd              string           `json:"cwd,omitempty"`
	Provider         string           `json:"provider,omitempty"`
	Model            string           `json:"model,omitempty"`
	Delta            string           `json:"delta,omitempty"`
	Text             string           `json:"text,omitempty"`
	ID               string           `json:"id,omitempty"`
	Name             string           `json:"name,omitempty"`
	Args             any              `json:"args,omitempty"`
	Status           string           `json:"status,omitempty"`
	Output           string           `json:"output,omitempty"`
	Error            string           `json:"error,omitempty"`
	Message          string           `json:"message,omitempty"`
	PromptTokens     *int             `json:"promptTokens,omitempty"`
	CompletionTokens *int             `json:"completionTokens,omitempty"`
	TotalTokens      *int             `json:"totalTokens,omitempty"`
	Meta             map[string]string `json:"meta,omitempty"`
}

// Encoder writes events as JSONL to an underlying writer.
type Encoder struct {
	w   io.Writer
	mu  sync.Mutex
	enc *json.Encoder
}

// NewEncoder creates an encoder that writes JSONL to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:   w,
		enc: json.NewEncoder(w),
	}
}

// Encode writes a single event as a JSON line.
func (e *Encoder) Encode(event Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if event.SchemaVersion == 0 {
		event.SchemaVersion = SchemaVersion
	}
	return e.enc.Encode(event)
}

// Decoder reads events from a JSONL stream.
type Decoder struct {
	r   io.Reader
	dec *json.Decoder
}

// NewDecoder creates a decoder that reads JSONL from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:   r,
		dec: json.NewDecoder(r),
	}
}

// Decode reads the next event from the stream.
func (d *Decoder) Decode() (Event, error) {
	var event Event
	err := d.dec.Decode(&event)
	return event, err
}

// RunStart emits a run_start event.
func (e *Encoder) RunStart(runID, sessionID, cwd, provider, model string) error {
	return e.Encode(Event{
		Type:      EventRunStart,
		RunID:     runID,
		SessionID: sessionID,
		Cwd:       cwd,
		Provider:  provider,
		Model:     model,
	})
}

// Text emits a text delta event.
func (e *Encoder) Text(runID, delta string) error {
	return e.Encode(Event{
		Type:  EventText,
		RunID: runID,
		Delta: delta,
	})
}

// ToolCall emits a tool_call event.
func (e *Encoder) ToolCall(runID, id, name string, args any) error {
	return e.Encode(Event{
		Type:  EventToolCall,
		RunID: runID,
		ID:    id,
		Name:  name,
		Args:  args,
	})
}

// ToolResult emits a tool_result event.
func (e *Encoder) ToolResult(runID, id, name, output string, truncated bool) error {
	return e.Encode(Event{
		Type:  EventToolResult,
		RunID: runID,
		ID:    id,
		Name:  name,
		Output: output,
	})
}

// Usage emits a usage event with token counts.
func (e *Encoder) Usage(runID string, prompt, completion, total int) error {
	return e.Encode(Event{
		Type:             EventUsage,
		RunID:            runID,
		PromptTokens:     &prompt,
		CompletionTokens: &completion,
		TotalTokens:      &total,
	})
}

// Final emits a final event with the complete answer.
func (e *Encoder) Final(runID, text string) error {
	return e.Encode(Event{
		Type:  EventFinal,
		RunID: runID,
		Text:  text,
	})
}

// Error emits an error event.
func (e *Encoder) Error(runID, message, code string) error {
	return e.Encode(Event{
		Type:    EventError,
		RunID:   runID,
		Message: message,
		Error:   code,
	})
}

// RunEnd emits a run_end event.
func (e *Encoder) RunEnd(runID string) error {
	return e.Encode(Event{
		Type:  EventRunEnd,
		RunID: runID,
	})
}
