package agent

import (
	"fmt"
	"time"

	"cli_mate/internal/providers"
)

// Checkpoint represents a snapshot of conversation state.
type Checkpoint struct {
	ID        string
	Messages  []providers.Message
	Timestamp time.Time
	Label     string
}

// RewindManager manages conversation checkpoints for rewind functionality.
type RewindManager struct {
	checkpoints []Checkpoint
	maxStored   int
}

// NewRewindManager creates a new rewind manager.
func NewRewindManager(maxStored int) *RewindManager {
	if maxStored <= 0 {
		maxStored = 10
	}
	return &RewindManager{
		maxStored: maxStored,
	}
}

// SaveCheckpoint saves the current conversation state.
func (rm *RewindManager) SaveCheckpoint(messages []providers.Message, label string) string {
	id := fmt.Sprintf("cp_%d", len(rm.checkpoints))
	cp := Checkpoint{
		ID:        id,
		Messages:  copyMessages(messages),
		Timestamp: time.Now(),
		Label:     label,
	}
	rm.checkpoints = append(rm.checkpoints, cp)

	// Trim old checkpoints if we exceed max
	if len(rm.checkpoints) > rm.maxStored {
		rm.checkpoints = rm.checkpoints[len(rm.checkpoints)-rm.maxStored:]
	}

	return id
}

// RewindTo rewinds to a specific checkpoint and returns the messages.
func (rm *RewindManager) RewindTo(checkpointID string) ([]providers.Message, error) {
	for _, cp := range rm.checkpoints {
		if cp.ID == checkpointID {
			return copyMessages(cp.Messages), nil
		}
	}
	return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
}

// RewindToLatest rewinds to the most recent checkpoint.
func (rm *RewindManager) RewindToLatest() ([]providers.Message, error) {
	if len(rm.checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints available")
	}
	latest := rm.checkpoints[len(rm.checkpoints)-1]
	return copyMessages(latest.Messages), nil
}

// ListCheckpoints returns all available checkpoints.
func (rm *RewindManager) ListCheckpoints() []Checkpoint {
	return rm.checkpoints
}

// Clear removes all checkpoints.
func (rm *RewindManager) Clear() {
	rm.checkpoints = nil
}

func copyMessages(messages []providers.Message) []providers.Message {
	result := make([]providers.Message, len(messages))
	copy(result, messages)
	return result
}
