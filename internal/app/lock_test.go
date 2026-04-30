package app

import (
	"testing"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/lock"
)

// TestMain sets up the test environment for the app package.
// It disables the lock file for all tests so that parallel test runs
// do not contend on the real user home directory lock.
func TestMain(m *testing.M) {
	// Replace the lock acquisition with a no-op so tests never write to
	// ~/.informa-wizard/wizard.lock and do not block each other.
	lockAcquireFn = func(_ string) (*lock.Lock, error) {
		return nil, nil
	}

	m.Run()
}
