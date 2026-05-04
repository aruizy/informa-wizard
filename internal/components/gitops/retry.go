// Package gitops provides shared utilities for git operations.
package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ForceEnglish forces git to emit English error messages on the given command
// by ensuring LC_ALL=C and LANG=C are set. Existing entries in cmd.Env are
// preserved (so test helpers and other custom env entries remain intact); only
// the locale variables are forced to C. Pass the *exec.Cmd before Run /
// CombinedOutput is called.
//
// When cmd.Env is nil, the parent environment is used as the base. Go's exec
// honors the LAST occurrence of each variable, so appending LC_ALL=C and
// LANG=C overrides any earlier locale settings.
func ForceEnglish(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "LC_ALL=C", "LANG=C")
}

// nonRetryableSubstrings is the list of (lowercase) substrings the retry loop
// matches in an error's text to detect failures that will NOT improve with
// another attempt (auth failure, missing repo, missing credentials, etc.).
// Hits short-circuit the retry and return the original error immediately.
//
// Entries MUST be lowercase: matching is case-insensitive (the message is
// lowercased once before the loop). Locale-related variation is mitigated by
// forcing English git output (LC_ALL=C / LANG=C) at the command-exec layer.
var nonRetryableSubstrings = []string{
	"authentication failed",
	"repository not found",
	"fatal: not a git repository",
	"could not read username",
	"permission denied (publickey)",
	"invalid username or password",
}

// IsNonRetryable reports whether err's text matches a known non-transient
// failure mode. The check is substring-based on the rendered error string,
// since git surfaces these via stderr/combined output captured into wrapped
// errors. Matching is case-insensitive.
func IsNonRetryable(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, sub := range nonRetryableSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

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

		// Don't retry on permanent failures: auth, missing repo, etc.
		if IsNonRetryable(lastErr) {
			return lastErr
		}

		if i < attempts-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}

	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}
