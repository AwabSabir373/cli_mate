package ui

import (
	"time"
)

const (
	defaultFlushInterval = 50 * time.Millisecond
	defaultFlushLines    = 5
)

// flushBatcher batches render updates to avoid excessive redraws.
type flushBatcher struct {
	interval  time.Duration
	maxLines  int
	pending   int
	lastFlush time.Time
	timer     *time.Timer
}

// newFlushBatcher creates a new flush batcher with default settings.
func newFlushBatcher() *flushBatcher {
	return &flushBatcher{
		interval:  defaultFlushInterval,
		maxLines:  defaultFlushLines,
		lastFlush: time.Now(),
	}
}

// shouldFlush returns true if the batch should be flushed.
func (fb *flushBatcher) shouldFlush(force bool) bool {
	if force {
		return true
	}
	if fb.pending >= fb.maxLines {
		return true
	}
	if time.Since(fb.lastFlush) >= fb.interval {
		return true
	}
	return false
}

// recordLine records a line being added to the batch.
func (fb *flushBatcher) recordLine() {
	fb.pending++
}

// flush resets the batch counter and records the flush time.
func (fb *flushBatcher) flush() {
	fb.pending = 0
	fb.lastFlush = time.Now()
}

// pendingCount returns the number of pending lines.
func (fb *flushBatcher) pendingCount() int {
	return fb.pending
}

// setInterval sets the flush interval.
func (fb *flushBatcher) setInterval(d time.Duration) {
	fb.interval = d
}
