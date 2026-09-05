package layout

import (
	"os"
	"testing"

	"github.com/rivo/tview"
)

// Re-opening the "Add New Worktree" modal has to give a clean form: the branch
// name typed before cancelling must not be there anymore and the form items
// must not pile up on every open.
func TestShowWorktreeModalStartsFresh(t *testing.T) {
	repoPath := keepWorkingDirectory(t)

	l := NewLayout(tview.NewApplication(), repoPath)

	l.showWorktreeModal(repoPath)
	input, ok := l.WorktreeModal.GetFormItemByLabel("Enter Branch Name").(*tview.InputField)
	if !ok {
		t.Fatal("expected a branch name input field")
	}
	input.SetText("feature/typed-then-cancelled")

	l.dismissModal(repoPath)
	l.showWorktreeModal(repoPath)

	if got := l.WorktreeModal.GetFormItemCount(); got != 1 {
		t.Errorf("form item count = %d, want 1", got)
	}
	if got := l.WorktreeModal.GetButtonCount(); got != 2 {
		t.Errorf("button count = %d, want 2", got)
	}

	input, ok = l.WorktreeModal.GetFormItemByLabel("Enter Branch Name").(*tview.InputField)
	if !ok {
		t.Fatal("expected a branch name input field")
	}
	if got := input.GetText(); got != "" {
		t.Errorf("branch name input = %q, want it to be reset to empty", got)
	}
}

// Cancelling the modal has to leave a usable action list behind: putting the
// main layout back focuses the repository list, whose focus handler clears the
// action list, so it has to be rebuilt before it is focused again.
func TestDismissModalLeavesUsableActionList(t *testing.T) {
	repoPath := keepWorkingDirectory(t)

	l := NewLayout(tview.NewApplication(), repoPath)
	l.showWorktreeModal(repoPath)
	l.dismissModal(repoPath)

	if l.ActionList.GetItemCount() == 0 {
		t.Fatal("action list is empty after cancelling the modal")
	}

	first, _ := l.ActionList.GetItemText(0)
	if first != " .." {
		t.Errorf("first action = %q, want %q", first, " ..")
	}
}

// keepWorkingDirectory returns a temporary directory to work in and restores
// the working directory after the test, since the application navigates the
// file system with os.Chdir.
func keepWorkingDirectory(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	return t.TempDir()
}
