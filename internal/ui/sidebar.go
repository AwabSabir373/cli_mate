package ui

import (
	"fmt"
	"strings"
)

// Sidebar displays session info, file context, and other metadata.
type Sidebar struct {
	visible      bool
	sessionInfo  SessionInfo
	files        []string
	planPanel    *PlanPanel
}

// SessionInfo contains information about the current session.
type SessionInfo struct {
	Provider string
	Model    string
	Branch   string
	Style    string
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

// SetFiles updates the file list.
func (s *Sidebar) SetFiles(files []string) {
	s.files = files
}

// Render produces the sidebar view.
func (s *Sidebar) Render(width int, height int, styles appStyles) string {
	if !s.visible {
		return ""
	}

 sidebarWidth := 30
	if width < 60 {
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
		section := s.planPanel.Render(sidebarWidth, styles)
		if section != "" {
			sections = append(sections, section)
		}
	}

	// Files section (limited to prevent overflow)
	if len(s.files) > 0 {
		section := s.renderFiles(10, styles)
		if section != "" {
			sections = append(sections, section)
		}
	}

	content := strings.Join(sections, "\n\n")

	// Pad to fill height
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
	lines = append(lines, styles.muted.Render("Session"))
	lines = append(lines, "")

	if s.sessionInfo.Provider != "" {
		lines = append(lines, "Provider: "+s.sessionInfo.Provider)
	}
	if s.sessionInfo.Model != "" {
		lines = append(lines, "Model: "+s.sessionInfo.Model)
	}
	if s.sessionInfo.Branch != "" {
		lines = append(lines, "Branch: "+s.sessionInfo.Branch)
	}
	if s.sessionInfo.Style != "" {
		lines = append(lines, "Style: "+s.sessionInfo.Style)
	}

	return strings.Join(lines, "\n")
}

func (s *Sidebar) renderFiles(maxFiles int, styles appStyles) string {
	var lines []string
	lines = append(lines, styles.muted.Render("Files"))
	lines = append(lines, "")

	shown := s.files
	if len(shown) > maxFiles {
		shown = shown[:maxFiles]
	}
	for _, f := range shown {
		lines = append(lines, "  "+f)
	}
	if len(s.files) > maxFiles {
		lines = append(lines, styles.muted.Render(fmt.Sprintf("  ... +%d more", len(s.files)-maxFiles)))
	}

	return strings.Join(lines, "\n")
}
