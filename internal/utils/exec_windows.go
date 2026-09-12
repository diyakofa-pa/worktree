//go:build windows

package utils

import (
	"os"
	"os/exec"
)

// execShell has no process-replacement equivalent on Windows, so it falls
// back to running the shell as a child that inherits the console and
// blocking until it exits.
func execShell(shell string, env []string) error {
	cmd := exec.Command(shell)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
