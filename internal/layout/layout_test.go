package layout

import (
	"os"
	"testing"

	"github.com/rivo/tview"
)

// A selected worktree offers a shell first (so the worktree can be used from
// the command line), then the editors.
func TestSelectedWorktreeActionListOptions(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	l := NewLayout(tview.NewApplication(), t.TempDir())
	l.selectedWorktreeActionList(t.TempDir(), "feature/login")

	want := []string{" ..", "Open Bash", "Open VS Code", "Open Cursor", "Remove Worktree"}
	if got := l.ActionList.GetItemCount(); got != len(want) {
		t.Fatalf("action count = %d, want %d", got, len(want))
	}

	for i, wantItem := range want {
		gotItem, _ := l.ActionList.GetItemText(i)
		if gotItem != wantItem {
			t.Errorf("action %d = %q, want %q", i, gotItem, wantItem)
		}
	}
}
