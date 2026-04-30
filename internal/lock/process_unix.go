//go:build !windows

package lock

import (
	"os"
	"syscall"
)

// isProcessRunning reports whether a process with the given PID is currently running.
// On Unix, os.FindProcess always succeeds, so we send signal 0 to check liveness.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
