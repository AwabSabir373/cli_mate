package ui

import (
	"fmt"
	"strings"
)

// pickerKind defines what kind of items the picker displays.
type pickerKind int

const (
	pickerFiles    pickerKind = iota
	pickerSessions
	pickerModels
	pickerThemes
	pickerProviders
	pickerCommands
)

// pickerItem represents a single selectable item in the picker.
type pickerItem struct {
	Label       string
	Value       string
	Description string
	Group       string // optional group heading
	Icon        string // emoji or unicode icon
	Meta        string // secondary info displayed inline
}

// genericPicker is a reusable, keyboard-navigable picker dialog.
type genericPicker struct {
	visible     bool
	kind        pickerKind
	title       string
	items       []pickerItem
	allItems    []pickerItem // unfiltered items
	cursor      int
	scrollOff   int
	query       string
	searchMode  bool // when true, show search input instead of item count
	maxVisible  int
	allowCreate bool // allow creating a custom item (e.g., custom model)
	onSelect    func(string) // callback when item is selected
}

// newGenericPicker creates a new generic picker.
func newGenericPicker() *genericPicker {
	return &genericPicker{
		maxVisible: 12,
	}
}

// show opens the picker with the given items.
func (gp *genericPicker) show(kind pickerKind, title string, items []pickerItem) {
	gp.visible = true
	gp.kind = kind
	gp.title = title
	gp.allItems = items
	gp.items = make([]pickerItem, len(items))
	copy(gp.items, items)
	gp.cursor = 0
	gp.scrollOff = 0
	gp.query = ""
	gp.searchMode = false
}

// hide closes the picker.
func (gp *genericPicker) hide() {
	gp.visible = false
	gp.items = nil
	gp.allItems = nil
	gp.cursor = 0
	gp.query = ""
}

// isVisible returns true if the picker is active.
func (gp *genericPicker) isVisible() bool {
	return gp.visible
}

// setItems replaces the current item list.
func (gp *genericPicker) setItems(items []pickerItem) {
	gp.items = items
	if gp.cursor >= len(gp.items) {
		gp.cursor = len(gp.items) - 1
	}
}

// handleKey processes a keypress and returns (selectedValue string, finished bool).
func (gp *genericPicker) handleKey(key string) (string, bool) {
	if !gp.visible {
		return "", false
	}

	if gp.searchMode {
		switch key {
		case "enter":
			gp.searchMode = false
			return "", false
		case "esc":
			gp.searchMode = false
			return "", false
		case "backspace":
			if len(gp.query) > 0 {
				gp.query = gp.query[:len(gp.query)-1]
				gp.applyFilter()
			}
			return "", false
		default:
			if len(key) == 1 {
				gp.query += key
				gp.applyFilter()
			}
			return "", false
		}
	}

	switch key {
	case "up", "shift+tab":
		if gp.cursor > 0 {
			gp.cursor--
		} else if len(gp.items) > 0 {
			gp.cursor = len(gp.items) - 1
		}
		gp.adjustScroll()
	case "down", "tab":
		if gp.cursor < len(gp.items)-1 {
			gp.cursor++
		} else {
			gp.cursor = 0
		}
		gp.adjustScroll()
	case "pgup":
		gp.cursor -= gp.maxVisible
		if gp.cursor < 0 {
			gp.cursor = 0
		}
		gp.adjustScroll()
	case "pgdown":
		gp.cursor += gp.maxVisible
		if gp.cursor >= len(gp.items) {
			gp.cursor = len(gp.items) - 1
		}
		gp.adjustScroll()
	case "home":
		gp.cursor = 0
		gp.scrollOff = 0
	case "end":
		gp.cursor = len(gp.items) - 1
		gp.adjustScroll()
	case "enter", " ":
		if gp.cursor >= 0 && gp.cursor < len(gp.items) {
			selected := gp.items[gp.cursor].Value
			if gp.onSelect != nil {
				gp.onSelect(selected)
			}
			gp.visible = false
			return selected, true
		}
	case "esc":
		gp.visible = false
		return "", true
	case "/":
		// Enter search mode
		if len(gp.items) > 10 {
			gp.searchMode = true
			gp.query = ""
		}
	case "delete", "backspace":
		// Allow deletion of selected item
		if gp.cursor >= 0 && gp.cursor < len(gp.items) {
			gp.items = append(gp.items[:gp.cursor], gp.items[gp.cursor+1:]...)
			if gp.cursor >= len(gp.items) {
				gp.cursor = len(gp.items) - 1
			}
		}
	}

	return "", false
}

func (gp *genericPicker) adjustScroll() {
	if gp.cursor < gp.scrollOff {
		gp.scrollOff = gp.cursor
	}
	if gp.cursor >= gp.scrollOff+gp.maxVisible {
		gp.scrollOff = gp.cursor - gp.maxVisible + 1
	}
}

