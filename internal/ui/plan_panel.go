package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// PlanStep represents a single step in a plan.
type PlanStep struct {
	Title       string
	Description string
	Status      string // "pending", "in_progress", "completed", "failed"
	StartedAt   time.Time
	CompletedAt time.Time
}

// PlanPanel displays the current plan with progress bar and timestamps.
type PlanPanel struct {
	steps           []PlanStep
	visible         bool
	createdAt       time.Time
	completedAt     time.Time
	completedHidden bool // Auto-hide after completion
}

// NewPlanPanel creates a new plan panel.
func NewPlanPanel() *PlanPanel {
	return &PlanPanel{
		visible:   false,
		createdAt: time.Now(),
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
	if !p.visible {
		return false
	}
	// Auto-hide completed plans after 30 seconds
	if p.completedHidden {
		return false
	}
	if p.isComplete() && !p.completedAt.IsZero() {
		if time.Since(p.completedAt) > 30*time.Second {
			p.completedHidden = true
			return false
		}
	}
	return p.visible && len(p.steps) > 0
}

// SetSteps replaces the plan steps while preserving timestamps.
func (p *PlanPanel) SetSteps(steps []PlanStep) {
	// Preserve timestamps for matching steps
	for i, newStep := range steps {
		for _, oldStep := range p.steps {
			if oldStep.Title == newStep.Title {
				steps[i].StartedAt = oldStep.StartedAt
				steps[i].CompletedAt = oldStep.CompletedAt
				break
			}
		}
		if newStep.Status == "in_progress" && steps[i].StartedAt.IsZero() {
			steps[i].StartedAt = time.Now()
		}
		if newStep.Status == "completed" && steps[i].CompletedAt.IsZero() {
			steps[i].CompletedAt = time.Now()
		}
	}

	p.steps = steps
	p.completedHidden = false

	if p.isComplete() {
		p.completedAt = time.Now()
	}
}

// AddStep adds a step to the plan.
func (p *PlanPanel) AddStep(step PlanStep) {
	step.StartedAt = time.Now()
	p.steps = append(p.steps, step)
	p.completedHidden = false
}

// UpdateStepStatus updates the status of a step by title.
func (p *PlanPanel) UpdateStepStatus(title string, status string) {
	for i, step := range p.steps {
		if step.Title == title {
			p.steps[i].Status = status
			switch status {
			case "in_progress":
				if p.steps[i].StartedAt.IsZero() {
					p.steps[i].StartedAt = time.Now()
				}
			case "completed", "failed":
				if p.steps[i].CompletedAt.IsZero() {
					p.steps[i].CompletedAt = time.Now()
				}
			}
			return
		}
	}
}

// Clear removes all steps.
func (p *PlanPanel) Clear() {
	p.steps = nil
	p.createdAt = time.Now()
	p.completedAt = time.Time{}
	p.completedHidden = false
}

func (p *PlanPanel) isComplete() bool {
	if len(p.steps) == 0 {
		return false
	}
	for _, step := range p.steps {
		if step.Status != "completed" && step.Status != "failed" {
			return false
		}
	}
	return true
}

// Render produces the plan panel view.
func (p *PlanPanel) Render(width int, styles appStyles) string {
	if !p.IsVisible() {
		return ""
	}

	var b strings.Builder

	// Header with progress
	completed := 0
	running := 0
	for _, step := range p.steps {
		if step.Status == "completed" {
			completed++
		}
		if step.Status == "in_progress" {
			running++
		}
	}
	progress := fmt.Sprintf("%d/%d", completed, len(p.steps))
	header := fmt.Sprintf("Plan  %s", progress)
	b.WriteString(styles.pill.Render(header))
	b.WriteString("\n")

	// Progress bar
	if len(p.steps) > 0 {
		barWidth := 20
		fill := int(float64(completed) / float64(len(p.steps)) * float64(barWidth))
		if fill < 0 {
			fill = 0
		}
		if fill > barWidth {
			fill = barWidth
		}
		bar := strings.Repeat("█", fill) + strings.Repeat("░", barWidth-fill)
		fg := styles.success
		if !p.isComplete() {
			fg = styles.accent
		}
		b.WriteString(fg.Render(" " + bar))
		b.WriteString("\n\n")
	}

	// Steps
	for idx, step := range p.steps {
		icon := stepStatusIcon(step.Status)
		title := fmt.Sprintf("%s %s", icon, step.Title)
		if step.Status == "in_progress" && !step.StartedAt.IsZero() {
			elapsed := time.Since(step.StartedAt).Round(time.Second)
			title += styles.muted.Render(fmt.Sprintf(" (%s)", elapsed))
		}
		b.WriteString(title)
		b.WriteString("\n")
		if step.Description != "" && step.Status == "in_progress" {
			b.WriteString(styles.muted.Render("  " + step.Description))
			b.WriteString("\n")
		}
		_ = idx
	}

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
