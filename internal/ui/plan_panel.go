package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PlanStep represents a single step in a plan.
type PlanStep struct {
	Title       string
	Description string
	Status      string // "pending", "in_progress", "completed", "failed"
}

// PlanPanel displays the current plan in a sidebar-like view.
type PlanPanel struct {
	steps   []PlanStep
	visible bool
}

// NewPlanPanel creates a new plan panel.
func NewPlanPanel() *PlanPanel {
	return &PlanPanel{
		visible: false,
	}
}

// Toggle shows/hides the plan panel.
func (p *PlanPanel) Toggle() {
	p.visible = !p.visible
}

// SetVisible sets the visibility of the plan panel.
func (p *PlanPanel) SetVisible(visible bool) {
	p.visible = visible
}

// IsVisible returns whether the plan panel is visible.
func (p *PlanPanel) IsVisible() bool {
	return p.visible
}

// SetSteps replaces the plan steps.
func (p *PlanPanel) SetSteps(steps []PlanStep) {
	p.steps = steps
}

// AddStep adds a step to the plan.
func (p *PlanPanel) AddStep(step PlanStep) {
	p.steps = append(p.steps, step)
}

// UpdateStepStatus updates the status of a step by title.
func (p *PlanPanel) UpdateStepStatus(title string, status string) {
	for i, step := range p.steps {
		if step.Title == title {
			p.steps[i].Status = status
			return
		}
	}
}

// Clear removes all steps.
func (p *PlanPanel) Clear() {
	p.steps = nil
}

// Render produces the plan panel view.
func (p *PlanPanel) Render(width int, styles appStyles) string {
	if !p.visible || len(p.steps) == 0 {
		return ""
	}

	var b strings.Builder

	// Header
	header := "Plan"
	b.WriteString(styles.pill.Render(header))
	b.WriteString("\n\n")

	// Steps
	for i, step := range p.steps {
		icon := stepStatusIcon(step.Status)
		title := styles.muted.Render(fmt.Sprintf("%d.", i+1)) + " " + icon + " " + step.Title
		b.WriteString(title)
		b.WriteString("\n")
		if step.Description != "" {
			desc := styles.muted.Render("   " + step.Description)
			b.WriteString(desc)
			b.WriteString("\n")
		}
	}

	// Summary
	completed := 0
	for _, step := range p.steps {
		if step.Status == "completed" {
			completed++
		}
	}
	summary := styles.muted.Render(fmt.Sprintf("\n%d/%d completed", completed, len(p.steps)))
	b.WriteString(summary)

	return b.String()
}

func stepStatusIcon(status string) string {
	switch status {
	case "completed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	case "in_progress":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Render("●")
	case "failed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("○")
	}
}
