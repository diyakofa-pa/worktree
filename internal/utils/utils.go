package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func ListDirectories(path string) ([]string, error) {
	var dirs []string

	files, err := os.ReadDir(path)

	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file.Name())
		}
	}

	return dirs, nil
}

func CheckMainOrMasterExists(selectedRepo string) (string, error) {
	mainExists, err := dirExists(filepath.Join(selectedRepo, "main"))
	if err != nil {
		return "", err
	}
	if mainExists {
		return "main", nil
	}
	masterExists, err := dirExists(filepath.Join(selectedRepo, "master"))

	if err != nil {
		return "", err
	}
	if masterExists {
		return "master", nil
	}

	return "", fmt.Errorf("main or master branch not found")
}

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func CreateNewBranch(branchName string, mainBranchName string, logger func(format string, args ...interface{})) error {
	destinationPath := filepath.Join("..", branchName)

	cmd := exec.Command("git", "worktree", "add", "-b", branchName, destinationPath, mainBranchName)
	return cmd.Run()
}

func PrepareDestinationConfig(branchName string, logger func(format string, args ...interface{})) error {
	if _, err := os.Stat("workspace.code-workspace"); os.IsNotExist(err) {
		logger("Workspace file not found")
		return nil
	}

	workspaceFile := "workspace.code-workspace"
	src := filepath.Join(".", workspaceFile)
	dest := filepath.Join("..", branchName, workspaceFile)

	err := copyFile(src, dest)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying file contents: %v", err)
	}

	return nil
}

func OpenVSCode(openingDir string, logger func(format string, args ...interface{})) error {
	_, err := os.Stat(filepath.Join(openingDir, "workspace.code-workspace"))
	if !os.IsNotExist(err) {
		openingDir = filepath.Join(openingDir, "workspace.code-workspace")
	}

	logger("Opening vscode ...")

	return startDetached(exec.Command("code", openingDir))
}

// startDetached starts a GUI application without blocking the UI and reaps it
// in the background. Without the Wait the finished process would stay around
// as a zombie for the whole lifetime of the application, once per launch.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

func GetWorktrees(logger func(format string, args ...interface{})) ([]string, error) {
	mainBranchName, err := CheckMainOrMasterExists(".")
	if err != nil || mainBranchName == "" {
		return nil, fmt.Errorf("failed checking main or master branch: %w", err)
	}

	if err := ChangeDirectory(mainBranchName); err != nil {
		return nil, fmt.Errorf("failed to change directory to main branch %w", err)
	}

	if err := exec.Command("git", "status").Run(); err != nil {
		return nil, fmt.Errorf("failed to run git status: %w", err)
	}

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()

	if err != nil {
		return nil, fmt.Errorf("failed to get worktrees: %w", err)
	}

	lines := bytes.Split(output, []byte("\n"))
	var branches []string
	for i := 0; i < len(lines); i++ {
		if bytes.HasPrefix(lines[i], []byte("worktree ")) {
			for j := i + 1; j < len(lines) && !bytes.HasPrefix(lines[j], []byte("worktree ")); j++ {
				if bytes.HasPrefix(lines[j], []byte("branch ")) {
					branch := strings.TrimSpace(string(bytes.TrimPrefix(lines[j], []byte("branch refs/heads/"))))
					branches = append(branches, branch)
				}
			}
		}
	}

	if err := ChangeDirectory(".."); err != nil {
		return nil, fmt.Errorf("failed to change directory to parent: %w", err)
	}

	return branches, nil
}

func AddWorktree(path string, newBranchName string, logger func(format string, args ...interface{})) error {
	if err := ChangeDirectory(path); err != nil {
		return fmt.Errorf("failed to change directory to %v: %w", path, err)
	}

	mainBranchName, err := CheckMainOrMasterExists(".")
	if err != nil || mainBranchName == "" {
		logger("Failed checking main or master branch, error: %v", err)
		return err
	}

	if err := ChangeDirectory(mainBranchName); err != nil {
		logger("Failed to go to %s dir, error: %v", mainBranchName, err)

		return err
	}

	logger("Adding worktree %v", newBranchName)
	if err := CreateNewBranch(newBranchName, mainBranchName, logger); err != nil {
		logger("Failed to create a new branch, error: %v", err)

		return err
	}

	if err := PrepareDestinationConfig(newBranchName, logger); err != nil {
		logger("Failed to prepare destination config, error: %v", err)

		return err
	}

	logger("Worktree %v added successfully", newBranchName)

	return nil
}

