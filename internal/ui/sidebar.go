package ui

import (
	"fmt"
	"strings"
	"time"
)

// Sidebar displays session info, touched files, git status, and plan in a right-side panel.
type Sidebar struct {
	visible      bool
	sessionInfo  SessionInfo
	planPanel    *PlanPanel
	touchedFiles []touchedFile
	gitBranch    string
	gitStatus    string // short git status summary
	sessionTime  time.Duration
}

// SessionInfo contains information about the current session.
type SessionInfo struct {
	Provider string
	Model    string
	Branch   string
	Style    string
	Messages int
}

// NewSidebar creates a new sidebar.
func NewSidebar(planPanel *PlanPanel) *Sidebar {
	return &Sidebar{
		visible:   false,
		planPanel: planPanel,
	}
}

// Toggle shows/hides the sidebar.
func (s *Sidebar) Toggle() {
	s.visible = !s.visible
}

// SetVisible sets the visibility of the sidebar.
func (s *Sidebar) SetVisible(visible bool) {
	s.visible = visible
}

// IsVisible returns whether the sidebar is visible.
func (s *Sidebar) IsVisible() bool {
	return s.visible
}

// SetSessionInfo updates the session information.
func (s *Sidebar) SetSessionInfo(info SessionInfo) {
	s.sessionInfo = info
}

// SetTouchedFiles updates the touched files list.
func (s *Sidebar) SetTouchedFiles(files []touchedFile) {
	s.touchedFiles = files
}

// SetGitInfo updates the git branch and status.
func (s *Sidebar) SetGitInfo(branch, status string) {
	s.gitBranch = branch
	s.gitStatus = status
}

// hasContent returns true if the sidebar has any content to show.
func (s *Sidebar) hasContent() bool {
	return s.sessionInfo.Provider != "" ||
		s.sessionInfo.Model != "" ||
		(s.planPanel != nil && s.planPanel.IsVisible()) ||
		len(s.touchedFiles) > 0 ||
		s.gitBranch != ""
}

// SetDimensions explicitly assigns width and height to the sidebar.
// Called during resize to enforce top-down sizing propagation.
func (s *Sidebar) SetDimensions(width, height int) {
	// Store dimensions for use during render (no-op for now,
	// the Render method already receives these as parameters).
	_ = width
	_ = height
}

// Render produces the sidebar view.
func (s *Sidebar) Render(width int, height int, styles appStyles) string {
	if !s.visible {
		return ""
	}

	sidebarWidth := width
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}

	var sections []string

	// Workspace/session header
	header := s.renderHeader(styles)
	if header != "" {
		sections = append(sections, header)
	}

	// Session info section
	section := s.renderSessionInfo(styles)
	if section != "" {
		sections = append(sections, section)
	}

	// Git status section
	gitSection := s.renderGitStatus(styles)
	if gitSection != "" {
		sections = append(sections, gitSection)
	}

	// Plan panel section
	if s.planPanel != nil && s.planPanel.IsVisible() {
		planSection := s.planPanel.Render(sidebarWidth-4, styles)
		if planSection != "" {
			sections = append(sections, planSection)
		}
	}

	// Touched files section
	if len(s.touchedFiles) > 0 {
		filesSection := renderTouchedFiles(s.touchedFiles, 8, styles)
		if filesSection != "" {
			sections = append(sections, filesSection)
		}
	}

	// Keyboard hints at bottom
	hints := s.renderHints(styles)
	if hints != "" {
		sections = append(sections, hints)
	}

	if len(sections) == 0 {
		return ""
	}

	content := strings.Join(sections, "\n\n")

	// Pad to fill height if needed
	lines := strings.Split(content, "\n")
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	content = strings.Join(lines, "\n")

	// Border consolidation: sidebar uses no internal border.
	// Borders are applied only at the layout composition tier (the outer panel).
	return styles.sidebarPanel.
		Width(sidebarWidth).
		Height(height).
		Render(content)
}

func (s *Sidebar) renderHeader(styles appStyles) string {
	return styles.sidebarTitle.Render("cli_mate")
}

func (s *Sidebar) renderSessionInfo(styles appStyles) string {
	var lines []string
	lines = append(lines, styles.sidebarTitle.Render("Session"))
	lines = append(lines, "")

	if s.sessionInfo.Provider != "" {
		lines = append(lines, fmt.Sprintf("%s %s",
			styles.muted.Render("Provider:"),
			styles.accent.Render(s.sessionInfo.Provider),
		))
	}
	if s.sessionInfo.Model != "" {
		lines = append(lines, fmt.Sprintf("%s %s",
			styles.muted.Render("Model:"),
			s.sessionInfo.Model,
		))
	}
	if s.sessionInfo.Branch != "" {
		lines = append(lines, fmt.Sprintf("%s %s",
			styles.muted.Render("Branch:"),
			s.sessionInfo.Branch,
		))
	}
	if s.sessionInfo.Style != "" {
		lines = append(lines, fmt.Sprintf("%s %s",
			styles.muted.Render("Style:"),
			s.sessionInfo.Style,
		))
	}
	if s.sessionInfo.Messages > 0 {
		lines = append(lines, fmt.Sprintf("%s %d",
			styles.muted.Render("Messages:"),
			s.sessionInfo.Messages,
		))
	}

	return strings.Join(lines, "\n")
}

func (s *Sidebar) renderGitStatus(styles appStyles) string {
	if s.gitBranch == "" {
		return ""
	}

	var lines []string
	lines = append(lines, styles.sidebarTitle.Render("Git"))
	lines = append(lines, "")

	branchLabel := s.gitBranch
	if len(branchLabel) > 20 {
		branchLabel = branchLabel[:20] + "..."
	}
	lines = append(lines, fmt.Sprintf("%s %s",
		styles.muted.Render("Branch:"),
		styles.accent.Render(branchLabel),
	))

	if s.gitStatus != "" {
		lines = append(lines, fmt.Sprintf("%s %s",
			styles.muted.Render("Status:"),
			s.gitStatus,
		))
	}

	return strings.Join(lines, "\n")
}

func (s *Sidebar) renderHints(styles appStyles) string {
	return styles.muted.Render("Ctrl+B toggle · D detail")
}
