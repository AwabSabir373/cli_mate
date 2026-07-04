package ui

import (
	"fmt"
	"strings"
	"time"
)

// PRStatus represents the status of a pull request.
type PRStatus string

const (
	PROpen      PRStatus = "open"
	PRMerged    PRStatus = "merged"
	PRClosed    PRStatus = "closed"
	PRDraft     PRStatus = "draft"
	PRApproved  PRStatus = "approved"
	PRChangesRequested PRStatus = "changes_requested"
)

// PRInfo contains information about a pull request.
type PRInfo struct {
	Number      int
	Title       string
	Author      string
	Branch      string
	BaseBranch  string
	Status      PRStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Additions   int
	Deletions   int
	ChangedFiles int
	CIStatus    string // "passing", "failing", "pending", ""
	ReviewCount int
	Approved    bool
}

// PRDisplay manages the PR status display for the sidebar.
type PRDisplay struct {
	visible   bool
	prs       []PRInfo
	cursor    int
	scrollOff int
	loaded    bool
	err       string
}

// newPRDisplay creates a new PR display panel.
func newPRDisplay() *PRDisplay {
	return &PRDisplay{}
}

// show opens the PR display.
func (pd *PRDisplay) show() {
	pd.visible = true
	pd.cursor = 0
	pd.scrollOff = 0
}

// hide closes the PR display.
func (pd *PRDisplay) hide() {
	pd.visible = false
}

// isVisible returns true if the PR display is visible.
func (pd *PRDisplay) isVisible() bool {
	return pd.visible
}

// SetPRs sets the PR list and marks as loaded.
func (pd *PRDisplay) SetPRs(prs []PRInfo) {
	pd.prs = prs
	pd.loaded = true
	pd.err = ""
}

// SetError sets an error message.
func (pd *PRDisplay) SetError(err string) {
	pd.err = err
	pd.loaded = true
}

// handleKey processes key events for navigation.
func (pd *PRDisplay) handleKey(key string) string {
	if !pd.visible {
		return ""
	}

	switch key {
	case "up", "shift+tab":
		if pd.cursor > 0 {
			pd.cursor--
		}
		pd.adjustScroll()
	case "down", "tab":
		if pd.cursor < len(pd.prs)-1 {
			pd.cursor++
		}
		pd.adjustScroll()
	case "enter", " ":
		if pd.cursor >= 0 && pd.cursor < len(pd.prs) {
			return fmt.Sprintf("pr_%d", pd.prs[pd.cursor].Number)
		}
	case "esc":
		pd.hide()
	}
	return ""
}

func (pd *PRDisplay) adjustScroll() {
	maxVisible := 8
	if pd.cursor < pd.scrollOff {
		pd.scrollOff = pd.cursor
	}
	if pd.cursor >= pd.scrollOff+maxVisible {
		pd.scrollOff = pd.cursor - maxVisible + 1
	}
}

// prStatusIcon returns an icon for the PR status.
func prStatusIcon(status PRStatus) string {
	switch status {
	case PROpen:
		return "🟢"
	case PRDraft:
		return "🟡"
	case PRMerged:
		return "🟣"
	case PRClosed:
		return "🔴"
	case PRApproved:
		return "✅"
	case PRChangesRequested:
		return "❌"
	default:
		return "⚪"
	}
}

// ciStatusIcon returns an icon for CI status.
func ciStatusIcon(status string) string {
	switch status {
	case "passing":
		return "✓"
	case "failing":
		return "✗"
	case "pending":
		return "○"
	default:
		return ""
	}
}

// renderPRStatus renders the PR status panel.
func renderPRStatus(pd *PRDisplay, styles appStyles, width int) string {
	if !pd.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" Pull Requests "))
	b.WriteString("\n\n")

	if !pd.loaded {
		b.WriteString(styles.muted.Render("  Loading PRs..."))
		b.WriteString("\n")
		return b.String()
	}

	if pd.err != "" {
		b.WriteString(styles.error.Render(pd.err))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	if len(pd.prs) == 0 {
		b.WriteString(styles.muted.Render("  No open pull requests."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	maxVisible := 8
	start := pd.scrollOff
	end := start + maxVisible
	if end > len(pd.prs) {
		end = len(pd.prs)
	}

	for i := start; i < end; i++ {
		pr := pd.prs[i]

		title := pr.Title
		if len(title) > 35 {
			title = title[:35] + "..."
		}

		icon := prStatusIcon(pr.Status)
		line := fmt.Sprintf("%s #%d %s", icon, pr.Number, title)

		if i == pd.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", line)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", line))
			b.WriteString("\n")
		}

		// Details on next line
		details := fmt.Sprintf("   %s", pr.Author)
		if pr.Additions > 0 || pr.Deletions > 0 {
			details += fmt.Sprintf("  +%d/-%d", pr.Additions, pr.Deletions)
		}
		if pr.CIStatus != "" {
			ci := ciStatusIcon(pr.CIStatus)
			if ci != "" {
				details += fmt.Sprintf("  CI:%s", ci)
			}
		}
		b.WriteString(styles.muted.Render(details))
		b.WriteString("\n")
	}

	if len(pd.prs) > maxVisible {
		b.WriteString("\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  ... %d more PR(s) ...", len(pd.prs)-maxVisible)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter view · Esc close"))
	b.WriteString("\n")
	return b.String()
}

// renderPRSummary renders a compact PR summary for the sidebar.
func renderPRSummary(prs []PRInfo, styles appStyles) string {
	if len(prs) == 0 {
		return ""
	}

	openCount := 0
	draftCount := 0
	for _, pr := range prs {
		switch pr.Status {
		case PROpen:
			openCount++
		case PRDraft:
			draftCount++
		}
	}

	summary := fmt.Sprintf("%s PRs: %d open",
		prStatusIcon(PROpen), openCount,
	)
	if draftCount > 0 {
		summary += fmt.Sprintf(", %d draft", draftCount)
	}

	return styles.muted.Render(summary)
}