func RemoveWorktree(path string, branchName string, logger func(format string, args ...interface{})) error {
	dirLevel := strings.Count(branchName, "/") + 1
	if err := ChangeDirectory(strings.Repeat("../", dirLevel)); err != nil {
		logger("Failed to change directory to repository, error: %v", err)
	}

	mainBranchName, err := CheckMainOrMasterExists(".")
	if err != nil || mainBranchName == "" {
		logger("Failed checking main or master branch, error: %v", err)
	}

	if err := ChangeDirectory(mainBranchName); err != nil {
		logger("Failed to change directory to main branch, error: %v", err)
	}

	logger("Removing worktree %v", path)
	if err := execCommand("git", "worktree", "remove", path); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	logger("Removing git branch %v", branchName)
	if err := execCommand("git", "branch", "-D", branchName); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	return nil
}

func execCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(60 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		return fmt.Errorf("command timed out")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("command failed: %s", errBuffer.String())
		}
	}

	return nil
}

func Pwd() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get current directory, error: %s", err)
		return ""
	}

	return dir
}

func ChangeDirectory(path string) error {
	if err := os.Chdir(path); err != nil {
		return fmt.Errorf("failed to change directory to %v: %w", path, err)
	}

	return nil
}

// OpenShell hands the terminal to the user's own shell rooted at path, so
// picking a worktree feels like a plain "cd" into it rather than nesting a
// throwaway shell inside a suspended TUI. It reports whether a separate
// terminal session was opened elsewhere (tmux, or a GUI terminal emulator
// with tab-opening support), so the caller can close the app outright
// instead of resuming it.
//
// Inside tmux, a new window is opened. Under a GUI terminal emulator that
// exposes a way to script a new tab (Ghostty, iTerm2, Terminal.app), a new
// tab is opened there instead. Both leave the app's own pane free to close,
// since the worktree directory is already open elsewhere.
//
// Everywhere else, the worktree app's own process image is replaced by the
// shell (see execShell): the app closes and the same terminal tab becomes an
// interactive shell already sitting in the worktree directory, with nothing
// left to "exit" back out of.
func OpenShell(path string, logger func(format string, args ...interface{})) (bool, error) {
	if inTmux() {
		logger("Opening a new tmux window in %v ...", path)
		if err := startDetached(exec.Command("tmux", "new-window", "-c", path)); err != nil {
			return false, err
		}
		return true, nil
	}

	if openTerminalTab(path, logger) {
		return true, nil
	}

	shell := shellCommand()
	logger("Opening %v in %v ...", filepath.Base(shell), path)

	if err := os.Chdir(path); err != nil {
		return false, fmt.Errorf("failed to change directory to %v: %w", path, err)
	}

	// On success this replaces the current process and never returns.
	if err := execShell(shell, os.Environ()); err != nil {
		return false, fmt.Errorf("failed to run %v: %w", shell, err)
	}

	return false, nil
}

func inTmux() bool {
	return os.Getenv("TMUX") != ""
}

// shellCommand prefers the user's own login shell, so the session keeps their
// aliases, prompt, and history instead of dropping them into a bare bash.
// It falls back to bash, then sh, so a shell is always available.
//
// Every candidate is resolved through exec.LookPath before use: execShell's
// unix implementation execve(2)s the result directly, which (unlike a shell)
// never searches PATH for a bare name, so returning one unresolved could run
// whatever file happens to have that name in the worktree's own directory
// instead of a real shell.
func shellCommand() string {
	candidates := []string{os.Getenv("SHELL"), "bash", "sh"}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	// Every real candidate above failed to resolve, which should only happen
	// on a system with no shell installed at all. "/bin/sh" is a path, not a
	// bare name, so execve(2) treats it literally instead of searching PATH
	// or falling back to the current directory; it will simply fail to start
	// (a clean, fail-closed error) rather than resolve to something else.
	return "/bin/sh"
}

func OpenCursor(path string, logger func(format string, args ...interface{})) error {
	logger("Opening cursor ...")
	workspaceFile := "workspace.code-workspace"
	_, err := os.Stat(filepath.Join(path, workspaceFile))
	if !os.IsNotExist(err) {
		path = filepath.Join(path, workspaceFile)
	}

	return startDetached(exec.Command("cursor", path))
}
