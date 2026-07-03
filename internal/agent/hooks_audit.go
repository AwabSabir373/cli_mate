package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEntry records a single hook execution.
type AuditEntry struct {
	Sequence  int64     `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
	Event     HookEvent `json:"event"`
	ToolName  string    `json:"toolName,omitempty"`
	Command   string    `json:"command"`
	ExitCode  int       `json:"exitCode"`
	Blocked   bool      `json:"blocked"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// AuditStore maintains an append-only JSONL audit log.
type AuditStore struct {
	logPath string
	seq     int64
	mu      sync.Mutex
}

// NewAuditStore creates an audit store at the given path.
func NewAuditStore(logPath string) (*AuditStore, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return nil, fmt.Errorf("audit store: mkdir: %w", err)
	}
	return &AuditStore{logPath: logPath}, nil
}

// Record appends an audit entry to the log.
func (a *AuditStore) Record(entry AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.seq++
	entry.Sequence = a.seq
	entry.Timestamp = time.Now().UTC()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(a.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// RecordResults records multiple hook results as audit entries.
func (a *AuditStore) RecordResults(event HookEvent, toolName string, results []HookResult) {
	for _, r := range results {
		entry := AuditEntry{
			Event:    event,
			ToolName: toolName,
			Command:  r.Hook.Command,
			ExitCode: r.ExitCode,
			Blocked:  r.Blocked,
			Output:   truncateString(r.Output, 1000),
		}
		if r.Error != nil {
			entry.Error = r.Error.Error()
		}
		a.Record(entry)
	}
}

// Count returns the number of entries in the audit log.
func (a *AuditStore) Count() (int, error) {
	data, err := os.ReadFile(a.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count, nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
