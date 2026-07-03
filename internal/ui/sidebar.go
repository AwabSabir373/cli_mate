package ui

import (
	"fmt"
	"strings"
)

// Sidebar displays session info, touched files, and plan in a right-side panel.
type Sidebar struct {
	visible      bool
	sessionInfo  SessionInfo
	planPanel    *PlanPanel
	touchedFiles []touchedFile
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

// hasContent returns true if the sidebar has any content to show.
func (s *Sidebar) hasContent() bool {
	return s.sessionInfo.Provider != "" ||
		s.sessionInfo.Model != "" ||
		(s.planPanel != nil && s.planPanel.IsVisible()) ||
		len(s.touchedFiles) > 0
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

	// Session info section
	section := s.renderSessionInfo(styles)
	if section != "" {
		sections = append(sections, section)
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

	return styles.panel.
		Width(sidebarWidth).
		Height(height - 2).
		Render(content)
}

func (s *Sidebar) renderSessionInfo(styles appStyles) string {
	var lines []string
	lines = append(lines, styles.sidebarTitle.Render("Session"))
	lines = append(lines, "")

	if s.sessionInfo.Provider != "" {
		lines = append(lines, styles.muted.Render("Provider:")+" "+s.sessionInfo.Provider)
	}
	if s.sessionInfo.Model != "" {
		lines = append(lines, styles.muted.Render("Model:")+" "+s.sessionInfo.Model)
	}
	if s.sessionInfo.Branch != "" {
		lines = append(lines, styles.muted.Render("Branch:")+" "+s.sessionInfo.Branch)
	}
	if s.sessionInfo.Style != "" {
		lines = append(lines, styles.muted.Render("Style:")+" "+s.sessionInfo.Style)
	}
	if s.sessionInfo.Messages > 0 {
		lines = append(lines, styles.muted.Render(fmt.Sprintf("Messages: %d", s.sessionInfo.Messages)))
	}

	return strings.Join(lines, "\n")
}
