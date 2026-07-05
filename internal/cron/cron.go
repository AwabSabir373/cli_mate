// Package cron provides scheduled agent job execution.
// Jobs are stored as JSON files and executed by a background scheduler.
package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	StatusActive = "active"
	StatusPaused = "paused"
)

// Job is a stored scheduled job.
type Job struct {
	ID        string    `json:"id"`
	Expr      string    `json:"expr"`   // Simple cron expression: "@hourly", "@daily", "@weekly", or "*/5 * * * *"
	Prompt    string    `json:"prompt"` // The prompt to send to the agent
	Cwd       string    `json:"cwd,omitempty"`
	Model     string    `json:"model,omitempty"`
	Status    string    `json:"status"`
	FireCount int       `json:"fireCount"`
	NextRunAt time.Time `json:"nextRunAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// RunRecord is one fire's outcome.
type RunRecord struct {
	JobID  string    `json:"jobId"`
	At     time.Time `json:"at"`
	Status string    `json:"status"`
	Error  string    `json:"error,omitempty"`
}

// Store manages cron jobs persisted as JSON files.
type Store struct {
	root string
	mu   sync.RWMutex
}

// NewStore creates a store at the given directory.
func NewStore(root string) *Store {
	return &Store{root: root}
}

// Add creates a new job and persists it.
func (s *Store) Add(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job.ID == "" {
		job.ID = fmt.Sprintf("job_%d", time.Now().UnixNano())
	}
	if job.Status == "" {
		job.Status = StatusActive
	}
	job.CreatedAt = time.Now()
	if job.NextRunAt.IsZero() {
		job.NextRunAt = NextRunTime(job.Expr, time.Now())
	}

	if err := os.MkdirAll(s.root, 0700); err != nil {
		return err
	}

	path := filepath.Join(s.root, job.ID+".json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Get returns a job by ID.
func (s *Store) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.root, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// List returns all jobs sorted by next run time.
func (s *Store) List() ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var jobs []Job
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, entry.Name()))
		if err != nil {
			continue
		}
		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].NextRunAt.Before(jobs[j].NextRunAt)
	})
	return jobs, nil
}

// Remove deletes a job by ID.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.root, id+".json")
	return os.Remove(path)
}

// Update modifies a job and persists the changes.
func (s *Store) Update(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.root, job.ID+".json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// DueJobs returns jobs that should fire by the given time.
func (s *Store) DueJobs(now time.Time) ([]Job, error) {
	jobs, err := s.List()
	if err != nil {
		return nil, err
	}

	var due []Job
	for _, job := range jobs {
		if job.Status == StatusActive && !job.NextRunAt.After(now) {
			due = append(due, job)
		}
	}
	return due, nil
}

// NextRunTime calculates the next run time for a cron expression.
func NextRunTime(expr string, from time.Time) time.Time {
	switch strings.ToLower(expr) {
	case "@hourly":
		return from.Truncate(time.Hour).Add(time.Hour)
	case "@daily", "@midnight":
		return time.Date(from.Year(), from.Month(), from.Day()+1, 0, 0, 0, 0, from.Location())
	case "@weekly":
		daysUntilMonday := (8 - int(from.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		return time.Date(from.Year(), from.Month(), from.Day()+daysUntilMonday, 0, 0, 0, 0, from.Location())
	case "@monthly":
		return time.Date(from.Year(), from.Month()+1, 1, 0, 0, 0, 0, from.Location())
	default:
		// Try to parse simple interval like "*/5 * * * *"
		if strings.HasPrefix(expr, "*/") {
			var minutes int
			if _, err := fmt.Sscanf(expr, "*/%d", &minutes); err == nil && minutes > 0 {
				return from.Truncate(time.Duration(minutes) * time.Minute).Add(time.Duration(minutes) * time.Minute)
			}
		}
		// Default: next hour
		return from.Truncate(time.Hour).Add(time.Hour)
	}
}

// FormatJob returns a human-readable summary of a job.
func FormatJob(job Job) string {
	status := "active"
	if job.Status == StatusPaused {
		status = "paused"
	}
	return fmt.Sprintf("[%s] %s (fired %d times, next: %s, status: %s)",
		job.ID, truncate(job.Prompt, 50), job.FireCount,
		job.NextRunAt.Format("2006-01-02 15:04"), status)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
