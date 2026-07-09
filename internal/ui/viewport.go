package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// viewport handles smooth scrolling and auto-scroll behavior for the transcript.
type viewport struct {
	totalLines      int  // total entries
	visibleLines    int  // preferred max entries to show (before height cap)
	totalEntries    int  // total entry count
	renderedEntries int  // entries actually rendered (after height cap)
	scrollPos       int  // entries scrolled up from bottom (0 = at bottom)
	pinBottom       bool // When true, auto-scrolls to follow new content
	lastPinnedAt    int  // Entry count when pin was last set
}

// newViewport creates a new viewport.
func newViewport() *viewport {
	return &viewport{
		pinBottom: true,
	}
}

// setTotalLines updates the total number of entries in the content.
func (vp *viewport) setTotalLines(n int) {
	vp.totalLines = n
	vp.totalEntries = n
	if vp.pinBottom {
		vp.scrollToBottom()
	}
	vp.clamp()
}

// setVisibleLines updates the number of visible lines.
func (vp *viewport) setVisibleLines(n int) {
	vp.visibleLines = n
	vp.clamp()
}

// setRenderedEntries updates the count of entries actually rendered.
func (vp *viewport) setRenderedEntries(n int) {
	vp.renderedEntries = n
	vp.clamp()
}

// scrollToBottom scrolls to the bottom of the content.
func (vp *viewport) scrollToBottom() {
	vp.scrollPos = 0
	vp.pinBottom = true
	vp.lastPinnedAt = vp.totalLines
}

// scrollUp scrolls up by one entry (toward older content).
func (vp *viewport) scrollUp() {
	if vp.scrollPos < vp.maxScroll() {
		vp.scrollPos++
		vp.pinBottom = false
	}
}

// scrollDown scrolls down by one entry (toward newer content).
func (vp *viewport) scrollDown() {
	if vp.scrollPos > 0 {
		vp.scrollPos--
	}
	if vp.scrollPos == 0 {
		vp.pinBottom = true
		vp.lastPinnedAt = vp.totalLines
	}
}

// scrollBy scrolls by delta entries. Positive delta moves toward older content.
func (vp *viewport) scrollBy(delta int) {
	if delta == 0 {
		return
	}
	if delta > 0 {
		for i := 0; i < delta; i++ {
			vp.scrollUp()
		}
		return
	}
	for i := 0; i < -delta; i++ {
		vp.scrollDown()
	}
}

// scrollToEntry scrolls so that the given entry index is at the top.
func (vp *viewport) scrollToEntry(entryIdx int) {
	visible := vp.effectiveVisible()
	newPos := vp.totalEntries - entryIdx - visible
	if newPos < 0 {
		newPos = 0
	}
	if newPos > vp.maxScroll() {
		newPos = vp.maxScroll()
	}
	vp.scrollPos = newPos
	vp.pinBottom = false
}

// scrollPageUp scrolls up by a page.
func (vp *viewport) scrollPageUp() {
	page := vp.effectiveVisible()
	if page < 1 {
		page = 1
	}
	vp.scrollBy(page)
}

// scrollPageDown scrolls down by a page.
func (vp *viewport) scrollPageDown() {
	page := vp.effectiveVisible()
	if page < 1 {
		page = 1
	}
	vp.scrollBy(-page)
}

// effectiveVisible returns how many entries are currently considered visible.
func (vp *viewport) effectiveVisible() int {
	if vp.renderedEntries > 0 {
		return vp.renderedEntries
	}
	if vp.visibleLines > 0 {
		return vp.visibleLines
	}
	return 1
}

// visibleRange returns the start and end indices of the candidate entry window.
// Final packing may show fewer entries when individual rows are multi-line.
func (vp *viewport) visibleRange() (start, end int) {
	visible := vp.effectiveVisible()
	start = vp.totalLines - visible - vp.scrollPos
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > vp.totalLines {
		end = vp.totalLines
	}
	return start, end
}

// packWindow returns the end-exclusive index of the newest entry to consider
// for bottom-up packing, after applying scrollPos.
func (vp *viewport) packWindowEnd() int {
	end := vp.totalLines - vp.scrollPos
	if end < 0 {
		end = 0
	}
	if end > vp.totalLines {
		end = vp.totalLines
	}
	return end
}

// isAtBottom returns true if the viewport is showing the latest content.
func (vp *viewport) isAtBottom() bool {
	return vp.scrollPos == 0
}

// hasOlder returns true if there are older entries above the visible window.
func (vp *viewport) hasOlder() (int, bool) {
	start, _ := vp.visibleRange()
	if start > 0 {
		return start, true
	}
	return 0, false
}

// hasNewer returns true if there are newer entries below the visible window.
func (vp *viewport) hasNewer() (int, bool) {
	_, end := vp.visibleRange()
	remaining := vp.totalLines - end
	return remaining, remaining > 0
}

// pinToBottom forces pin-to-bottom behavior.
func (vp *viewport) pinToBottom() {
	vp.scrollToBottom()
}

// onNewContent should be called when new content is added.
func (vp *viewport) onNewContent(newTotal int) {
	vp.setTotalLines(newTotal)
}

// clamp ensures scroll position is valid.
func (vp *viewport) clamp() {
	maxScroll := vp.maxScroll()
	if vp.scrollPos > maxScroll {
		vp.scrollPos = maxScroll
	}
	if vp.scrollPos < 0 {
		vp.scrollPos = 0
	}
}

// maxScroll returns the maximum scroll position.
// scrollPos is "how many newest entries to skip" for bottom-up packing, so the
// wheel can always walk the full log even when only one tall tool card fits.
func (vp *viewport) maxScroll() int {
	if vp.totalLines <= 1 {
		return 0
	}
	return vp.totalLines - 1
}

// scrollHint returns a string indicating scroll position.
func (vp *viewport) scrollHint(styles appStyles) string {
	if vp.isAtBottom() || vp.totalLines <= vp.visibleLines {
		return ""
	}

	var hints []string
	if _, ok := vp.hasOlder(); ok {
		hints = append(hints, styles.muted.Render("↑ older entries"))
	}
	if vp.pinBottom {
		hints = append(hints, styles.muted.Render("▼ auto-scroll"))
	}

	return strings.Join(hints, " ")
}

// renderScrollIndicator renders a scroll position indicator bar.
func (vp *viewport) renderScrollIndicator(_ int, _ appStyles) string {
	maxScroll := vp.maxScroll()
	if maxScroll <= 0 {
		return ""
	}

	pos := float64(vp.scrollPos) / float64(maxScroll)
	barHeight := 1
	barPos := int(pos * float64(barHeight))

	var b strings.Builder
	for i := 0; i < barHeight; i++ {
		if i == barPos {
			b.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("243")).
				Foreground(lipgloss.Color("243")).
				Render("━"))
		} else {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("236")).
				Render("━"))
		}
	}

	return b.String()
}
