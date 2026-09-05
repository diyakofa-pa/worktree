package layout

import (
	"testing"

	"github.com/rivo/tview"
)

// Re-opening the "Add New Worktree" modal has to give a clean form: the branch
// name typed before cancelling must not be there anymore and the form items
// must not pile up on every open.
func TestShowWorktreeModalStartsFresh(t *testing.T) {
	l := NewLayout(tview.NewApplication(), ".")

	l.showWorktreeModal(".")
	input, ok := l.WorktreeModal.GetFormItemByLabel("Enter Branch Name").(*tview.InputField)
	if !ok {
		t.Fatal("expected a branch name input field")
	}
	input.SetText("feature/typed-then-cancelled")

	l.dismissModal()
	l.showWorktreeModal(".")

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
