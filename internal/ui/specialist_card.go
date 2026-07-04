package ui

import (
	"fmt"
	"strings"
	"time"
)

// specialistKind identifies the type of specialist agent.
type specialistKind string

const (
	specialistCodeReview   specialistKind = "code-reviewer"
	specialistFilePicker   specialistKind = "file-picker"
	specialistWebSearch    specialistKind = "web-search"
	specialistResearcher   specialistKind = "researcher"
	specialistBasher       specialistKind = "basher"
	specialistBrowserUse   specialistKind = "browser-use"
	specialistThinker      specialistKind = "thinker"
	specialistCodeSearcher specialistKind = "code-searcher"
)

// specialistCard represents a spawned specialist agent card.
type specialistCard struct {
	kind      specialistKind
	label     string
	prompt    string
	status    string // "running", "completed", "failed"
	startedAt time.Time
	duration  time.Duration
	expanded  bool
}

// newSpecialistCard creates a new specialist card.
func newSpecialistCard(kind specialistKind, prompt string) *specialistCard {
	return &specialistCard{
		kind:      kind,
		label:     specialistLabel(kind),
		prompt:    prompt,
		status:    "running",
		startedAt: time.Now(),
		expanded:  false,
	}
}

// specialistLabel returns the display label for a specialist kind.
func specialistLabel(kind specialistKind) string {
	switch kind {
	case specialistCodeReview:
		return "Code Reviewer"
	case specialistFilePicker:
		return "File Picker"
	case specialistWebSearch:
		return "Web Search"
	case specialistResearcher:
		return "Researcher"
	case specialistBasher:
		return "Bash Executor"
	case specialistBrowserUse:
		return "Browser Automation"
	case specialistThinker:
		return "Deep Thinker"
	case specialistCodeSearcher:
		return "Code Searcher"
	default:
		return "Specialist"
	}
}

// specialistIcon returns an emoji icon for the specialist kind.
func specialistIcon(kind specialistKind) string {
	switch kind {
	case specialistCodeReview:
		return "🔍"
	case specialistFilePicker:
		return "📂"
	case specialistWebSearch:
		return "🌐"
	case specialistResearcher:
		return "📚"
	case specialistBasher:
		return "💻"
	case specialistBrowserUse:
		return "🖥️"
	case specialistThinker:
		return "🧠"
	case specialistCodeSearcher:
		return "🔎"
	default:
		return "⚙"
	}
}

// complete marks the specialist as completed.
func (sc *specialistCard) complete() {
	sc.status = "completed"
	sc.duration = time.Since(sc.startedAt)
}

// fail marks the specialist as failed.
func (sc *specialistCard) fail() {
	sc.status = "failed"
	sc.duration = time.Since(sc.startedAt)
}

// toggleExpanded toggles the expanded state.
func (sc *specialistCard) toggleExpanded() {
	sc.expanded = !sc.expanded
}

// renderSpecialistCard renders a specialist card in the transcript.
func renderSpecialistCard(sc *specialistCard, styles appStyles, width int) string {
	if sc == nil {
		return ""
	}

	sc.duration = time.Since(sc.startedAt)

	var b strings.Builder

	// Status indicator
	var statusStr string
	switch sc.status {
	case "running":
		statusStr = styles.spinner.Render("●")
	case "completed":
		statusStr = styles.success.Render("✓")
	case "failed":
		statusStr = styles.error.Render("✗")
	}

	// Icon and label
	icon := specialistIcon(sc.kind)
	label := specialistLabel(sc.kind)

	if sc.expanded {
		b.WriteString(fmt.Sprintf("%s %s %s  %s",
			styles.roleSystem.Render("▼"),
			statusStr,
			styles.roleTool.Render(fmt.Sprintf("%s %s", icon, label)),
			styles.muted.Render(fmt.Sprintf("(%s)", sc.duration.Round(time.Second))),
		))
		b.WriteString("\n")
		if sc.prompt != "" {
			truncated := sc.prompt
			if len(truncated) > width-20 {
				truncated = truncated[:width-23] + "..."
			}
			b.WriteString(styles.muted.Render(fmt.Sprintf("  %s", truncated)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(fmt.Sprintf("%s %s %s  %s",
			styles.roleSystem.Render("▶"),
			statusStr,
			styles.roleTool.Render(fmt.Sprintf("%s %s", icon, label)),
			styles.muted.Render(fmt.Sprintf("(%s)", sc.duration.Round(time.Second))),
		))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSpecialistSummaryCard renders a compact summary of all spawned specialists.
func renderSpecialistSummaryCard(cards []*specialistCard, styles appStyles) string {
	if len(cards) == 0 {
		return ""
	}

	var b strings.Builder
	icon := styles.roleSystem.Render("🔄")
	b.WriteString(fmt.Sprintf("%s %s",
		icon,
		styles.muted.Render(fmt.Sprintf("Spawned %d specialist(s)", len(cards))),
	))
	b.WriteString("\n")

	for _, sc := range cards {
		statusIcon := "○"
		switch sc.status {
		case "running":
			statusIcon = styles.spinner.Render("●")
		case "completed":
			statusIcon = styles.success.Render("✓")
		case "failed":
			statusIcon = styles.error.Render("✗")
		}
		b.WriteString(fmt.Sprintf("  %s %s %s",
			statusIcon,
			specialistIcon(sc.kind),
			specialistLabel(sc.kind),
		))
		b.WriteString("\n")
	}

	return b.String()
}
