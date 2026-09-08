package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// OpenIn launches an editor on a worktree without waiting for it to exit. When
// the worktree carries a workspace file the editor is pointed at that instead,
// so multi-root workspaces open as one window.
func OpenIn(command, path string) error {
	if command == "" {
		return fmt.Errorf("no editor command given")
	}

	target := path
	if workspace := filepath.Join(path, WorkspaceFile); fileExists(workspace) {
		target = workspace
	}

	cmd := exec.Command(command, target)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run %q: %w", command, err)
	}

	// The editor outlives this process, so release it rather than waiting.
	return cmd.Process.Release()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
