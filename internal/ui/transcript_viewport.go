package ui

import "strings"

// transcriptViewport tracks scroll state for the transcript body.
type transcriptViewport struct {
	totalLines int
	height     int
	offset     int
}

type transcriptViewportWindow struct {
	start     int
	end       int
	height    int
	maxOffset int
	offset    int
}

func newTranscriptViewport(totalLines int, height int, offset int) transcriptViewport {
	totalLines = max(0, totalLines)
	height = max(1, height)
	maxOffset := max(0, totalLines-height)
	return transcriptViewport{
		totalLines: totalLines,
		height:     height,
		offset:     clamp(offset, 0, maxOffset),
	}
}

func (v transcriptViewport) maxOffset() int {
	return max(0, v.totalLines-v.height)
}

func (v transcriptViewport) scroll(delta int) transcriptViewport {
	v.offset = clamp(v.offset+delta, 0, v.maxOffset())
	return v
}

func (v transcriptViewport) window() transcriptViewportWindow {
	maxOffset := v.maxOffset()
	offset := clamp(v.offset, 0, maxOffset)
	start := max(0, v.totalLines-v.height-offset)
	end := min(v.totalLines, start+v.height)
	return transcriptViewportWindow{
		start:     start,
		end:       end,
		height:    v.height,
		maxOffset: maxOffset,
		offset:    offset,
	}
}

func viewLines(value string) []string {
	if value == "" {
		return nil
	}
	s := value
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
