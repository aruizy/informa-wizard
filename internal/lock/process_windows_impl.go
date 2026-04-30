//go:build windows

package lock

import (
	"golang.org/x/sys/windows"
)

// windowsProcessExists checks if a process with the given PID is running by
// attempting to open it with PROCESS_QUERY_LIMITED_INFORMATION access.
// If the open succeeds, the process exists. We then verify it hasn't exited
// by checking its exit code.
func windowsProcessExists(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle) //nolint:errcheck

	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}
	// STILL_ACTIVE (259) means the process is still running.
	return exitCode == 259
}
