package layout

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"worktree/internal/utils"

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

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	return entryPoint
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

	var items []string
	for i := 0; i < l.ActionList.GetItemCount(); i++ {
		mainText, _ := l.ActionList.GetItemText(i)
		items = append(items, mainText)
	}
	if !contains(items, "Remove Worktree") {
		t.Errorf("action list items = %v, want the worktree actions to be shown", items)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
