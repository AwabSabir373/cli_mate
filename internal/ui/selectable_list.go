package ui

import (
	"fmt"
	"strings"
)

// selectableListItem represents an item in a selectable list.
type selectableListItem struct {
	Label       string
	Description string
}

// selectableList manages a selectable list with keyboard navigation.
type selectableList struct {
	items      []selectableListItem
	selected   int
	maxVisible int
	scrollOff  int
}

// newSelectableList creates a new selectable list.
func newSelectableList(items []selectableListItem) *selectableList {
	return &selectableList{
		items:      items,
		selected:   0,
		maxVisible: 10,
	}
}

// newSelectableListFromStrings creates a selectable list from string labels.
func newSelectableListFromStrings(labels []string) *selectableList {
	items := make([]selectableListItem, len(labels))
	for i, l := range labels {
		items[i] = selectableListItem{Label: l}
	}
	return newSelectableList(items)
}

// setItems replaces the items in the list.
func (sl *selectableList) setItems(items []selectableListItem) {
	sl.items = items
	sl.selected = 0
	sl.scrollOff = 0
}

// setItemsFromStrings replaces items from string labels.
func (sl *selectableList) setItemsFromStrings(labels []string) {
	items := make([]selectableListItem, len(labels))
	for i, l := range labels {
		items[i] = selectableListItem{Label: l}
	}
	sl.setItems(items)
}

// moveUp moves the selection up.
func (sl *selectableList) moveUp() {
	if sl.selected > 0 {
		sl.selected--
	}
	sl.adjustScroll()
}

// moveDown moves the selection down.
func (sl *selectableList) moveDown() {
	if sl.selected < len(sl.items)-1 {
		sl.selected++
	}
	sl.adjustScroll()
}

// selectedItem returns the currently selected item.
func (sl *selectableList) selectedItem() *selectableListItem {
	if sl.selected >= 0 && sl.selected < len(sl.items) {
		return &sl.items[sl.selected]
	}
	return nil
}

// selectedLabel returns the label of the currently selected item.
func (sl *selectableList) selectedLabel() string {
	item := sl.selectedItem()
	if item != nil {
		return item.Label
	}
	return ""
}

func (sl *selectableList) adjustScroll() {
	if sl.selected < sl.scrollOff {
		sl.scrollOff = sl.selected
	}
	if sl.selected >= sl.scrollOff+sl.maxVisible {
		sl.scrollOff = sl.selected - sl.maxVisible + 1
	}
}

// render renders the selectable list.
func (sl *selectableList) render(_ int, styles appStyles) string {
	if len(sl.items) == 0 {
		return styles.muted.Render("  No items available.")
	}

	start := sl.scrollOff
	end := start + sl.maxVisible
	if end > len(sl.items) {
		end = len(sl.items)
	}

	visible := sl.items[start:end]

	var b strings.Builder
	for i, item := range visible {
		idx := start + i
		prefix := "  "
		if idx == sl.selected {
			prefix = "▸ "
		}

		line := prefix + item.Label

		if item.Description != "" {
			line += strings.Repeat(" ", 3) + styles.muted.Render(item.Description)
		}

		if idx == sl.selected {
			b.WriteString(styles.selected.Render(line))
		} else {
			b.WriteString(styles.muted.Render(line))
		}
		b.WriteString("\n")
	}

	if len(sl.items) > sl.maxVisible {
		b.WriteString(styles.scrollHint.Render(fmt.Sprintf("  %d items, ↑/↓ to scroll", len(sl.items))))
	}

	return b.String()
}

// renderWithTitle renders the selectable list with a title header.
func (sl *selectableList) renderWithTitle(title string, width int, styles appStyles) string {
	var b strings.Builder
	b.WriteString(styles.pill.Render(title))
	b.WriteString("\n\n")
	b.WriteString(sl.render(width, styles))
	b.WriteString("\n")
	b.WriteString(styles.scrollHint.Render("  ↑/↓ navigate · Enter select · Esc cancel"))
	return b.String()
}
