//go:build windows

package lock

import (
	"os"
)

// isProcessRunning reports whether a process with the given PID is currently running.
// On Windows, we use os.FindProcess which returns an error if the PID doesn't exist.
// We then attempt to open the process to verify it's actually running (not just a
// handle from a recycled PID).
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess only fails if the PID is 0 or similar invalid value.
	// We attempt to query the process state by trying to get its exit code via
	// a zero-timeout wait. If the process is still running, Wait will return an
	// error indicating it's still alive (not yet exited).
	// The simplest reliable approach: use OpenProcess via the process handle.
	// Since proc.Wait() would consume the process, we use a non-destructive check.
	// We rely on the fact that Release doesn't kill the process.
	_ = proc.Release()

	// Use the Windows API via the os package: try to open with access rights.
	// A simple heuristic: check if we can open /proc/<pid> style path — not available
	// on Windows. Instead we use the known limitation and just try FindProcess again
	// with a signal-0 equivalent: attempt to open the process for query.
	return windowsProcessExists(pid)
}
