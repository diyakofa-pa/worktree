package core

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// commandTimeout bounds every git invocation so a hung command can never wedge
// the CLI or the TUI.
const commandTimeout = 60 * time.Second

// git runs a git command inside dir and returns its trimmed stdout. Using
// "git -C" instead of changing the process working directory keeps the
// operations independent of each other and safe to call from anywhere.
func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("git %s timed out after %s", strings.Join(args, " "), commandTimeout)
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// gitOK reports whether a git command succeeded, discarding its output. It is
// used for the query style commands git exposes through its exit code.
func gitOK(dir string, args ...string) bool {
	_, err := git(dir, args...)
	return err == nil
}
