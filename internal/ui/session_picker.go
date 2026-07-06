package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"cli_mate/internal/agent"
	"cli_mate/internal/storage"
)

// sessionPicker manages the interactive session selection UI.
type sessionPicker struct {
	visible   bool
	sessions  []storage.SessionRecord
	cursor    int
	scrollOff int
	loaded    bool
	err       string
}

// sessionResumeMsg carries the result of a session resume selection.
type sessionResumeMsg struct {
	sessionID string
	messages  []agent.Message
	err       error
}

// newSessionPicker creates a new session picker.
func newSessionPicker() *sessionPicker {
	return &sessionPicker{}
}

// loadSessions loads sessions from the store.
func (sp *sessionPicker) loadSessions(ctx context.Context, store storage.SessionStore) {
	if store == nil {
		sp.err = "No session store available"
		return
	}
	sessions, err := store.ListSessions(ctx)
	if err != nil {
		sp.err = err.Error()
		return
	}
	// Filter out empty sessions (no messages)
	var valid []storage.SessionRecord
	for _, s := range sessions {
		msgs, err := store.Messages(ctx, s.ID)
		if err == nil && len(msgs) > 0 {
			valid = append(valid, s)
		}
	}
	sp.sessions = valid
	sp.loaded = true
	sp.cursor = 0
	sp.scrollOff = 0
}

// show opens the session picker.
func (sp *sessionPicker) show() {
	sp.visible = true
	sp.cursor = 0
	sp.scrollOff = 0
}

// hide closes the session picker.
func (sp *sessionPicker) hide() {
	sp.visible = false
	sp.sessions = nil
	sp.loaded = false
	sp.err = ""
}

// isVisible returns true if the picker is visible.
func (sp *sessionPicker) isVisible() bool {
	return sp.visible
}

// handleKey processes a keypress and returns (selectedID string, finished bool).
func (sp *sessionPicker) handleKey(key string) (string, bool) {
	if !sp.visible {
		return "", false
	}

	switch key {
	case "up", "shift+tab":
		if sp.cursor > 0 {
			sp.cursor--
		}
		sp.adjustScroll()
	case "down", "tab":
		if sp.cursor < len(sp.sessions)-1 {
			sp.cursor++
		}
		sp.adjustScroll()
	case "enter", " ":
		if sp.cursor >= 0 && sp.cursor < len(sp.sessions) {
			selected := sp.sessions[sp.cursor].ID
			sp.visible = false
			return selected, true
		}
	case "esc":
		sp.visible = false
		return "", true
	case "delete", "backspace":
		// Delete the selected session
		if sp.cursor >= 0 && sp.cursor < len(sp.sessions) {
			sp.sessions = append(sp.sessions[:sp.cursor], sp.sessions[sp.cursor+1:]...)
			if sp.cursor >= len(sp.sessions) {
				sp.cursor = len(sp.sessions) - 1
			}
		}
	}

	return "", false
}

func (sp *sessionPicker) adjustScroll() {
	maxVisible := 10
	if sp.cursor < sp.scrollOff {
		sp.scrollOff = sp.cursor
	}
	if sp.cursor >= sp.scrollOff+maxVisible {
		sp.scrollOff = sp.cursor - maxVisible + 1
	}
}

// loadSessionsCmd returns a command that loads sessions from the store.
func loadSessionsCmd(store storage.SessionStore, picker *sessionPicker) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		picker.loadSessions(ctx, store)
		return picker
	}
}

// renderSessionPicker renders the session picker UI.
func renderSessionPicker(sp *sessionPicker, styles appStyles, _ int) string {
	if !sp.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" Resume Session "))
	b.WriteString("\n\n")

	if !sp.loaded {
		b.WriteString(styles.muted.Render("  Loading sessions..."))
		b.WriteString("\n")
		return b.String()
	}

	if sp.err != "" {
		b.WriteString(styles.error.Render(sp.err))
		b.WriteString("\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	if len(sp.sessions) == 0 {
		b.WriteString(styles.muted.Render("  No previous sessions found."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(styles.muted.Render(fmt.Sprintf("  %d session(s) available:", len(sp.sessions))))
	b.WriteString("\n\n")

	maxVisible := 10
	start := sp.scrollOff
	end := start + maxVisible
	if end > len(sp.sessions) {
		end = len(sp.sessions)
	}

	for i := start; i < end; i++ {
		session := sp.sessions[i]
		title := session.Title
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		timeStr := session.UpdatedAt.Format("Jan 02 15:04")

		label := fmt.Sprintf("%s  %s", timeStr, title)
		if i == sp.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", label)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", label))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter resume · Del delete · Esc close"))
	b.WriteString("\n")
	return b.String()
}

// formatSessionSummary formats a session for display after resume.
func formatSessionSummary(rec storage.SessionRecord, msgCount int) string {
	return fmt.Sprintf("Resumed session %s (%d messages, last active %s)",
		rec.Title, msgCount, rec.UpdatedAt.Format(time.RFC822))
}

// resumeSession loads messages from a session and returns them.
func resumeSession(ctx context.Context, store storage.SessionStore, sessionID string) ([]agent.Message, error) {
	if store == nil {
		return nil, fmt.Errorf("no session store available")
	}
	return store.Messages(ctx, sessionID)
}

// updateSessionTitle updates the title of a session.
func updateSessionTitle(store storage.SessionStore, sessionID string, title string) {
	if store == nil || sessionID == "" || title == "" {
		return
	}
	// For now, we just track the title in memory since the SQLite store
	// doesn't have an UpdateSession method. We'll update the title by
	// recreating the session.
	ctx := context.Background()
	_ = store.CreateSession(ctx, storage.SessionRecord{
		ID:    sessionID,
		Title: title,
	})
}
