//go:build !windows

package lock

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// isWizardProcess reports whether the running PID corresponds to a process
// whose binary name matches the expected wizard name (case-insensitive).
// Tries /proc/<pid>/comm first (Linux), then falls back to `ps` (macOS, BSD).
func isWizardProcess(pid int, expectedName string) bool {
	expected := strings.ToLower(filepath.Base(expectedName))
	expected = strings.TrimSuffix(expected, ".exe")

	got := readProcessName(pid)
	if got == "" {
		// Can't determine — assume match (graceful degradation).
		return true
	}
	return strings.HasPrefix(got, expected) || strings.HasPrefix(expected, got)
}

// readProcessName returns the lowercase basename of the binary running with the given PID.
// Empty string means could not determine.
func readProcessName(pid int) string {
	// Try Linux first: /proc/<pid>/comm
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm"); err == nil {
		name := strings.ToLower(strings.TrimSpace(string(data)))
		return strings.TrimSuffix(name, ".exe")
	}
	// macOS / BSD fallback: ps -p <pid> -o comm=
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(string(out)))
	// ps may return the full path on macOS — take basename.
	name = filepath.Base(name)
	return strings.TrimSuffix(name, ".exe")
}
