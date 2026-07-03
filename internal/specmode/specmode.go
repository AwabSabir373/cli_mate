// Package specmode implements a draft-first workflow where the agent creates
// a spec before implementation, the user reviews and approves, then the
// approved spec guides the implementation.
package specmode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status represents the status of a spec or step.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusRejected   Status = "rejected"
)

// Step is a single step in a spec.
type Step struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      Status `json:"status"`
}

// Spec is a draft specification for a task.
type Spec struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Steps       []Step    `json:"steps"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Store manages specs persisted as JSON files.
type Store struct {
	root string
}

// NewStore creates a spec store at the given directory.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// Save persists a spec to disk.
func (s *Store) Save(spec Spec) error {
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}

	spec.UpdatedAt = time.Now()
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = spec.UpdatedAt
	}

	path := filepath.Join(s.root, spec.ID+".json")
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Get loads a spec by ID.
func (s *Store) Get(id string) (*Spec, error) {
	path := filepath.Join(s.root, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// List returns all specs.
func (s *Store) List() ([]Spec, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var specs []Spec
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, entry.Name()))
		if err != nil {
			continue
		}
		var spec Spec
		if err := json.Unmarshal(data, &spec); err != nil {
			continue
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// Delete removes a spec by ID.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.root, id+".json")
	return os.Remove(path)
}

// FormatSpec returns a human-readable representation of a spec.
func FormatSpec(spec Spec) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", spec.Title))
	if spec.Description != "" {
		b.WriteString(spec.Description + "\n\n")
	}
	b.WriteString("## Steps\n\n")
	for i, step := range spec.Steps {
		icon := stepIcon(step.Status)
		b.WriteString(fmt.Sprintf("%s %d. %s\n", icon, i+1, step.Title))
		if step.Description != "" {
			b.WriteString(fmt.Sprintf("   %s\n", step.Description))
		}
	}
	b.WriteString(fmt.Sprintf("\nStatus: %s\n", spec.Status))
	return b.String()
}

func stepIcon(status Status) string {
	switch status {
	case StatusCompleted:
		return "[x]"
	case StatusInProgress:
		return "[>]"
	case StatusFailed:
		return "[!]"
	case StatusRejected:
		return "[-]"
	default:
		return "[ ]"
	}
}
