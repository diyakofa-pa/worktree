//go:build darwin

package utils

import (
	"strings"
	"testing"
)

func TestTerminalScriptForProgramUnknown(t *testing.T) {
	if _, ok := terminalScriptForProgram("SomeOtherTerminal", "/tmp/example"); ok {
		t.Error("terminalScriptForProgram() ok = true, want false for an unscripted terminal")
	}
}

func TestTerminalScriptForProgramGhostty(t *testing.T) {
	script, ok := terminalScriptForProgram("ghostty", "/tmp/example")
	if !ok {
		t.Fatal("terminalScriptForProgram() ok = false, want true for ghostty")
	}
	if !strings.Contains(script, `tell application "Ghostty"`) {
		t.Errorf("script does not target Ghostty: %s", script)
	}
	if !strings.Contains(script, `"/tmp/example"`) {
		t.Errorf("script does not embed the target path: %s", script)
	}
}

func TestTerminalScriptForProgramITerm(t *testing.T) {
	script, ok := terminalScriptForProgram("iTerm.app", "/tmp/example")
	if !ok {
		t.Fatal("terminalScriptForProgram() ok = false, want true for iTerm.app")
	}
	if !strings.Contains(script, `tell application "iTerm2"`) {
		t.Errorf("script does not target iTerm2: %s", script)
	}
}

func TestTerminalScriptForProgramAppleTerminal(t *testing.T) {
	script, ok := terminalScriptForProgram("Apple_Terminal", "/tmp/example")
	if !ok {
		t.Fatal("terminalScriptForProgram() ok = false, want true for Apple_Terminal")
	}
	if !strings.Contains(script, `tell application "Terminal"`) {
		t.Errorf("script does not target Terminal: %s", script)
	}
}

func TestAppleScriptStringEscapesQuotesAndBackslashes(t *testing.T) {
	got := appleScriptString(`a "quoted" \path`)
	want := `"a \"quoted\" \\path"`
	if got != want {
		t.Errorf("appleScriptString() = %s, want %s", got, want)
	}
}
