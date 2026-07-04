package ui

import (
	"fmt"
	"strings"
	"time"
)

// subchatStatus represents the status of a sub-conversation.
type subchatStatus string

const (
	subchatActive    subchatStatus = "active"
	subchatCompleted subchatStatus = "completed"
	subchatCancelled subchatStatus = "cancelled"
)

// subchatEntry represents a single sub-conversation within the main chat.
type subchatEntry struct {
	id          string
	prompt      string
	response    string
	status      subchatStatus
	createdAt   time.Time
	completedAt time.Time
	expanded    bool
}

// subchatManager manages nested conversations.
type subchatManager struct {
	active    bool
	chats     []subchatEntry
	cursor    int
	scrollOff int
}

// newSubchatManager creates a new subchat manager.
func newSubchatManager() *subchatManager {
	return &subchatManager{}
}

// start begins a new sub-conversation.
func (scm *subchatManager) start(id, prompt string) {
	entry := subchatEntry{
		id:        id,
		prompt:    prompt,
		status:    subchatActive,
		createdAt: time.Now(),
		expanded:  true,
	}
	scm.chats = append(scm.chats, entry)
	scm.active = true
}

// complete marks a sub-conversation as completed with a response.
func (scm *subchatManager) complete(id, response string) {
	for i, chat := range scm.chats {
		if chat.id == id {
			scm.chats[i].response = response
			scm.chats[i].status = subchatCompleted
			scm.chats[i].completedAt = time.Now()
			break
		}
	}
}

// cancel cancels a sub-conversation.
func (scm *subchatManager) cancel(id string) {
	for i, chat := range scm.chats {
		if chat.id == id {
			scm.chats[i].status = subchatCancelled
			scm.chats[i].completedAt = time.Now()
			break
		}
	}
}

// close ends the current subchat session.
func (scm *subchatManager) close() {
	scm.active = false
}

// isActive returns true if subchat mode is active.
func (scm *subchatManager) isActive() bool {
	return scm.active
}

// handleKey processes key events.
func (scm *subchatManager) handleKey(key string) string {
	if !scm.active || len(scm.chats) == 0 {
		return ""
	}

	switch key {
	case "up", "shift+tab":
		if scm.cursor > 0 {
			scm.cursor--
		}
		scm.adjustScroll()
	case "down", "tab":
		if scm.cursor < len(scm.chats)-1 {
			scm.cursor++
		}
		scm.adjustScroll()
	case "enter", " ":
		if scm.cursor >= 0 && scm.cursor < len(scm.chats) {
			scm.chats[scm.cursor].expanded = !scm.chats[scm.cursor].expanded
		}
	case "esc":
		scm.close()
		return "subchat_close"
	}
	return ""
}

func (scm *subchatManager) adjustScroll() {
	maxVisible := 10
	if scm.cursor < scm.scrollOff {
		scm.scrollOff = scm.cursor
	}
	if scm.cursor >= scm.scrollOff+maxVisible {
		scm.scrollOff = scm.cursor - maxVisible + 1
	}
}

// renderSubchat renders the subchat overlay.
func renderSubchat(scm *subchatManager, styles appStyles, width int) string {
	if !scm.active || len(scm.chats) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" Sub-Chat "))
	b.WriteString("\n\n")

	maxVisible := 10
	start := scm.scrollOff
	end := start + maxVisible
	if end > len(scm.chats) {
		end = len(scm.chats)
	}

	for i := start; i < end; i++ {
		chat := scm.chats[i]

		statusIcon := styles.spinner.Render("●")
		switch chat.status {
		case subchatCompleted:
			statusIcon = styles.success.Render("✓")
		case subchatCancelled:
			statusIcon = styles.error.Render("✗")
		}

		expandIcon := "▶"
		if chat.expanded {
			expandIcon = "▼"
		}

		line := fmt.Sprintf("%s %s %s", statusIcon, expandIcon, truncateString(chat.prompt, width-20))

		if i == scm.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", line)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", line))
			b.WriteString("\n")
		}

		if chat.expanded && chat.response != "" {
			response := chat.response
			if len(response) > width-20 {
				response = response[:width-23] + "..."
			}
			b.WriteString(styles.muted.Render(fmt.Sprintf("     %s", response)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter expand · Esc close"))
	b.WriteString("\n")
	return b.String()
}
