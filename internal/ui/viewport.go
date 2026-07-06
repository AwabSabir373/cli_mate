package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// viewport handles smooth scrolling and auto-scroll behavior for the transcript.
type viewport struct {
	totalLines   int
	visibleLines int
	scrollPos    int
	pinBottom    bool // When true, auto-scrolls to follow new content
	lastPinnedAt int  // Line count when pin was last set
}

// newViewport creates a new viewport.
func newViewport() *viewport {
	return &viewport{
		pinBottom: true,
	}
}

// setTotalLines updates the total number of lines in the content.
func (vp *viewport) setTotalLines(n int) {
	wasPinned := vp.pinBottom

	// If we were pinned to bottom, stay at bottom
	if wasPinned {
		vp.totalLines = n
		vp.scrollToBottom()
		return
	}

	// Track if user manually scrolled up - keep position stable
	vp.totalLines = n
}

// setVisibleLines updates the number of visible lines.
func (vp *viewport) setVisibleLines(n int) {
	vp.visibleLines = n
	vp.clamp()
}

// scrollToBottom scrolls to the bottom of the content.
func (vp *viewport) scrollToBottom() {
	vp.scrollPos = 0
	vp.pinBottom = true
	vp.lastPinnedAt = vp.totalLines
}

// scrollUp scrolls up by one line.
func (vp *viewport) scrollUp() {
	if vp.scrollPos < vp.maxScroll() {
		vp.scrollPos++
		vp.pinBottom = false
	}
}

// scrollDown scrolls down by one line.
func (vp *viewport) scrollDown() {
	if vp.scrollPos > 0 {
		vp.scrollPos--
	}
	// If we reached the bottom, re-pin
	if vp.scrollPos == 0 {
		vp.pinBottom = true
		vp.lastPinnedAt = vp.totalLines
	}
}

// scrollPageUp scrolls up by a page.
func (vp *viewport) scrollPageUp() {
	vp.scrollPos += vp.visibleLines
	if vp.scrollPos > vp.maxScroll() {
		vp.scrollPos = vp.maxScroll()
	}
	vp.pinBottom = false
}

// scrollPageDown scrolls down by a page.
func (vp *viewport) scrollPageDown() {
	vp.scrollPos -= vp.visibleLines
	if vp.scrollPos < 0 {
		vp.scrollPos = 0
	}
	if vp.scrollPos == 0 {
		vp.pinBottom = true
		vp.lastPinnedAt = vp.totalLines
	}
}

// visibleRange returns the start and end indices of visible lines.
func (vp *viewport) visibleRange() (start, end int) {
	start = vp.totalLines - vp.visibleLines - vp.scrollPos
	if start < 0 {
		start = 0
	}
	end = start + vp.visibleLines
	if end > vp.totalLines {
		end = vp.totalLines
	}
	return start, end
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
func (vp *viewport) maxScroll() int {
	maxScroll := vp.totalLines - vp.visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
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
	if vp.totalLines <= vp.visibleLines {
		return ""
	}

	total := vp.totalLines - vp.visibleLines
	if total <= 0 {
		total = 1
	}
	pos := float64(vp.scrollPos) / float64(total)
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
