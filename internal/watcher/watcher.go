// Package watcher provides file system monitoring for the workspace.
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Change represents a detected file change.
type Change struct {
	Path      string
	Operation string // "created", "modified", "deleted"
	Timestamp time.Time
}

// Watcher monitors file system changes in a directory.
type Watcher struct {
	root      string
	interval  time.Duration
	callback  func(Change)
	running   bool
	stopCh    chan struct{}
	mu        sync.Mutex
	snapshots map[string]os.FileInfo
}

// New creates a new file watcher.
func New(root string, interval time.Duration, callback func(Change)) *Watcher {
	return &Watcher{
		root:      root,
		interval:  interval,
		callback:  callback,
		stopCh:    make(chan struct{}),
		snapshots: make(map[string]os.FileInfo),
	}
}

// Start begins watching for file changes.
func (w *Watcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return nil
	}

	// Take initial snapshot
	if err := w.takeSnapshot(); err != nil {
		return err
	}

	w.running = true
	go w.watchLoop()
	return nil
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.running = false
	close(w.stopCh)
}

// IsRunning returns whether the watcher is active.
func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Watcher) watchLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.checkForChanges()
		}
	}
}

func (w *Watcher) checkForChanges() {
	newSnapshot := make(map[string]os.FileInfo)

	filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories and common non-project dirs
		name := info.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if name == "node_modules" || name == "vendor" || name == ".git" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(w.root, path)
		newSnapshot[rel] = info
		return nil
	})

	w.mu.Lock()
	oldSnapshot := w.snapshots
	w.snapshots = newSnapshot
	w.mu.Unlock()

	// Detect changes
	for path, newInfo := range newSnapshot {
		oldInfo, existed := oldSnapshot[path]
		if !existed {
			w.emitChange(Change{Path: path, Operation: "created", Timestamp: time.Now()})
		} else if newInfo.ModTime().After(oldInfo.ModTime()) {
			w.emitChange(Change{Path: path, Operation: "modified", Timestamp: time.Now()})
		}
	}

	for path := range oldSnapshot {
		if _, exists := newSnapshot[path]; !exists {
			w.emitChange(Change{Path: path, Operation: "deleted", Timestamp: time.Now()})
		}
	}
}

func (w *Watcher) takeSnapshot() error {
	w.snapshots = make(map[string]os.FileInfo)

	return filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		name := info.Name()
		if strings.HasPrefix(name, ".") && name != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if name == "node_modules" || name == "vendor" || name == ".git" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(w.root, path)
		w.snapshots[rel] = info
		return nil
	})
}

func (w *Watcher) emitChange(change Change) {
	if w.callback != nil {
		w.callback(change)
	}
}
