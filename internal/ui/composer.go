package ui

import (
	"fmt"
	"strings"
)

// composerPastePreview represents a block of pasted text shown as a preview.
type composerPastePreview struct {
	start int    // start position in the composed text
	end   int    // end position in the composed text
	lines int    // number of lines in the pasted content
	label string // display label
}

// composerSelectionState tracks text selection in the composer.
type composerSelectionState struct {
	active bool
	anchor int // cursor position when selection started
	cursor int // current cursor position
}

// composerState manages the rich text input area.
type composerState struct {
	text            string
	cursor          int
	selection       composerSelectionState
	pastePreviews   []composerPastePreview
	multiline       bool // allow multi-line input
	showLineNumbers bool
	history         []string
	historyIndex    int
}

// newComposerState creates a new composer state.
func newComposerState() *composerState {
	return &composerState{
		multiline:       true,
		showLineNumbers: false,
	}
}

// setText sets the composer text and resets cursor.
func (cs *composerState) setText(text string) {
	cs.text = text
	cs.cursor = len(text)
	cs.selection = composerSelectionState{}
	cs.pastePreviews = nil
}

// insertText inserts text at the current cursor position.
func (cs *composerState) insertText(text string) {
	if cs.selection.active {
		cs.deleteSelection()
	}

	if cs.cursor >= len(cs.text) {
		cs.text += text
		cs.cursor = len(cs.text)
	} else {
		before := cs.text[:cs.cursor]
		after := cs.text[cs.cursor:]
		cs.text = before + text + after
		cs.cursor += len(text)
	}

	// Check for paste preview creation
	cs.checkPastePreview(text)
}

// insertNewline inserts a newline at the current cursor position.
func (cs *composerState) insertNewline() {
	if !cs.multiline {
		return
	}
	if cs.selection.active {
		cs.deleteSelection()
	}
	cs.insertText("\n")
}

// deleteBefore deletes the character before the cursor.
func (cs *composerState) deleteBefore() {
	if cs.selection.active {
		cs.deleteSelection()
		return
	}
	if cs.cursor == 0 {
		return
	}

	// Check if we're inside a paste preview
	for _, pp := range cs.pastePreviews {
		if cs.cursor > pp.start && cs.cursor <= pp.end {
			// Deleting within a paste preview - delete the entire preview
			cs.deletePastePreview(pp)
			return
		}
	}

	before := cs.text[:cs.cursor-1]
	after := cs.text[cs.cursor:]
	cs.text = before + after
	cs.cursor--
	cs.adjustPastePreviews(cs.cursor, -1)
}

// deleteAfter deletes the character after the cursor.
func (cs *composerState) deleteAfter() {
	if cs.selection.active {
		cs.deleteSelection()
		return
	}
	if cs.cursor >= len(cs.text) {
		return
	}

	// Check if we're at the start of a paste preview
	for _, pp := range cs.pastePreviews {
		if cs.cursor >= pp.start && cs.cursor < pp.end {
			cs.deletePastePreview(pp)
			return
		}
	}

	before := cs.text[:cs.cursor]
	after := cs.text[cs.cursor+1:]
	cs.text = before + after
}

// deleteWordBefore deletes the word before the cursor.
func (cs *composerState) deleteWordBefore() {
	if cs.selection.active {
		cs.deleteSelection()
		return
	}
	if cs.cursor == 0 {
		return
	}

	pos := cs.cursor - 1
	// Skip trailing spaces
	for pos > 0 && cs.text[pos] == ' ' {
		pos--
	}
	// Skip word characters
	for pos > 0 && cs.text[pos-1] != ' ' && cs.text[pos-1] != '\n' {
		pos--
	}
	// Check if cursor is at start of a paste preview
	for _, pp := range cs.pastePreviews {
		if pp.start >= pos && pp.start < cs.cursor {
			cs.deletePastePreview(pp)
			return
		}
	}

	before := cs.text[:pos]
	after := cs.text[cs.cursor:]
	cs.text = before + after
	removed := cs.cursor - pos
	cs.cursor = pos
	cs.adjustPastePreviews(cs.cursor, -removed)
}

// deleteToLineStart deletes from cursor to start of the current line.
func (cs *composerState) deleteToLineStart() {
	if cs.cursor == 0 {
		return
	}
	// Find the start of the current line
	pos := cs.cursor - 1
	for pos > 0 && cs.text[pos-1] != '\n' {
		pos--
	}
	before := cs.text[:pos]
	after := cs.text[cs.cursor:]
	cs.text = before + after
	cs.cursor = pos
	cs.adjustPastePreviews(cs.cursor, -(cs.cursor - pos))
}

// deleteToLineEnd deletes from cursor to end of the current line.
func (cs *composerState) deleteToLineEnd() {
	if cs.cursor >= len(cs.text) {
		return
	}
	// Find the end of the current line
	pos := cs.cursor
	for pos < len(cs.text) && cs.text[pos] != '\n' {
		pos++
	}
	before := cs.text[:cs.cursor]
	after := cs.text[pos:]
	cs.text = before + after
}

