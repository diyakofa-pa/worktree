//go:build !windows

package utils

import "syscall"

// execShell replaces the current process image with shell, argv0 set to the
// same path. On success it never returns: the calling process becomes the
// shell in place, so there is no parent left holding open file descriptors
// or waiting on a child, and no nested shell to exit back out of.
func execShell(shell string, env []string) error {
	return syscall.Exec(shell, []string{shell}, env)
}
