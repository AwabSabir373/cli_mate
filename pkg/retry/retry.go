package retry

import (
	"context"
	"fmt"
	"time"
)

type RetriableStatusError struct {
	StatusCode int
}

func (e RetriableStatusError) Error() string {
	return fmt.Sprintf("retriable status code: %d", e.StatusCode)
}

func Do(ctx context.Context, attempts int, baseDelay time.Duration, fn func() error) error {
	if attempts < 1 {
		attempts = 1
	}
	if baseDelay <= 0 {
		baseDelay = 100 * time.Millisecond
	}

	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt == attempts {
			return err
		}

		delay := baseDelay * time.Duration(1<<uint(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}
