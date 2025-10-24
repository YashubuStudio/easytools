//go:build !windows

package util

// AttachConsole is a no-op on non-Windows platforms where consoles are managed
// by the terminal/TTY subsystem.
func AttachConsole() error {
	return nil
}