// moveCursorLeft moves cursor one position left.
func (cs *composerState) moveCursorLeft() {
	if cs.cursor > 0 {
		cs.cursor--
	}
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorRight moves cursor one position right.
func (cs *composerState) moveCursorRight() {
	if cs.cursor < len(cs.text) {
		cs.cursor++
	}
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorUp moves cursor up one line.
func (cs *composerState) moveCursorUp() {
	lineStart := cs.lineStartFor(cs.cursor)
	if lineStart == 0 {
		cs.cursor = 0
		return
	}
	prevLineStart := cs.lineStartFor(lineStart - 1)
	col := cs.cursor - lineStart
	target := prevLineStart + col
	if target > lineStart-1 {
		target = lineStart - 1
	}
	cs.cursor = target
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorDown moves cursor down one line.
func (cs *composerState) moveCursorDown() {
	lineEnd := cs.lineEndFor(cs.cursor)
	if lineEnd >= len(cs.text) {
		cs.cursor = len(cs.text)
		return
	}
	nextLineStart := lineEnd + 1
	lineStart := cs.lineStartFor(cs.cursor)
	col := cs.cursor - lineStart
	target := nextLineStart + col
	nextLineEnd := cs.lineEndFor(nextLineStart)
	if target > nextLineEnd {
		target = nextLineEnd
	}
	cs.cursor = target
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorToLineStart moves cursor to the start of the current line.
func (cs *composerState) moveCursorToLineStart() {
	cs.cursor = cs.lineStartFor(cs.cursor)
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorToLineEnd moves cursor to the end of the current line.
func (cs *composerState) moveCursorToLineEnd() {
	cs.cursor = cs.lineEndFor(cs.cursor)
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorWordLeft moves cursor to the beginning of the previous word.
func (cs *composerState) moveCursorWordLeft() {
	if cs.cursor <= 0 {
		return
	}
	pos := cs.cursor - 1
	// Skip spaces
	for pos > 0 && cs.text[pos] == ' ' {
		pos--
	}
	// Skip word
	for pos > 0 && cs.text[pos-1] != ' ' && cs.text[pos-1] != '\n' {
		pos--
	}
	cs.cursor = pos
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// moveCursorWordRight moves cursor to the beginning of the next word.
func (cs *composerState) moveCursorWordRight() {
	if cs.cursor >= len(cs.text) {
		return
	}
	pos := cs.cursor
	// Skip current word
	for pos < len(cs.text) && cs.text[pos] != ' ' && cs.text[pos] != '\n' {
		pos++
	}
	// Skip spaces
	for pos < len(cs.text) && cs.text[pos] == ' ' {
		pos++
	}
	cs.cursor = pos
	if cs.selection.active {
		cs.selection.cursor = cs.cursor
	}
}

// startSelection begins text selection at the current cursor position.
func (cs *composerState) startSelection() {
	cs.selection.active = true
	cs.selection.anchor = cs.cursor
	cs.selection.cursor = cs.cursor
}

// endSelection ends text selection.
func (cs *composerState) endSelection() {
	cs.selection.active = false
}

// selectedText returns the currently selected text.
func (cs *composerState) selectedText() string {
	if !cs.selection.active {
		return ""
	}
	start, end := cs.selectionRange()
	if start >= end {
		return ""
	}
	return cs.text[start:end]
}

// deleteSelection deletes the currently selected text.
func (cs *composerState) deleteSelection() {
	if !cs.selection.active {
		return
	}
	start, end := cs.selectionRange()
	if start >= end {
		cs.selection.active = false
		return
	}
	before := cs.text[:start]
	after := cs.text[end:]
	cs.text = before + after
	cs.cursor = start
	cs.selection.active = false
	cs.adjustPastePreviews(start, -(end - start))
}

// selectionRange returns the start and end of the selection.
func (cs *composerState) selectionRange() (int, int) {
	if cs.selection.anchor < cs.selection.cursor {
		return cs.selection.anchor, cs.selection.cursor
	}
	return cs.selection.cursor, cs.selection.anchor
}

// lineStartFor returns the start position of the line containing pos.
func (cs *composerState) lineStartFor(pos int) int {
	if pos <= 0 {
		return 0
	}
	for i := pos - 1; i >= 0; i-- {
		if cs.text[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// lineEndFor returns the end position of the line containing pos.
func (cs *composerState) lineEndFor(pos int) int {
	if pos >= len(cs.text) {
		return len(cs.text)
	}
	for i := pos; i < len(cs.text); i++ {
		if cs.text[i] == '\n' {
			return i
		}
	}
	return len(cs.text)
}

// checkPastePreview checks if a paste preview should be created.
func (cs *composerState) checkPastePreview(text string) {
	// Create a paste preview if the inserted text has multiple lines
	lines := strings.Count(text, "\n")
	if lines > 2 {
		cs.pastePreviews = append(cs.pastePreviews, composerPastePreview{
			start: cs.cursor - len(text),
			end:   cs.cursor,
			lines: lines + 1,
			label: fmt.Sprintf("[paste · %d lines]", lines+1),
		})
	}
}

// deletePastePreview removes a paste preview from the text.
func (cs *composerState) deletePastePreview(pp composerPastePreview) {
	if pp.start >= pp.end {
		return
	}
	before := cs.text[:pp.start]
	after := cs.text[pp.end:]
	cs.text = before + after
	cs.cursor = pp.start
	cs.removePastePreview(pp)
}

// removePastePreview removes a paste preview from the list.
func (cs *composerState) removePastePreview(pp composerPastePreview) {
	for i, p := range cs.pastePreviews {
		if p.start == pp.start && p.end == pp.end {
			cs.pastePreviews = append(cs.pastePreviews[:i], cs.pastePreviews[i+1:]...)
			return
		}
	}
}

// adjustPastePreviews adjusts paste preview positions after an edit.
func (cs *composerState) adjustPastePreviews(afterPos int, delta int) {
	for i := range cs.pastePreviews {
		if cs.pastePreviews[i].start >= afterPos {
			cs.pastePreviews[i].start += delta
			cs.pastePreviews[i].end += delta
		}
	}

	// Remove paste previews that have been destroyed
	var valid []composerPastePreview
	for _, pp := range cs.pastePreviews {
		if pp.start >= 0 && pp.end <= len(cs.text) && pp.start < pp.end {
			valid = append(valid, pp)
		}
	}
	cs.pastePreviews = valid
}

// addToHistory adds the current text to history.
func (cs *composerState) addToHistory(text string) {
	if text == "" {
		return
	}
	cs.history = append(cs.history, text)
	if len(cs.history) > 50 {
		cs.history = cs.history[1:]
	}
	cs.historyIndex = len(cs.history)
}

// navigateHistory loads a previous/next entry from history.
func (cs *composerState) navigateHistory(delta int) string {
	if len(cs.history) == 0 {
		return ""
	}
	newIndex := cs.historyIndex + delta
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex > len(cs.history) {
		newIndex = len(cs.history)
	}
	cs.historyIndex = newIndex

	if cs.historyIndex >= len(cs.history) {
		cs.text = ""
	} else {
		cs.text = cs.history[cs.historyIndex]
	}
	cs.cursor = len(cs.text)
	return cs.text
}

// clear resets the composer.
func (cs *composerState) clear() {
	cs.text = ""
	cs.cursor = 0
	cs.selection = composerSelectionState{}
	cs.pastePreviews = nil
}

// renderComposer renders the composer input area.
func renderComposer(cs *composerState, styles appStyles, width int, promptStr string) string {
	var b strings.Builder

	if cs.text == "" && !cs.selection.active {
		b.WriteString(styles.prompt.Render(promptStr))
		b.WriteString(styles.cursor.Render(" "))
		b.WriteString(styles.muted.Render("Describe changes, mention @files, or type /"))
		return b.String()
	}

	lines := strings.Split(cs.text, "\n")
	cursorLine := 0
	cursorCol := cs.cursor

	// Find which line the cursor is on
	pos := 0
	for i, line := range lines {
		if i > 0 {
			pos++ // account for the newline
		}
		nextPos := pos + len(line)
		if cs.cursor <= nextPos {
			cursorLine = i
			cursorCol = cs.cursor - pos
			break
		}
		pos = nextPos
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
	}

	// Render paste preview indicators
	if len(cs.pastePreviews) > 0 {
		b.WriteString(styles.softPanel.Width(width - 6).Render(
			styles.muted.Render(fmt.Sprintf("  📋 %d paste(s) — Esc to collapse", len(cs.pastePreviews))),
		))
		b.WriteString("\n")
	}

	// Check if any line is part of a paste preview
	for i, line := range lines {
		prefix := styles.prompt.Render(promptStr)
		if i > 0 {
			prefix = styles.prompt.Render(" · ")
		}

		// Check if this line is inside a paste preview
		lineStart := 0
		for j := 0; j < i; j++ {
			lineStart += len(lines[j]) + 1
		}
		lineEnd := lineStart + len(line)
		isPasted := false
		for _, pp := range cs.pastePreviews {
			if lineStart >= pp.start && lineEnd <= pp.end {
				isPasted = true
				break
			}
		}

		if isPasted && i == cursorLine {
			// Show paste preview label at the cursor line
			for _, pp := range cs.pastePreviews {
				if lineStart >= pp.start && lineEnd <= pp.end {
					b.WriteString(prefix)
					b.WriteString(styles.muted.Render(pp.label))
					if i == cursorLine {
						b.WriteString(styles.cursor.Render(" "))
					}
					b.WriteString("\n")
					break
				}
			}
		} else if i == cursorLine {
			// Render cursor on this line
			b.WriteString(prefix)
			if cursorCol <= len(line) {
				left := line[:cursorCol]
				right := line[cursorCol:]
				b.WriteString(styles.input.Render(left))
				b.WriteString(styles.cursor.Render("█"))
				b.WriteString(styles.input.Render(right))
			} else {
				b.WriteString(styles.input.Render(line))
				b.WriteString(styles.cursor.Render(" "))
			}
			b.WriteString("\n")
		} else {
			b.WriteString(prefix)
			b.WriteString(styles.input.Render(line))
			b.WriteString("\n")
		}
	}

	return b.String()
}
