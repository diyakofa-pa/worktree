package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShellCommandPrefersBash(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not installed")
	}

	t.Setenv("SHELL", "/definitely/not/a/shell")

	if got := shellCommand(); got != bash {
		t.Errorf("shellCommand() = %q, want %q", got, bash)
	}
}

func TestShellCommandIsExecutable(t *testing.T) {
	shell := shellCommand()

	info, err := os.Stat(shell)
	if err != nil {
		t.Fatalf("shellCommand() = %q, which does not exist: %v", shell, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("shellCommand() = %q, which is not executable", shell)
	}
	if !filepath.IsAbs(shell) {
		t.Errorf("shellCommand() = %q, want an absolute path", shell)
	}
}
