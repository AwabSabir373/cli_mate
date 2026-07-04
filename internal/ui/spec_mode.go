package ui

import (
	"fmt"
	"strings"
	"time"
)

// specConstraint represents a single constraint in the specification.
type specConstraint struct {
	field       string // "behavior", "performance", "security", "style", "testing", "custom"
	description string
	severity    string // "required", "recommended", "optional"
	met         bool   // whether the constraint has been satisfied
}

// specMode manages specification-driven development mode.
type specMode struct {
	active      bool
	visible     bool
	title       string
	description string
	constraints []specConstraint
	cursor      int
	scrollOff   int
	createdAt   time.Time
	updatedAt   time.Time
}

// newSpecMode creates a new spec mode manager.
func newSpecMode() *specMode {
	return &specMode{}
}

// start begins a new specification.
func (sm *specMode) start(title, description string) {
	sm.active = true
	sm.title = title
	sm.description = description
	sm.constraints = nil
	sm.cursor = 0
	sm.scrollOff = 0
	sm.createdAt = time.Now()
	sm.updatedAt = time.Now()
}

// stop ends the specification mode.
func (sm *specMode) stop() {
	sm.active = false
	sm.visible = false
}

// show makes the spec panel visible.
func (sm *specMode) show() {
	sm.visible = true
}

// hide hides the spec panel.
func (sm *specMode) hide() {
	sm.visible = false
}

// isActive returns true if spec mode is active.
func (sm *specMode) isActive() bool {
	return sm.active
}

// isVisible returns true if the spec panel is visible.
func (sm *specMode) isVisible() bool {
	return sm.visible && sm.active
}

// AddConstraint adds a constraint to the specification.
func (sm *specMode) AddConstraint(field, description, severity string) {
	sm.constraints = append(sm.constraints, specConstraint{
		field:       field,
		description: description,
		severity:    severity,
		met:         false,
	})
	sm.updatedAt = time.Now()
}

// MarkConstraintMet marks a constraint as satisfied.
func (sm *specMode) MarkConstraintMet(index int) {
	if index >= 0 && index < len(sm.constraints) {
		sm.constraints[index].met = true
		sm.updatedAt = time.Now()
	}
}

// completionPercent returns the percentage of constraints met.
func (sm *specMode) completionPercent() float64 {
	if len(sm.constraints) == 0 {
		return 0
	}
	met := 0
	for _, c := range sm.constraints {
		if c.met {
			met++
		}
	}
	return float64(met) / float64(len(sm.constraints)) * 100
}

// handleKey processes key events.
func (sm *specMode) handleKey(key string) string {
	if !sm.visible {
		return ""
	}

	switch key {
	case "up", "shift+tab":
		if sm.cursor > 0 {
			sm.cursor--
		}
		sm.adjustScroll()
	case "down", "tab":
		if sm.cursor < len(sm.constraints) {
			sm.cursor++
		}
		sm.adjustScroll()
	case "enter", " ":
		if sm.cursor >= 0 && sm.cursor < len(sm.constraints) {
			sm.MarkConstraintMet(sm.cursor)
		}
	case "esc":
		sm.hide()
	}
	return ""
}

func (sm *specMode) adjustScroll() {
	maxVisible := 10
	if sm.cursor < sm.scrollOff {
		sm.scrollOff = sm.cursor
	}
	if sm.cursor >= sm.scrollOff+maxVisible {
		sm.scrollOff = sm.cursor - maxVisible + 1
	}
}

// fieldIcon returns an icon for the constraint field.
func fieldIcon(field string) string {
	switch field {
	case "behavior":
		return "🎯"
	case "performance":
		return "⚡"
	case "security":
		return "🔒"
	case "style":
		return "🎨"
	case "testing":
		return "🧪"
	default:
		return "📋"
	}
}

// severityIndicator returns a severity indicator string.
func severityIndicator(severity string) string {
	switch severity {
	case "required":
		return "🔴 REQUIRED"
	case "recommended":
		return "🟡 RECOMMENDED"
	case "optional":
		return "🟢 OPTIONAL"
	default:
		return severity
	}
}

// renderSpecMode renders the specification mode overlay.
func renderSpecMode(sm *specMode, styles appStyles, _ int) string {
	if !sm.isVisible() {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(fmt.Sprintf(" Spec Mode: %s ", sm.title)))
	b.WriteString("\n\n")

	// Description
	if sm.description != "" {
		b.WriteString(styles.muted.Render(sm.description))
		b.WriteString("\n\n")
	}

	// Progress
	if len(sm.constraints) > 0 {
		pct := sm.completionPercent()
		barWidth := 30
		fill := int(pct / 100 * float64(barWidth))
		if fill < 0 {
			fill = 0
		}
		if fill > barWidth {
			fill = barWidth
		}
		bar := strings.Repeat("█", fill) + strings.Repeat("░", barWidth-fill)
		b.WriteString(styles.accent.Render(fmt.Sprintf("  Progress: %.0f%%", pct)))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s", bar))
		b.WriteString("\n\n")
	}

	// Constraints
	if len(sm.constraints) == 0 {
		b.WriteString(styles.muted.Render("  No constraints defined yet."))
		b.WriteString("\n\n")
	} else {
		b.WriteString(styles.sidebarTitle.Render("  Constraints:"))
		b.WriteString("\n")

		maxVisible := 10
		start := sm.scrollOff
		end := start + maxVisible
		if end > len(sm.constraints) {
			end = len(sm.constraints)
		}

		for i := start; i < end; i++ {
			c := sm.constraints[i]
			icon := fieldIcon(c.field)
			metIcon := "○"
			if c.met {
				metIcon = styles.success.Render("✓")
			}

			desc := c.description
			if len(desc) > 40 {
				desc = desc[:40] + "..."
			}

			line := fmt.Sprintf("  %s %s %s", metIcon, icon, desc)
			if i == sm.cursor {
				b.WriteString(styles.selected.Render(line))
				b.WriteString("\n")
			} else {
				b.WriteString(line)
				b.WriteString("\n")
			}

			// Show severity on the next line
			if i == sm.cursor {
				b.WriteString(styles.muted.Render(fmt.Sprintf("     %s", severityIndicator(c.severity))))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter toggle complete · Esc close"))
	b.WriteString("\n")
	return b.String()
}

// renderSpecSummary renders a compact spec summary for the sidebar.
func renderSpecSummary(sm *specMode, styles appStyles) string {
	if !sm.isActive() {
		return ""
	}

	met := 0
	for _, c := range sm.constraints {
		if c.met {
			met++
		}
	}

	summary := fmt.Sprintf("Spec: %d/%d constraints met", met, len(sm.constraints))
	return styles.accent.Render(summary)
}
