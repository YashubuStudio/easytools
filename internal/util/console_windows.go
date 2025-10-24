//go:build windows

package util

import (
	"os"

	"golang.org/x/sys/windows"
)

// AttachConsole ensures that the process has a console when running in CLI mode.
// On GUI subsystem builds we have to attach to the parent console so that stdout
// and stderr are visible. If no parent console exists we allocate a new one.
func AttachConsole() error {
	// If we already have a console attached, nothing to do.
	if windows.GetConsoleWindow() != 0 {
		return nil
	}

	// Try to attach to the parent process' console so output is visible in
	// the invoking terminal (e.g. PowerShell or cmd).
	if err := windows.AttachConsole(windows.ATTACH_PARENT_PROCESS); err == nil {
		return redirectStdIO()
	} else if err != windows.ERROR_ACCESS_DENIED && err != windows.ERROR_INVALID_HANDLE {
		return err
	}

	// Fallback: allocate a new console window if attaching failed.
	if err := windows.AllocConsole(); err != nil {
		return err
	}

	return redirectStdIO()
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
