package ui

import (
	"fmt"
	"strings"
	"time"

	"cli_mate/internal/providers"
)

// sessionControlAction represents a session control action.
type sessionControlAction string

const (
	actionRewind  sessionControlAction = "rewind"
	actionSave    sessionControlAction = "save"
	actionExport  sessionControlAction = "export"
	actionRename  sessionControlAction = "rename"
	actionCompact sessionControlAction = "compact"
)

// rewindCheckpoint represents a save point for rewind.
type rewindCheckpoint struct {
	index    int    // message index to rewind to
	label    string // user-provided or auto-generated label
	created  time.Time
	messages int // number of messages at this checkpoint
}

// sessionControls manages session-level operations.
type sessionControls struct {
	visible      bool
	action       sessionControlAction
	checkpoints  []rewindCheckpoint
	cursor       int
	scrollOff    int
	err          string
	renameInput  string
	exportPath   string
	completedMsg string
}

// newSessionControls creates new session controls.
func newSessionControls() *sessionControls {
	return &sessionControls{}
}

// show opens the session controls panel.
func (sc *sessionControls) show() {
	sc.visible = true
	sc.action = ""
	sc.cursor = 0
	sc.err = ""
	sc.completedMsg = ""
}

// hide closes the session controls panel.
func (sc *sessionControls) hide() {
	sc.visible = false
	sc.action = ""
	sc.err = ""
	sc.completedMsg = ""
	sc.renameInput = ""
	sc.exportPath = ""
}

// isVisible returns true if the controls panel is visible.
func (sc *sessionControls) isVisible() bool {
	return sc.visible
}

// handleKey processes a keypress and returns (action string, finished bool).
func (sc *sessionControls) handleKey(key string) (string, bool) {
	if !sc.visible {
		return "", false
	}

	if sc.completedMsg != "" {
		if key == "enter" || key == "esc" || key == " " {
			sc.hide()
			return "", true
		}
		return "", false
	}

	switch sc.action {
	case actionRename:
		return sc.handleRenameKey(key)
	case actionExport:
		return sc.handleExportKey(key)
	case actionRewind:
		return sc.handleRewindKey(key)
	case actionCompact:
		return sc.handleCompactKey(key)
	default:
		return sc.handleMainKey(key)
	}
}

func (sc *sessionControls) handleMainKey(key string) (string, bool) {
	actions := []struct {
		action sessionControlAction
		label  string
	}{
		{actionRewind, "↩ Rewind to a checkpoint"},
		{actionRename, "✎ Rename session"},
		{actionExport, "📤 Export session"},
		{actionCompact, "📦 Compact session"},
	}

	switch key {
	case "up", "shift+tab":
		if sc.cursor > 0 {
			sc.cursor--
		}
	case "down", "tab":
		if sc.cursor < len(actions)-1 {
			sc.cursor++
		}
	case "enter", " ":
		if sc.cursor >= 0 && sc.cursor < len(actions) {
			sc.action = actions[sc.cursor].action
			sc.cursor = 0
			sc.err = ""
		}
	case "esc":
		sc.hide()
		return "", true
	}

	return "", false
}

func (sc *sessionControls) handleRenameKey(key string) (string, bool) {
	switch key {
	case "enter":
		if sc.renameInput == "" {
			sc.err = "Name cannot be empty."
			return "", false
		}
		sc.completedMsg = fmt.Sprintf("Session renamed to \"%s\"", sc.renameInput)
		return fmt.Sprintf("rename:%s", sc.renameInput), true
	case "esc":
		sc.action = ""
		sc.renameInput = ""
		sc.err = ""
		return "", false
	default:
		if text, ok := keyText(key); ok {
			sc.renameInput += text
		} else if key == "backspace" && len(sc.renameInput) > 0 {
			sc.renameInput = sc.renameInput[:len(sc.renameInput)-1]
		}
	}

	return "", false
}

func (sc *sessionControls) handleExportKey(key string) (string, bool) {
	switch key {
	case "enter":
		sc.completedMsg = "Session exported successfully."
		return "export", true
	case "esc":
		sc.action = ""
		sc.err = ""
		return "", false
	default:
		if text, ok := keyText(key); ok {
			sc.exportPath += text
		} else if key == "backspace" && len(sc.exportPath) > 0 {
			sc.exportPath = sc.exportPath[:len(sc.exportPath)-1]
		}
	}

	return "", false
}

func (sc *sessionControls) handleRewindKey(key string) (string, bool) {
	if len(sc.checkpoints) == 0 {
		if key == "esc" || key == "enter" || key == " " {
			sc.action = ""
			sc.cursor = 0
		}
		return "", false
	}

	switch key {
	case "up", "shift+tab":
		if sc.cursor > 0 {
			sc.cursor--
		}
	case "down", "tab":
		if sc.cursor < len(sc.checkpoints)-1 {
			sc.cursor++
		}
	case "enter", " ":
		cp := sc.checkpoints[clamp(sc.cursor, 0, len(sc.checkpoints)-1)]
		sc.completedMsg = fmt.Sprintf("Rewound to %s", cp.label)
		return fmt.Sprintf("rewind:%d", cp.index), true
	case "esc":
		sc.action = ""
		sc.cursor = 0
	}
	return "", false
}

func (sc *sessionControls) handleCompactKey(key string) (string, bool) {
	switch key {
	case "up", "shift+tab", "down", "tab":
		if sc.cursor == 0 {
			sc.cursor = 1
		} else {
			sc.cursor = 0
		}
	case "enter", " ":
		if sc.cursor == 0 {
			sc.completedMsg = "Conversation marked for compaction."
			return "compact", true
		}
		sc.action = ""
		sc.cursor = 0
	case "esc":
		sc.action = ""
		sc.cursor = 0
	}
	return "", false
}

