package oauth

import (
	"context"
	"sync"
	"time"
)

const maxSchedulerLoadErrors = 5

// RefreshScheduler proactively refreshes OAuth tokens shortly before expiry.
type RefreshScheduler struct {
	manager *Manager
	key     string
	cfg     Config
	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	running bool
}

// NewRefreshScheduler creates a new refresh scheduler for a token key.
func NewRefreshScheduler(manager *Manager, key string, cfg Config) *RefreshScheduler {
	return &RefreshScheduler{
		manager: manager,
		key:     key,
		cfg:     cfg,
	}
}

// Start begins the background token refresh loop.
func (s *RefreshScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.loop(ctx)
}

// Stop stops the background refresh loop.
func (s *RefreshScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stop)
	s.running = false
}

func (s *RefreshScheduler) loop(ctx context.Context) {
	defer close(s.done)
	loadErrors := 0

	for {
		token, err := s.manager.loadToken(s.key)
		if err != nil {
			loadErrors++
			if loadErrors >= maxSchedulerLoadErrors {
				return
			}
			select {
			case <-s.stop:
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		loadErrors = 0

		delay := delayUntilRefresh(token, s.manager.buffer)
		if delay <= 0 {
			// Token already needs refresh
			_, _ = s.manager.GetFresh(ctx, s.key)
			select {
			case <-s.stop:
				return
			case <-time.After(time.Minute):
			}
			continue
		}

		select {
		case <-s.stop:
			return
		case <-time.After(delay):
			_, _ = s.manager.GetFresh(ctx, s.key)
		}
	}
}

// delayUntilRefresh calculates how long to wait before refreshing.
// Adds deterministic jitter to prevent thundering herd.
func delayUntilRefresh(token Token, buffer time.Duration) time.Duration {
	if token.ExpiresAt == 0 {
		return time.Hour // no expiry, don't schedule
	}
	now := time.Now().Unix()
	remaining := token.ExpiresAt - now
	if remaining <= 0 {
		return 0 // already expired
	}
	bufSecs := int64(buffer.Seconds())
	if remaining <= bufSecs {
		return 0 // within buffer, refresh now
	}
	// Schedule at (expiry - buffer) with jitter
	delay := time.Duration(remaining-bufSecs) * time.Second
	// Add jitter ±10%
	jitter := time.Duration(float64(delay) * 0.1 * (1 - 2*float64(token.ExpiresAt%100)/100))
	return delay + jitter
}
