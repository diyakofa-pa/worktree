package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShellCommandPrefersLoginShell(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh is not installed")
	}

	t.Setenv("SHELL", sh)

	if got := shellCommand(); got != sh {
		t.Errorf("shellCommand() = %q, want %q", got, sh)
	}
}

func TestShellCommandFallsBackToBash(t *testing.T) {
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

func TestInTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	if inTmux() {
		t.Error("inTmux() = true, want false when TMUX is unset")
	}

	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	if !inTmux() {
		t.Error("inTmux() = false, want true when TMUX is set")
	}
}

// TestOpenShellOpensTmuxWindowInsideTmux locks down that, inside tmux,
// OpenShell opens a new tmux window rather than taking over the calling
// process: OpenShell's non-tmux path replaces the process image outright
// (see execShell), which would tear down the test binary itself if it ever
// ran here by mistake.
func TestOpenShellOpensTmuxWindowInsideTmux(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")

	fakeTmux := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\necho \"$@\" > " + logPath + "\n"
	if err := os.WriteFile(fakeTmux, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake tmux: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	target := t.TempDir()
	opened, err := OpenShell(target, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("OpenShell() = %v, want nil", err)
	}
	if !opened {
		t.Error("OpenShell() opened = false, want true when a tmux window was opened")
	}

	// startDetached reaps the process in the background, so give the fake
	// tmux a moment to have written its args before reading them.
	deadline := time.Now().Add(2 * time.Second)
	var out []byte
	for time.Now().Before(deadline) {
		var err error
		if out, err = os.ReadFile(logPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got, want := strings.TrimSpace(string(out)), fmt.Sprintf("new-window -c %s", target); got != want {
		t.Errorf("tmux invoked with %q, want %q", got, want)
	}
}

func TestStartDetached(t *testing.T) {
	if err := startDetached(exec.Command("sh", "-c", "exit 0")); err != nil {
		t.Errorf("startDetached() = %v, want nil", err)
	}

	if err := startDetached(exec.Command("worktree-no-such-binary")); err == nil {
		t.Error("startDetached() = nil, want an error for a missing binary")
	}
}
