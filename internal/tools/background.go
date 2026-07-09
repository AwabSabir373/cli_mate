package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type BackgroundSession struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	StartedAt time.Time `json:"startedAt"`
	Status    string    `json:"status"` // "running", "completed", "failed", "killed"
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type BackgroundManager struct {
	sessions map[string]*BackgroundSession
	mu       sync.RWMutex
	logDir   string
}

func NewBackgroundManager(workspaceRoot string) *BackgroundManager {
	logDir := filepath.Join(workspaceRoot, ".cli_mate", "bg_sessions")
	os.MkdirAll(logDir, 0700)
	m := &BackgroundManager{
		sessions: make(map[string]*BackgroundSession),
		logDir:   logDir,
	}
	// Load persisted sessions on startup
	m.loadSessions()
	return m
}

func (m *BackgroundManager) StartSession(command string) *BackgroundSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("bg_%d", time.Now().UnixMilli())
	session := &BackgroundSession{
		ID:        id,
		Command:   command,
		StartedAt: time.Now(),
		Status:    "running",
	}
	m.sessions[id] = session

	go m.runCommand(session)

	return session
}

func (m *BackgroundManager) runCommand(session *BackgroundSession) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Parse and validate the command against the whitelist
	name, args, err := parseCommand(session.Command)
	if err != nil {
		m.mu.Lock()
		session.Status = "failed"
		session.Error = fmt.Sprintf("command rejected: %v", err)
		session.Output = ""
		m.saveSession(session)
		m.mu.Unlock()
		return
	}
	if err := validateCommand(name); err != nil {
		m.mu.Lock()
		session.Status = "failed"
		session.Error = err.Error()
		session.Output = ""
		m.saveSession(session)
		m.mu.Unlock()
		return
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = m.logDir

	output, err := cmd.CombinedOutput()

	m.mu.Lock()
	session.Output = string(output)
	if err != nil {
		session.Status = "failed"
		session.Error = err.Error()
	} else {
		session.Status = "completed"
	}
	m.saveSession(session)
	m.mu.Unlock()
}

func (m *BackgroundManager) ListSessions() []*BackgroundSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*BackgroundSession
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

func (m *BackgroundManager) GetSession(id string) (*BackgroundSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *BackgroundManager) KillSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	if s.Status != "running" {
		return fmt.Errorf("session %s is not running", id)
	}
	s.Status = "killed"
	m.saveSession(s)
	return nil
}

// saveSession persists a session to disk.
func (m *BackgroundManager) saveSession(session *BackgroundSession) {
	path := filepath.Join(m.logDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0600)
}

// loadSessions loads persisted sessions from disk.
func (m *BackgroundManager) loadSessions() {
	entries, err := os.ReadDir(m.logDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.logDir, entry.Name()))
		if err != nil {
			continue
		}
		var session BackgroundSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		// Mark stale running sessions as failed
		if session.Status == "running" {
			session.Status = "failed"
			session.Error = "session lost on restart"
		}
		m.sessions[session.ID] = &session
	}
}

type BGRunTool struct {
	Manager *BackgroundManager
}

func NewBGRunTool(manager *BackgroundManager) *BGRunTool {
	return &BGRunTool{Manager: manager}
}

func (t *BGRunTool) Name() string {
	return "bg_run"
}

func (t *BGRunTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Run a command in the background. Use this for long-running tasks that should not block the conversation.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"command"},
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to run in background",
				},
			},
			"description": "Run a command in the background and return immediately.",
		},
	}
}

func (t *BGRunTool) Execute(_ context.Context, call Call) (Result, error) {
	command, _ := call.Argument["command"].(string)
	if strings.TrimSpace(command) == "" {
		return Result{Error: "command is required"}, fmt.Errorf("command is required")
	}

	session := t.Manager.StartSession(command)
	return Result{Content: fmt.Sprintf("Background session started: %s\nCommand: %s", session.ID, session.Command)}, nil
}

type BGStatusTool struct {
	Manager *BackgroundManager
}

func NewBGStatusTool(manager *BackgroundManager) *BGStatusTool {
	return &BGStatusTool{Manager: manager}
}

func (t *BGStatusTool) Name() string {
	return "bg_status"
}

func (t *BGStatusTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Check the status of background sessions.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "Optional session ID to check. If not provided, lists all sessions.",
				},
			},
			"description": "Check status of background sessions.",
		},
	}
}

func (t *BGStatusTool) Execute(_ context.Context, call Call) (Result, error) {
	sessionID, _ := call.Argument["session_id"].(string)

	if sessionID != "" {
		session, ok := t.Manager.GetSession(sessionID)
		if !ok {
			return Result{Error: fmt.Sprintf("session %s not found", sessionID)}, fmt.Errorf("session %s not found", sessionID)
		}
		return Result{Content: formatSession(session)}, nil
	}

	sessions := t.Manager.ListSessions()
	if len(sessions) == 0 {
		return Result{Content: "No background sessions."}, nil
	}

	var b strings.Builder
	b.WriteString("Background Sessions:\n\n")
	for _, s := range sessions {
		b.WriteString(formatSession(s))
		b.WriteString("\n")
	}
	return Result{Content: b.String()}, nil
}

func formatSession(s *BackgroundSession) string {
	return fmt.Sprintf("ID: %s\nCommand: %s\nStatus: %s\nStarted: %s\n",
		s.ID, s.Command, s.Status, s.StartedAt.Format("15:04:05"))
}

type BGLogsTool struct {
	Manager *BackgroundManager
}

func NewBGLogsTool(manager *BackgroundManager) *BGLogsTool {
	return &BGLogsTool{Manager: manager}
}

func (t *BGLogsTool) Name() string {
	return "bg_logs"
}

func (t *BGLogsTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Show logs/output of a background session.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "The session ID to get logs for",
				},
			},
			"description": "Show logs of a background session.",
		},
	}
}

func (t *BGLogsTool) Execute(_ context.Context, call Call) (Result, error) {
	sessionID, _ := call.Argument["session_id"].(string)
	if strings.TrimSpace(sessionID) == "" {
		return Result{Error: "session_id is required"}, fmt.Errorf("session_id is required")
	}

	session, ok := t.Manager.GetSession(sessionID)
	if !ok {
		return Result{Error: fmt.Sprintf("session %s not found", sessionID)}, fmt.Errorf("session %s not found", sessionID)
	}

	output := session.Output
	if output == "" {
		output = "(no output yet)"
	}

	return Result{Content: fmt.Sprintf("Session %s (%s):\n\n%s", session.ID, session.Status, output)}, nil
}

type BGKillTool struct {
	Manager *BackgroundManager
}

func NewBGKillTool(manager *BackgroundManager) *BGKillTool {
	return &BGKillTool{Manager: manager}
}

func (t *BGKillTool) Name() string {
	return "bg_kill"
}

func (t *BGKillTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Kill a running background session.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{
					"type":        "string",
					"description": "The session ID to kill",
				},
			},
			"description": "Kill a running background session.",
		},
	}
}

func (t *BGKillTool) Execute(_ context.Context, call Call) (Result, error) {
	sessionID, _ := call.Argument["session_id"].(string)
	if strings.TrimSpace(sessionID) == "" {
		return Result{Error: "session_id is required"}, fmt.Errorf("session_id is required")
	}

	if err := t.Manager.KillSession(sessionID); err != nil {
		return Result{Error: err.Error()}, err
	}

	return Result{Content: fmt.Sprintf("Session %s killed.", sessionID)}, nil
}
