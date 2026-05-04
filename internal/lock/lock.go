// Package lock provides a PID-based lock file to prevent concurrent wizard runs.
package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const lockFile = "wizard.lock"

// Lock represents an acquired lock file.
type Lock struct {
	path string
}

// Acquire tries to acquire the lock file at ~/.informa-wizard/wizard.lock.
// If the lock is held by a running process, it returns an error containing the PID.
// If the lock is stale (process no longer running), it removes the old file and claims it.
// If homeDir is empty or the directory cannot be created, Acquire returns (nil, nil)
// so the caller can continue without locking (graceful degradation).
func Acquire(homeDir string) (*Lock, error) {
	if homeDir == "" {
		return nil, nil //nolint:nilnil
	}

	dir := filepath.Join(homeDir, ".informa-wizard")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		// Cannot create the directory — skip locking rather than blocking the user.
		return nil, nil //nolint:nilnil
	}

	path := filepath.Join(dir, lockFile)

	for {
		// Attempt to create the lock file exclusively.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// Write PID + process name so a reused PID by another binary
			// doesn't trigger a false "already locked" error.
			pid := os.Getpid()
			exe, _ := os.Executable()
			exeName := filepath.Base(exe)
			_, writeErr := fmt.Fprintf(f, "%d\n%s", pid, exeName)
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("write lock file: %v / %v", writeErr, closeErr)
			}
			return &Lock{path: path}, nil
		}

		if !os.IsExist(err) {
			// Unexpected error — skip locking.
			return nil, nil //nolint:nilnil
		}

		// File already exists — read the PID and check if the process is alive.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			// File disappeared between our open and read (race) — retry.
			continue
		}

		// Parse: first line is PID, second line (optional) is exe name.
		lines := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(lines[0]))
		if parseErr != nil {
			// Corrupted lock file — remove and retry.
			_ = os.Remove(path)
			continue
		}
		var lockedExe string
		if len(lines) > 1 {
			lockedExe = strings.TrimSpace(lines[1])
		}

		// Legacy lock files (written before exe-name was recorded) only have a PID.
		// We can't verify by exe name, but if the PID is still alive we must NOT
		// reclaim — that PID may belong to a still-running legacy wizard, and
		// stealing its lock would let two wizards run concurrently during the
		// version transition. Only reclaim if the PID is dead.
		if lockedExe == "" {
			if isProcessRunning(pid) {
				return nil, fmt.Errorf(
					"another informa-wizard instance may be running (PID %d, legacy lock without exe name). "+
						"Wait for it to finish or remove %s if you're sure no instance is running.",
					pid, path,
				)
			}
			fmt.Fprintf(os.Stderr, "Warning: legacy lock file at %s (no exe name); reclaiming\n", path)
			_ = os.Remove(path)
			continue
		}

		// If process is running and it matches the wizard binary, refuse to claim.
		if isProcessRunning(pid) && isWizardProcess(pid, lockedExe) {
			return nil, fmt.Errorf(
				"another informa-wizard instance is running (PID %d). "+
					"Wait for it to finish or remove %s if you're sure no instance is running.",
				pid, path,
			)
		}

		// Stale lock (process gone, or PID reused by a different binary) — remove and retry.
		_ = os.Remove(path)
	}
}

// Release removes the lock file.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	return os.Remove(l.path)
}
