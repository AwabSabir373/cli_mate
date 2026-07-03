package oauth

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const lockRetryDelay = 100 * time.Millisecond
const lockMaxRetries = 50 // 5 seconds total
const lockMaxStaleAge = 30 * time.Second // locks older than this are considered stale

// acquireFileLock acquires a cross-process lock via a lock file.
// Returns an unlock function. Detects and cleans up stale locks.
func acquireFileLock(lockPath string, now func() time.Time) (func(), error) {
	for i := 0; i < lockMaxRetries; i++ {
		data, err := os.ReadFile(lockPath)
		if err == nil {
			// Lock file exists — check if it's stale
			if isStaleLock(lockPath, string(data), now) {
				_ = os.Remove(lockPath)
			} else {
				// Valid lock held by another process, wait and retry
				time.Sleep(lockRetryDelay)
				continue
			}
		}

		// Try to create the lock file
		f, createErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if createErr == nil {
			_, _ = f.WriteString(fmt.Sprintf("%d\n%s\n", os.Getpid(), now().Format(time.RFC3339)))
			f.Close()
			unlock := func() {
				_ = os.Remove(lockPath)
			}
			return unlock, nil
		}

		time.Sleep(lockRetryDelay)
	}
	return nil, fmt.Errorf("oauth: could not acquire lock at %s (all retries exhausted)", lockPath)
}

// isStaleLock checks if a lock file is stale by verifying the PID or age.
func isStaleLock(lockPath, data string, now func() time.Time) bool {
	lines := strings.SplitN(strings.TrimSpace(data), "\n", 2)
	if len(lines) == 0 || lines[0] == "" {
		return true // empty lock file is stale
	}

	// Check if the lock is older than the max stale age (cross-platform heuristic)
	if len(lines) > 1 && lines[1] != "" {
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[1]))
		if err == nil && now().Sub(ts) > lockMaxStaleAge {
			return true
		}
	}

	// Try PID check (best-effort — works on Unix, limited on Windows)
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return true // invalid PID, treat as stale
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return true // process doesn't exist
	}

	// Age-based heuristic: if the lock is within the stale window,
	// assume it's valid and held by a live process. This is the most
	// portable approach — `os.FindProcess` + signal checks have
	// platform-specific behavior that's hard to get right everywhere.
	// The 30-second stale window is short enough to prevent long blocks.
	_ = proc
	return false
}
