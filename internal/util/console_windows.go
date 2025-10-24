//go:build windows

package util

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procAttachConsole    = kernel32.NewProc("AttachConsole")
	procAllocConsole     = kernel32.NewProc("AllocConsole")
)

const attachParentProcess = ^uint32(0)

// AttachConsole ensures that the process has a console when running in CLI mode.
// On GUI subsystem builds we have to attach to the parent console so that stdout
// and stderr are visible. If no parent console exists we allocate a new one.
func AttachConsole() error {
	// If we already have a console attached, nothing to do.
	if hasConsoleWindow() {
		return nil
	}

	// Try to attach to the parent process' console so output is visible in
	// the invoking terminal (e.g. PowerShell or cmd).
	if err := attachConsole(attachParentProcess); err == nil {
		return redirectStdIO()
	} else if err != windows.ERROR_ACCESS_DENIED && err != windows.ERROR_INVALID_HANDLE {
		return err
	}

	// Fallback: allocate a new console window if attaching failed.
	if err := allocConsole(); err != nil {
		return err
	}

	return redirectStdIO()
}

func hasConsoleWindow() bool {
	hwnd, _, _ := procGetConsoleWindow.Call()
	return hwnd != 0
}

func attachConsole(pid uint32) error {
	r1, _, err := procAttachConsole.Call(uintptr(pid))
	if r1 != 0 {
		return nil
	}
	if err == nil || err == syscall.Errno(0) {
		return windows.ERROR_INVALID_HANDLE
	}
	return err
}

func allocConsole() error {
	r1, _, err := procAllocConsole.Call()
	if r1 != 0 {
		return nil
	}
	if err == nil || err == syscall.Errno(0) {
		return windows.ERROR_INVALID_HANDLE
	}
	return err
}

func redirectStdIO() error {
	if out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
	} else {
		return err
	}
	if in, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		os.Stdin = in
	}
	return nil
}
