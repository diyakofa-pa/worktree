package layout

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"worktree/internal/utils"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newTestEntryPoint creates an entry point directory holding a single
// repository ("myrepo") with an initialised "main" branch, mirroring the
// layout the application expects.
func newTestEntryPoint(t *testing.T) string {
	t.Helper()

	entryPoint := t.TempDir()
	mainDir := filepath.Join(entryPoint, "myrepo", "main")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed, skipping: %v: %s", args, err, out)
		}
	}

	keepWorkingDirectory(t)

	return entryPoint
}

// keepWorkingDirectory restores the working directory after the test, since
// the application navigates the file system with os.Chdir.
func keepWorkingDirectory(t *testing.T) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

// A newly created worktree must be the one the user ends up in: the repository
// stays selected on the left panel and the right panel already shows the
// actions of the new worktree.
func TestSetupActionsAndSelectOpensNewWorktree(t *testing.T) {
	entryPoint := newTestEntryPoint(t)
	repoPath := filepath.Join(entryPoint, "myrepo")

	l := NewLayout(tview.NewApplication(), entryPoint)
	l.SetupLayoutContentMenus(entryPoint)

	if err := utils.AddWorktree(repoPath, "feature/login", l.Log); err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	l.setupActionsAndSelect(repoPath, "feature/login")

	repoName, _ := l.LeftList.GetItemText(l.LeftList.GetCurrentItem())
	if repoName != "myrepo" {
		t.Errorf("selected repository = %q, want %q", repoName, "myrepo")
	}

	worktreePath := filepath.Join(repoPath, "feature/login")
	if title := l.ActionList.GetTitle(); !strings.Contains(title, worktreePath) {
		t.Errorf("action list title = %q, want it to contain %q", title, worktreePath)
	}

	if !contains(actionListItems(l), "Remove Worktree") {
		t.Errorf("action list items = %v, want the worktree actions to be shown", actionListItems(l))
	}
}

// Going back from a worktree has to rebuild the actions of the repository it
// belongs to, otherwise the user is stuck in the worktree that was opened for
// them after creation.
func TestWorktreeBackActionRebuildsRepositoryActions(t *testing.T) {
	entryPoint := newTestEntryPoint(t)
	repoPath := filepath.Join(entryPoint, "myrepo")

	l := NewLayout(tview.NewApplication(), entryPoint)
	l.SetupLayoutContentMenus(entryPoint)

	if err := utils.AddWorktree(repoPath, "feature/login", l.Log); err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	l.setupActionsAndSelect(repoPath, "feature/login")

	l.ActionList.SetCurrentItem(0) // the ".." entry
	selectCurrentAction(l)

	items := actionListItems(l)
	if !contains(items, " Add New Worktree") {
		t.Errorf("action list items = %v, want the repository actions to be back", items)
	}
	if title := l.ActionList.GetTitle(); !strings.HasSuffix(title, repoPath) {
		t.Errorf("action list title = %q, want it to end with %q", title, repoPath)
	}
}

// A selected worktree offers a shell first (so the worktree can be used from
// the command line), then the editors.
func TestSelectedWorktreeActionListOptions(t *testing.T) {
	keepWorkingDirectory(t)

	entryPoint := t.TempDir()
	repoPath := filepath.Join(entryPoint, "myrepo")
	worktreePath := filepath.Join(repoPath, "feature", "login")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("failed to create worktree directory: %v", err)
	}

	l := NewLayout(tview.NewApplication(), entryPoint)
	l.selectedWorktreeActionList(worktreePath, "feature/login", repoPath)

	want := []string{" ..", "Open Bash", "Open VS Code", "Open Cursor", "Remove Worktree"}
	got := actionListItems(l)
	if len(got) != len(want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("action %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// selectCurrentAction runs the selected function of the highlighted action, the
// same way pressing Enter on it would.
func selectCurrentAction(l *Layout) {
	l.ActionList.InputHandler()(
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
		func(p tview.Primitive) {},
	)
}

func actionListItems(l *Layout) []string {
	var items []string
	for i := 0; i < l.ActionList.GetItemCount(); i++ {
		mainText, _ := l.ActionList.GetItemText(i)
		items = append(items, mainText)
	}

	return items
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}

	return false
}
