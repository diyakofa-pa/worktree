package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newEntryPoint builds a temporary entry point holding one namespace per name,
// each with a committed main branch, plus a directory that is not a repository
// at all.
func newEntryPoint(t *testing.T, names ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range names {
		newNamespaceDir(t, root, name)
	}

	if err := os.MkdirAll(filepath.Join(root, "not-a-repo", "src"), 0o755); err != nil {
		t.Fatalf("failed to create the non repository directory: %v", err)
	}

	return root
}

func newNamespaceDir(t *testing.T, root, name string) Namespace {
	t.Helper()

	base := filepath.Join(root, name, "main")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("failed to create %q: %v", base, err)
	}

	run(t, base, "init", "--initial-branch=main")
	run(t, base, "config", "user.email", "worktree@example.com")
	run(t, base, "config", "user.name", "Worktree Test")

	if err := os.WriteFile(filepath.Join(base, "README.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write the readme: %v", err)
	}
	run(t, base, "add", ".")
	run(t, base, "commit", "-m", "initial commit")

	return Namespace{Name: name, Path: filepath.Join(root, name), Base: "main", BasePath: base}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := git(dir, args...)
	if err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}

	return out
}

func mustFindNamespace(t *testing.T, root, name string) Namespace {
	t.Helper()

	ns, err := FindNamespace(root, name)
	if err != nil {
		t.Fatalf("failed to find namespace %q: %v", name, err)
	}

	return ns
}

func findWorktree(t *testing.T, worktrees []Worktree, branch string) Worktree {
	t.Helper()

	for _, worktree := range worktrees {
		if worktree.Branch == branch {
			return worktree
		}
	}

	t.Fatalf("worktree %q not found in %v", branch, branches(worktrees))
	return Worktree{}
}

func branches(worktrees []Worktree) []string {
	var names []string
	for _, worktree := range worktrees {
		names = append(names, worktree.Branch)
	}

	return names
}

func TestListNamespacesSkipsDirectoriesWithoutATrunk(t *testing.T) {
	root := newEntryPoint(t, "api", "web")

	namespaces, err := ListNamespaces(root)
	if err != nil {
		t.Fatalf("ListNamespaces failed: %v", err)
	}

	if len(namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %v", namespaces)
	}
	if namespaces[0].Name != "api" || namespaces[1].Name != "web" {
		t.Errorf("expected namespaces sorted as [api web], got %v", namespaces)
	}
	if namespaces[0].Base != "main" {
		t.Errorf("expected the trunk to be main, got %q", namespaces[0].Base)
	}
	if namespaces[0].Worktrees != 1 {
		t.Errorf("expected the trunk to count as one worktree, got %d", namespaces[0].Worktrees)
	}

	if _, err := FindNamespace(root, "not-a-repo"); err == nil {
		t.Error("expected a directory without a trunk to be rejected")
	}
}

func TestListWorktreesReportsTheTrunkFirst(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	if _, err := CreateWorktree(ns, "feature/login", CreateOptions{}); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	worktrees, err := ListWorktreesWithStatus(ns)
	if err != nil {
		t.Fatalf("ListWorktreesWithStatus failed: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected the trunk and one worktree, got %v", branches(worktrees))
	}
	if !worktrees[0].IsBase || worktrees[0].Branch != "main" {
		t.Errorf("expected main to be reported first as the trunk, got %+v", worktrees[0])
	}

	feature := worktrees[1]
	if feature.Type != "feature" || feature.Name != "login" {
		t.Errorf("expected feature/login to split into type feature and name login, got %+v", feature)
	}
	if feature.Path != filepath.Join(ns.Path, "feature", "login") {
		t.Errorf("expected the worktree under <namespace>/feature/login, got %q", feature.Path)
	}
	if feature.Status != "clean,merged" {
		t.Errorf("expected a fresh worktree to be clean and merged, got %q", feature.Status)
	}
	if feature.Head == "" {
		t.Error("expected the head commit to be reported")
	}
}

func TestCreateWorktreeChecksOutAnExistingBranch(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")
	run(t, ns.BasePath, "branch", "hotfix/crash")

	worktree, err := CreateWorktree(ns, "hotfix/crash", CreateOptions{})
	if err != nil {
		t.Fatalf("CreateWorktree failed for an existing branch: %v", err)
	}

	if _, err := CreateWorktree(ns, "hotfix/other", CreateOptions{From: "does-not-exist"}); err == nil {
		t.Error("expected an unknown ref to be rejected")
	}
	if _, err := os.Stat(filepath.Join(ns.Path, "hotfix", "other")); !os.IsNotExist(err) {
		t.Error("expected a failed creation to leave nothing behind")
	}

	if worktree.Type != "hotfix" || worktree.Name != "crash" {
		t.Errorf("unexpected worktree %+v", worktree)
	}
	if _, err := os.Stat(filepath.Join(ns.Path, "hotfix", "crash", ".git")); err != nil {
		t.Errorf("expected a git worktree on disk: %v", err)
	}

	if _, err := CreateWorktree(ns, "hotfix/crash", CreateOptions{}); err == nil {
		t.Error("expected an existing path to be rejected")
	}
}

