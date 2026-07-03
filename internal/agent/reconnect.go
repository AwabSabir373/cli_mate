package agent

import (
	"context"
	"strings"
	"time"

	"cli_mate/internal/providers"
)

// Mid-stream reconnect: a long autonomous task should survive a single
// transient upstream hiccup instead of dying and re-burning every token.
// When the initial StreamChat connect fails with a disconnect-shaped error
// before any content has been forwarded, re-issue the same request with
// backoff a few times. We retry ONLY the connect (not a partially-consumed
// stream), so no already-forwarded OnToken is ever duplicated.
const (
	maxStreamReconnects = 2
	streamReconnectBase = 500 * time.Millisecond
)

// reconnectNotifier is called before each retry with the 1-based attempt
// number and the max, so the caller can surface a notice. Nil is fine.
type reconnectNotifier func(attempt, max int)

// streamWithReconnect issues request via provider.StreamChat and, on a
// transient disconnect error, retries the connect up to maxStreamReconnects
// times with exponential backoff. It returns the live stream on success, or
// the last error.
func streamWithReconnect(ctx context.Context, provider providers.Provider, request providers.ChatRequest, notify reconnectNotifier) (<-chan providers.StreamEvent, error) {
	stream, err := provider.StreamChat(ctx, request)
	if err == nil {
		return stream, nil
	}
	for attempt := 1; attempt <= maxStreamReconnects; attempt++ {
		if !shouldReconnect(ctx, err) {
			return nil, err
		}
		if notify != nil {
			notify(attempt, maxStreamReconnects)
		}
		if waitErr := sleepWithContext(ctx, backoffFor(attempt)); waitErr != nil {
			return nil, err // ctx cancelled while waiting; surface the original error
		}
		stream, err = provider.StreamChat(ctx, request)
		if err == nil {
			return stream, nil
		}
	}
	return nil, err
}

// shouldReconnect reports whether err is a transient disconnect worth retrying.
// It excludes context cancellation/expiry and context-limit errors (handled
// by compaction), so the reconnect path never fights existing handlers.
func shouldReconnect(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if isContextLimitError(msg) {
		return false
	}
	for _, needle := range []string{
		"eof",
		"connection reset",
		"connection refused",
		"broken pipe",
		"connection closed",
		"timeout",
		"timed out",
		"temporarily unavailable",
		"i/o timeout",
		"503",
		"502",
		"server closed",
		"unexpected end",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func backoffFor(attempt int) time.Duration {
	d := streamReconnectBase
	for i := 1; i < attempt; i++ {
		d *= 2
	}
	return d
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
