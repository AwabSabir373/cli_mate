package ui

import (
	"sync"
	"time"
)

// renderCacheEntry stores a cached rendered markdown result.
type renderCacheEntry struct {
	text      string
	rendered  string
	width     int
	timestamp time.Time
}

// renderCache provides TTL-based caching for markdown rendering.
type renderCache struct {
	mu       sync.RWMutex
	entries  map[string]*renderCacheEntry
	ttl      time.Duration
	maxSize  int
}

// newRenderCache creates a new render cache with the given TTL and max size.
func newRenderCache(ttl time.Duration, maxSize int) *renderCache {
	return &renderCache{
		entries: make(map[string]*renderCacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// get returns a cached rendered string if available and not expired.
func (rc *renderCache) get(key string, width int) (string, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, ok := rc.entries[key]
	if !ok {
		return "", false
	}
	if time.Since(entry.timestamp) > rc.ttl {
		return "", false
	}
	if entry.width != width {
		return "", false
	}
	return entry.rendered, true
}

// set stores a rendered string in the cache.
func (rc *renderCache) set(key string, rendered string, width int) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if len(rc.entries) >= rc.maxSize {
		// Evict oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range rc.entries {
			if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		delete(rc.entries, oldestKey)
	}

	rc.entries[key] = &renderCacheEntry{
		text:      rendered,
		rendered:  rendered,
		width:     width,
		timestamp: time.Now(),
	}
}

// clear removes all entries from the cache.
func (rc *renderCache) clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.entries = make(map[string]*renderCacheEntry)
}

// renderCacheKey generates a cache key for a log entry.
func renderCacheKey(kind string, text string) string {
	return kind + ":" + text
}