func TestCreateWorktreeCopiesTheWorkspaceFile(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	if err := os.WriteFile(filepath.Join(ns.BasePath, WorkspaceFile), []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write the workspace file: %v", err)
	}

	worktree, err := CreateWorktree(ns, "feature/login", CreateOptions{})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree.Path, WorkspaceFile)); err != nil {
		t.Errorf("expected the workspace file to be copied: %v", err)
	}

	if _, err := CreateWorktree(ns, "feature/logout", CreateOptions{NoCopyWorkspace: true}); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ns.Path, "feature", "logout", WorkspaceFile)); !os.IsNotExist(err) {
		t.Error("expected --no-workspace to skip the workspace file")
	}
}

func TestCreateWorktreeRejectsUnsafeNames(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	for _, branch := range []string{"", "main", "../escape", "feature/../../escape", "with space", "/leading"} {
		if _, err := CreateWorktree(ns, branch, CreateOptions{}); err == nil {
			t.Errorf("expected branch %q to be rejected", branch)
		}
	}
}

func TestRemoveWorktreeProtectsUnmergedWork(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	worktree, err := CreateWorktree(ns, "feature/login", CreateOptions{})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(worktree.Path, "login.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("failed to write a file in the worktree: %v", err)
	}
	run(t, worktree.Path, "add", ".")
	run(t, worktree.Path, "commit", "-m", "add login")

	if err := RemoveWorktree(ns, "feature/login", RemoveOptions{}); err == nil {
		t.Fatal("expected an unmerged branch to be refused without force")
	}
	if _, err := git(ns.BasePath, "show-ref", "--verify", "refs/heads/feature/login"); err != nil {
		t.Error("expected the branch to survive a refused removal")
	}
	if _, err := os.Stat(filepath.Join(worktree.Path, "login.go")); err != nil {
		t.Errorf("expected a refused removal to leave the worktree untouched: %v", err)
	}

	if err := RemoveWorktree(ns, "feature/login", RemoveOptions{Force: true}); err != nil {
		t.Fatalf("forced removal failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(ns.Path, "feature")); !os.IsNotExist(err) {
		t.Error("expected the empty feature directory to be cleaned up")
	}
	if branchExists(ns, "feature/login") {
		t.Error("expected the branch to be deleted")
	}
}

func TestRemoveWorktreeKeepsTheBranchOnRequest(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	if _, err := CreateWorktree(ns, "chore/tidy", CreateOptions{}); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if err := RemoveWorktree(ns, "chore/tidy", RemoveOptions{KeepBranch: true}); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if !branchExists(ns, "chore/tidy") {
		t.Error("expected --keep-branch to leave the branch behind")
	}
	if _, err := os.Stat(filepath.Join(ns.Path, "chore", "tidy")); !os.IsNotExist(err) {
		t.Error("expected the working directory to be removed")
	}
}

func TestRemoveWorktreeRefusesTheTrunk(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	if err := RemoveWorktree(ns, "main", RemoveOptions{Force: true}); err == nil {
		t.Fatal("expected the trunk worktree to be protected")
	}
}

func TestDirtyWorktreeIsReportedAndRefused(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	worktree, err := CreateWorktree(ns, "feature/login", CreateOptions{})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree.Path, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("failed to dirty the worktree: %v", err)
	}

	worktrees, err := ListWorktreesWithStatus(ns)
	if err != nil {
		t.Fatalf("ListWorktreesWithStatus failed: %v", err)
	}
	if got := findWorktree(t, worktrees, "feature/login"); !got.Dirty || !strings.Contains(got.Status, "dirty") {
		t.Errorf("expected the worktree to be reported dirty, got %+v", got)
	}

	if err := RemoveWorktree(ns, "feature/login", RemoveOptions{}); err == nil {
		t.Error("expected a dirty worktree to be refused without force")
	}
}

