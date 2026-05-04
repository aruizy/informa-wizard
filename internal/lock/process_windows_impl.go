//go:build windows

package lock

import (
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowsProcessExists checks if a process with the given PID is running.
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
	return exitCode == 259
}

// isWizardProcess returns true if the PID corresponds to a process whose binary name
// matches the expected wizard name. Compares base name case-insensitively, ignoring .exe.
func isWizardProcess(pid int, expectedName string) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle) //nolint:errcheck

	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))
	err = queryFullProcessImageName(handle, 0, &buf[0], &size)
	if err != nil {
		// Can't determine — assume match to be conservative.
		return true
	}
	got := strings.ToLower(filepath.Base(syscall.UTF16ToString(buf[:size])))
	got = strings.TrimSuffix(got, ".exe")
	expected := strings.ToLower(filepath.Base(expectedName))
	expected = strings.TrimSuffix(expected, ".exe")
	return got == expected
}

var (
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procQueryFullProcessImageNam = modKernel32.NewProc("QueryFullProcessImageNameW")
)

func queryFullProcessImageName(handle windows.Handle, flags uint32, buf *uint16, size *uint32) error {
	r1, _, e1 := procQueryFullProcessImageNam.Call(
		uintptr(handle),
		uintptr(flags),
		uintptr(unsafe.Pointer(buf)),
		uintptr(unsafe.Pointer(size)),
	)
	if r1 == 0 {
		return e1
	}
	return nil
}
