//go:build darwin

package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openTerminalTab asks the surrounding GUI terminal emulator to open a new
// tab already rooted at path, for the emulators known to expose a scriptable
// way to do so. It reports whether that succeeded; the caller falls back to
// taking over the current pane when it did not, whether because the
// emulator isn't one of these or because the attempt itself failed.
func openTerminalTab(path string, logger func(format string, args ...interface{})) bool {
	script, ok := terminalScriptForProgram(os.Getenv("TERM_PROGRAM"), path)
	if !ok {
		return false
	}

	logger("Opening a new terminal tab in %v ...", path)

	cmd := exec.Command("osascript")
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logger("Failed to open a new terminal tab, falling back: %s", strings.TrimSpace(stderr.String()))
		return false
	}

	return true
}

// terminalScriptForProgram returns the AppleScript that opens a new tab at
// path for the given $TERM_PROGRAM value, and whether that program is one
// this knows how to script at all.
func terminalScriptForProgram(program string, path string) (string, bool) {
	switch program {
	case "ghostty":
		return fmt.Sprintf(`tell application "Ghostty"
	activate
	set cfg to new surface configuration
	set initial working directory of cfg to %s
	new tab in front window with configuration cfg
end tell`, appleScriptString(path)), true

	case "iTerm.app":
		return fmt.Sprintf(`tell application "iTerm2"
	activate
	tell current window
		create tab with default profile
		tell current session
			write text "cd " & quoted form of %s
		end tell
	end tell
end tell`, appleScriptString(path)), true

	case "Apple_Terminal":
		return fmt.Sprintf(`tell application "Terminal"
	do script "cd " & quoted form of %s in front window
	activate
end tell`, appleScriptString(path)), true

	default:
		return "", false
	}
}

// appleScriptString renders s as a double-quoted AppleScript string literal.
func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
