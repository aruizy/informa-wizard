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
//
// On Linux we prefer /proc/<pid>/exe (full executable path, not truncated).
// /proc/<pid>/comm is limited by TASK_COMM_LEN (16 bytes including NUL → 15 chars),
// so a binary like "informa-wizard-dev" (18 chars) reads back as "informa-wizard-"
// and a strict equality check would falsely report "not the wizard" — allowing the
// running wizard's lock to be stolen. We accept either an exact match or a truncated
// prefix match (got is a prefix of expected AND got is at the truncation length).
func isWizardProcess(pid int, expectedName string) bool {
	expected := strings.ToLower(filepath.Base(expectedName))
	expected = strings.TrimSuffix(expected, ".exe")

	got, truncated := readProcessName(pid)
	if got == "" {
		// Can't determine — assume match (graceful degradation).
		return true
	}
	if got == expected {
		return true
	}
	// Truncation case: comm was capped at 15 chars; treat as match if got is a prefix of expected.
	if truncated && len(got) >= 15 && strings.HasPrefix(expected, got) {
		return true
	}
	return false
}

// readProcessName returns the lowercase basename of the binary running with the given PID,
// plus a flag indicating whether the source could have truncated the name (comm/ps -o comm=).
// Empty string means could not determine.
func readProcessName(pid int) (string, bool) {
	// Linux preferred: /proc/<pid>/exe symlink → full path, no truncation.
	if target, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe"); err == nil && target != "" {
		name := strings.ToLower(filepath.Base(target))
		return strings.TrimSuffix(name, ".exe"), false
	}
	// Linux fallback: /proc/<pid>/comm — truncated to TASK_COMM_LEN-1 (15 chars).
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm"); err == nil {
		name := strings.ToLower(strings.TrimSpace(string(data)))
		return strings.TrimSuffix(name, ".exe"), true
	}
	// macOS / BSD fallback: ps -p <pid> -o comm=
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return "", false
	}
	name := strings.ToLower(strings.TrimSpace(string(out)))
	// ps may return the full path on macOS — take basename.
	name = filepath.Base(name)
	return strings.TrimSuffix(name, ".exe"), false
}