// applyFilter filters items by the current query string.
func (gp *genericPicker) applyFilter() {
	if gp.query == "" {
		gp.items = make([]pickerItem, len(gp.allItems))
		copy(gp.items, gp.allItems)
		gp.cursor = 0
		gp.scrollOff = 0
		return
	}

	q := strings.ToLower(gp.query)
	var filtered []pickerItem
	for _, item := range gp.allItems {
		if strings.Contains(strings.ToLower(item.Label), q) ||
			strings.Contains(strings.ToLower(item.Description), q) ||
			strings.Contains(strings.ToLower(item.Group), q) {
			filtered = append(filtered, item)
		}
	}

	gp.items = filtered
	if gp.cursor >= len(gp.items) {
		gp.cursor = 0
	}
}

// buildPickerItems constructs picker items from strings.
func buildPickerItems(labels []string, kind pickerKind) []pickerItem {
	items := make([]pickerItem, len(labels))
	for i, label := range labels {
		icon := pickerIconFor(kind)
		items[i] = pickerItem{
			Label: label,
			Value: label,
			Icon:  icon,
			Group: pickerGroupFor(kind),
		}
	}
	return items
}

func pickerIconFor(kind pickerKind) string {
	switch kind {
	case pickerFiles:
		return "📄"
	case pickerSessions:
		return "💬"
	case pickerModels:
		return "🧠"
	case pickerThemes:
		return "🎨"
	case pickerProviders:
		return "🔌"
	case pickerCommands:
		return "⚡"
	default:
		return "•"
	}
}

func pickerGroupFor(kind pickerKind) string {
	switch kind {
	case pickerFiles:
		return "Files"
	case pickerSessions:
		return "Sessions"
	case pickerModels:
		return "Models"
	case pickerThemes:
		return "Themes"
	case pickerProviders:
		return "Providers"
	case pickerCommands:
		return "Commands"
	default:
		return "Items"
	}
}

// renderPicker renders the generic picker overlay.
func renderPicker(gp *genericPicker, styles appStyles, width int) string {
	if !gp.visible {
		return ""
	}

	var b strings.Builder

	// Title bar
	b.WriteString(styles.pill.Render(fmt.Sprintf(" %s ", gp.title)))
	b.WriteString("\n\n")

	// Search bar
	if gp.searchMode {
		b.WriteString(styles.prompt.Render(" 🔍 "))
		b.WriteString(styles.input.Render(gp.query))
		b.WriteString(styles.cursor.Render("█"))
		b.WriteString("\n\n")
	} else if len(gp.items) > 10 {
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %d items · Press / to search", len(gp.items))))
		b.WriteString("\n\n")
	}

	if len(gp.items) == 0 {
		if gp.query != "" {
			b.WriteString(styles.muted.Render(fmt.Sprintf("  No matches for \"%s\"", gp.query)))
		} else {
			b.WriteString(styles.muted.Render("  No items available."))
		}
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	// Group items
	type group struct {
		name  string
		items []pickerItem
		start int // global index
	}
	var groups []group
	currentGroup := ""
	groupStart := 0

	for i, item := range gp.items {
		if item.Group != currentGroup {
			if currentGroup != "" {
				groups = append(groups, group{name: currentGroup, items: gp.items[groupStart:i], start: groupStart})
			}
			currentGroup = item.Group
			groupStart = i
		}
	}
	if currentGroup != "" {
		groups = append(groups, group{name: currentGroup, items: gp.items[groupStart:], start: groupStart})
	}

	// Render visible items
	start := gp.scrollOff
	end := start + gp.maxVisible
	if end > len(gp.items) {
		end = len(gp.items)
	}

	currentGroup = ""
	for i := start; i < end; i++ {
		item := gp.items[i]

		// Group header
		if item.Group != currentGroup {
			currentGroup = item.Group
			if currentGroup != "" {
				b.WriteString(styles.sidebarTitle.Render(fmt.Sprintf("  %s:", currentGroup)))
				b.WriteString("\n")
			}
		}

		icon := item.Icon
		if icon == "" {
			icon = " "
		}

		label := item.Label
		if len(label) > 40 {
			label = label[:40] + "..."
		}

		meta := ""
		if item.Meta != "" {
			meta = " " + styles.muted.Render(item.Meta)
		}

		line := fmt.Sprintf("  %s %s%s", icon, label, meta)

		if i == gp.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", line)))
			b.WriteString("\n")
			if item.Description != "" {
				b.WriteString(styles.muted.Render(fmt.Sprintf("    %s", truncateString(item.Description, width-30))))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(gp.items) > gp.maxVisible {
		pct := float64(gp.scrollOff) / float64(len(gp.items)-gp.maxVisible) * 100
		barWidth := 20
		fill := int(pct / 100 * float64(barWidth))
		if fill < 0 {
			fill = 0
		}
		if fill > barWidth {
			fill = barWidth
		}
		bar := strings.Repeat("█", fill) + strings.Repeat("░", barWidth-fill)
		b.WriteString("\n")
		b.WriteString(styles.muted.Render(fmt.Sprintf("  %s", bar)))
		b.WriteString("\n")
	}

	// Footer with keybindings
	b.WriteString("\n")
	b.WriteString(styles.muted.Render("  ↑/↓ navigate · PgUp/PgDn page · / search · Enter select · Esc close"))
	b.WriteString("\n")

	return styles.panel.Width(width - 4).Render(b.String())
}