// addCheckpoint creates a new rewind checkpoint.
func (sc *sessionControls) addCheckpoint(index int, label string) {
	cp := rewindCheckpoint{
		index:    index,
		label:    label,
		created:  time.Now(),
		messages: index,
	}

	// Replace existing checkpoint at same index if any
	for i, c := range sc.checkpoints {
		if c.index == index {
			sc.checkpoints[i] = cp
			return
		}
	}

	sc.checkpoints = append(sc.checkpoints, cp)

	// Sort by index descending (newest first)
	for i := 0; i < len(sc.checkpoints); i++ {
		for j := i + 1; j < len(sc.checkpoints); j++ {
			if sc.checkpoints[j].index > sc.checkpoints[i].index {
				sc.checkpoints[i], sc.checkpoints[j] = sc.checkpoints[j], sc.checkpoints[i]
			}
		}
	}

	// Keep max 10 checkpoints
	if len(sc.checkpoints) > 10 {
		sc.checkpoints = sc.checkpoints[:10]
	}
}

// renderSessionControls renders the session controls panel.
func renderSessionControls(sc *sessionControls, styles appStyles, width int, messages []providers.Message) string {
	if !sc.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" Session Controls "))
	b.WriteString("\n\n")

	// Show completion message
	if sc.completedMsg != "" {
		b.WriteString(styles.success.Render("  " + sc.completedMsg))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press any key to close"))
		b.WriteString("\n")
		return b.String()
	}

	switch sc.action {
	case actionRename:
		sc.renderRename(&b, styles)
	case actionExport:
		sc.renderExport(&b, styles)
	case actionCompact:
		sc.renderCompact(&b, styles, messages)
	case actionRewind:
		sc.renderRewind(&b, styles)
	default:
		sc.renderMainMenu(&b, styles)
	}

	return styles.panel.Width(width - 4).Render(b.String())
}

func (sc *sessionControls) renderMainMenu(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.muted.Render("  Choose an action:"))
	b.WriteString("\n\n")

	actions := []struct {
		action sessionControlAction
		label  string
		desc   string
	}{
		{actionRewind, "↩ Rewind to a checkpoint", "Go back to an earlier conversation state"},
		{actionRename, "✎ Rename session", "Give this session a meaningful name"},
		{actionExport, "📤 Export session", "Save the conversation to a file"},
		{actionCompact, "📦 Compact session", "Summarize older messages and reclaim context"},
	}

	for i, a := range actions {
		if i == sc.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", a.label)))
			b.WriteString("\n")
			b.WriteString(styles.muted.Render(fmt.Sprintf("    %s", a.desc)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", a.label))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Esc close"))
	b.WriteString("\n")
}

func (sc *sessionControls) renderRewind(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.muted.Render("  Available checkpoints:"))
	b.WriteString("\n\n")

	if len(sc.checkpoints) == 0 {
		b.WriteString(styles.muted.Render("  No checkpoints yet. Checkpoints are created automatically"))
		b.WriteString("\n")
		b.WriteString(styles.muted.Render("  during long conversations."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to go back"))
		b.WriteString("\n")
		return
	}

	for i, cp := range sc.checkpoints {
		label := cp.label
		if len(label) > 30 {
			label = label[:30] + "..."
		}
		timeStr := cp.created.Format("15:04:05")
		line := fmt.Sprintf("%s  %d msgs  at %s", label, cp.messages, timeStr)
		if i == sc.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", line)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter rewind to · Esc back"))
	b.WriteString("\n")
}

func (sc *sessionControls) renderRename(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.muted.Render("  Enter a new name for this session:"))
	b.WriteString("\n\n")

	if sc.renameInput == "" {
		b.WriteString(styles.prompt.Render("  Name: "))
		b.WriteString(styles.cursor.Render("█"))
	} else {
		b.WriteString(styles.prompt.Render("  Name: "))
		b.WriteString(styles.input.Render(sc.renameInput))
		b.WriteString(styles.cursor.Render("█"))
	}

	b.WriteString("\n")

	if sc.err != "" {
		b.WriteString("\n")
		b.WriteString(styles.error.Render(sc.err))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Type the name · Enter confirm · Esc back"))
	b.WriteString("\n")
}

func (sc *sessionControls) renderExport(b *strings.Builder, styles appStyles) {
	b.WriteString(styles.muted.Render("  Export session to a file:"))
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("  The session will be saved as Markdown with"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  all messages, tool calls, and results."))
	b.WriteString("\n\n")

	b.WriteString(styles.prompt.Render("  Path: "))
	b.WriteString(styles.input.Render(sc.exportPath))
	if sc.exportPath == "" {
		b.WriteString(styles.cursor.Render("█"))
	}
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("  Leave empty for default (./cli_mate_export.md)"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  Press Enter to export · Esc back"))
	b.WriteString("\n")
}

func (sc *sessionControls) renderCompact(b *strings.Builder, styles appStyles, messages []providers.Message) {
	b.WriteString(styles.muted.Render("  Compact session:"))
	b.WriteString("\n\n")

	if len(messages) == 0 {
		b.WriteString(styles.muted.Render("  No messages to compact."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to go back"))
		b.WriteString("\n")
		return
	}

	msgCount := len(messages)
	b.WriteString(fmt.Sprintf("  %s %d messages will be summarized", styles.accent.Render(fmt.Sprintf("%d", msgCount)), msgCount))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  The summary will replace older messages in the"))
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  conversation, saving context window space."))
	b.WriteString("\n\n")

	options := []string{"✓ Compact Now", "← Cancel"}
	for i, opt := range options {
		if i == sc.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf("  %s", opt)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s", opt))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter confirm · Esc back"))
	b.WriteString("\n")
}
