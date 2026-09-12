//go:build !darwin

package utils

// openTerminalTab is a no-op outside macOS: none of the terminal emulators
// scripted in terminal_darwin.go are scriptable this way on other
// platforms, so the caller always falls back to taking over the current
// pane instead.
func openTerminalTab(path string, logger func(format string, args ...interface{})) bool {
	return false
}
