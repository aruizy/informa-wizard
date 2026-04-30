// Package gitops provides shared utilities for git operations.
package gitops

import (
	"fmt"
	"time"
)

// RetryAttempts is the default number of attempts for git network operations.
const RetryAttempts = 3

// RetryBaseDelay is the base delay between retries (doubles on each attempt).
const RetryBaseDelay = time.Second

// RunWithRetry executes fn up to attempts times, backing off exponentially
// between retries (baseDelay, 2*baseDelay, 4*baseDelay, …).
// If gitNotFound reports true for an error, that error is returned immediately
// without retrying — git being absent is not a transient failure.
// After all attempts fail, the last error is wrapped with an attempt count.
func RunWithRetry(fn func() error, gitNotFound func(error) bool, attempts int, baseDelay time.Duration) error {
	var lastErr error
	delay := baseDelay

	for i := 0; i < attempts; i++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't retry if git is not installed — it won't improve with waiting.
		if gitNotFound(lastErr) {
			return lastErr
		}

		if i < attempts-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}

	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}
