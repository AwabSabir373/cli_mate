package ui

import (
	"fmt"
	"strings"
	"time"
)

// bgTerminalSession represents an active background terminal session.
type bgTerminalSession struct {
	ID        string
	Command   string
	Status    string // "running", "completed", "failed"
	StartedAt time.Time
	PID       int
}

// bgTerminalManager manages background terminal sessions in the UI.
type bgTerminalManager struct {
	sessions []bgTerminalSession
	visible  bool
}

// newBGTerminalManager creates a new background terminal manager.
func newBGTerminalManager() *bgTerminalManager {
	return &bgTerminalManager{}
}

// setSessions updates the list of active sessions.
func (bgm *bgTerminalManager) setSessions(sessions []bgTerminalSession) {
	bgm.sessions = sessions
}

// addSession adds a new session.
func (bgm *bgTerminalManager) addSession(session bgTerminalSession) {
	bgm.sessions = append(bgm.sessions, session)
}

// removeSession removes a session by ID.
func (bgm *bgTerminalManager) removeSession(id string) {
	for i, s := range bgm.sessions {
		if s.ID == id {
			bgm.sessions = append(bgm.sessions[:i], bgm.sessions[i+1:]...)
			return
		}
	}
}

// toggleVisibility toggles the visibility of the background terminal view.
func (bgm *bgTerminalManager) toggleVisibility() {
	bgm.visible = !bgm.visible
}

// isVisible returns whether the background terminal view is visible.
func (bgm *bgTerminalManager) isVisible() bool {
	return bgm.visible && len(bgm.sessions) > 0
}

// activeCount returns the number of running sessions.
func (bgm *bgTerminalManager) activeCount() int {
	count := 0
	for _, s := range bgm.sessions {
		if s.Status == "running" {
			count++
		}
	}
	return count
}

// summary returns a short summary string of active sessions.
func (bgm *bgTerminalManager) summary() string {
	count := bgm.activeCount()
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("%d bg session(s) running", count)
}

// render renders the background terminal view.
func (bgm *bgTerminalManager) render(width int, styles appStyles) string {
	if !bgm.isVisible() {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render("Background Terminals"))
	b.WriteString("\n\n")

	if len(bgm.sessions) == 0 {
		b.WriteString(styles.muted.Render("No background sessions."))
		return b.String()
	}

	for _, session := range bgm.sessions {
		// Status icon
		var icon string
		switch session.Status {
		case "running":
			icon = styles.accent.Render("●")
		case "completed":
			icon = styles.success.Render("✓")
		case "failed":
			icon = styles.error.Render("✗")
		default:
			icon = "○"
		}

		elapsed := time.Since(session.StartedAt).Round(time.Second)

		b.WriteString(fmt.Sprintf("%s %s", icon, session.ID))
		b.WriteString("\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %s", truncateString(session.Command, 50))))
		b.WriteString("\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %s elapsed", elapsed)))
		b.WriteString("\n\n")
	}

	b.WriteString(styles.scrollHint.Render("Use /bg-status to view · /bg-kill <id> to stop"))

	return b.String()
}

// renderBGTerminalSummary renders an inline summary for the header.
func renderBGTerminalSummary(bgm *bgTerminalManager, styles appStyles) string {
	if bgm == nil {
		return ""
	}
	summary := bgm.summary()
	if summary == "" {
		return ""
	}
	return styles.muted.Render(summary)
}