func TestPruneReportsMissingWorktreesBeforeRemovingThem(t *testing.T) {
	root := newEntryPoint(t, "api")
	ns := mustFindNamespace(t, root, "api")

	worktree, err := CreateWorktree(ns, "feature/login", CreateOptions{})
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if err := os.RemoveAll(worktree.Path); err != nil {
		t.Fatalf("failed to delete the worktree directory: %v", err)
	}

	stale, err := PruneWorktrees(ns, false)
	if err != nil {
		t.Fatalf("PruneWorktrees failed: %v", err)
	}
	if len(stale) != 1 || stale[0].Branch != "feature/login" || stale[0].Status != "missing" {
		t.Fatalf("expected the deleted worktree to be reported as missing, got %+v", stale)
	}

	if after, err := ListWorktrees(ns); err != nil || len(after) != 2 {
		t.Fatalf("expected a dry run to leave the record in place, got %v (%v)", branches(after), err)
	}

	if _, err := PruneWorktrees(ns, true); err != nil {
		t.Fatalf("PruneWorktrees failed: %v", err)
	}

	after, err := ListWorktrees(ns)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(after) != 1 {
		t.Errorf("expected only the trunk to remain, got %v", branches(after))
	}
}

func TestMasterIsAcceptedAsTheTrunk(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "legacy", "master")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("failed to create %q: %v", base, err)
	}

	run(t, base, "init", "--initial-branch=master")
	run(t, base, "config", "user.email", "worktree@example.com")
	run(t, base, "config", "user.name", "Worktree Test")
	run(t, base, "commit", "--allow-empty", "-m", "initial commit")

	ns := mustFindNamespace(t, root, "legacy")
	if ns.Base != "master" {
		t.Fatalf("expected master to be the trunk, got %q", ns.Base)
	}

	if _, err := CreateWorktree(ns, "feature/login", CreateOptions{}); err != nil {
		t.Fatalf("CreateWorktree failed on a master based namespace: %v", err)
	}
}

func TestSplitBranch(t *testing.T) {
	cases := []struct {
		branch string
		typ    string
		name   string
	}{
		{"feature/login", "feature", "login"},
		{"feature/auth/login", "feature", "auth/login"},
		{"main", BaseType, "main"},
		{"master", BaseType, "master"},
		{"spike", "", "spike"},
	}

	for _, c := range cases {
		typ, name := SplitBranch(c.branch)
		if typ != c.typ || name != c.name {
			t.Errorf("SplitBranch(%q) = (%q, %q), want (%q, %q)", c.branch, typ, name, c.typ, c.name)
		}
	}
}

func TestJoinBranch(t *testing.T) {
	cases := []struct{ typ, name, want string }{
		{"feature", "login", "feature/login"},
		{"feature", "feature/login", "feature/login"},
		{"", "feature/login", "feature/login"},
		{"hotfix/", "/crash", "hotfix/crash"},
		{"feature", "", ""},
	}

	for _, c := range cases {
		if got := JoinBranch(c.typ, c.name); got != c.want {
			t.Errorf("JoinBranch(%q, %q) = %q, want %q", c.typ, c.name, got, c.want)
		}
	}
}

func TestParseTarget(t *testing.T) {
	cases := []struct {
		ref, flag  string
		ns, branch string
		wantErr    bool
	}{
		{ref: "api/feature/login", ns: "api", branch: "feature/login"},
		{ref: "feature/login", flag: "api", ns: "api", branch: "feature/login"},
		{ref: "login", flag: "api", ns: "api", branch: "login"},
		{ref: "login", wantErr: true},
		{ref: "", wantErr: true},
	}

	for _, c := range cases {
		ns, branch, err := ParseTarget(c.ref, c.flag)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseTarget(%q, %q) should have failed", c.ref, c.flag)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTarget(%q, %q) failed: %v", c.ref, c.flag, err)
		}
		if ns != c.ns || branch != c.branch {
			t.Errorf("ParseTarget(%q, %q) = (%q, %q), want (%q, %q)", c.ref, c.flag, ns, branch, c.ns, c.branch)
		}
	}
}

func TestGitReportsTheCommandThatFailed(t *testing.T) {
	if _, err := git(t.TempDir(), "status"); err == nil || !strings.Contains(err.Error(), "git status") {
		t.Errorf("expected the failing command in the error, got %v", err)
	}
}

// TestMain keeps git from reading the developer's own configuration, so the
// tests behave the same everywhere.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "worktree-git-home")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(home)

	for key, value := range map[string]string{
		"HOME":              home,
		"XDG_CONFIG_HOME":   filepath.Join(home, ".config"),
		"GIT_CONFIG_GLOBAL": filepath.Join(home, ".gitconfig"),
		"GIT_CONFIG_SYSTEM": filepath.Join(home, ".gitconfig-system"),
	} {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}

	if _, err := exec.LookPath("git"); err != nil {
		panic("git is required to run these tests: " + err.Error())
	}

	os.Exit(m.Run())
}
