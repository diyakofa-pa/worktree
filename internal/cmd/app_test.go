package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"worktree/internal/core"

	"github.com/urfave/cli"
)

// runCLI runs the command line as a user would, returning stdout, stderr and
// the error the application exited with.
func runCLI(t *testing.T, root string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	app := New("test", func(entryPoint string) error {
		_, err := out.WriteString("ui:" + entryPoint + "\n")
		return err
	})
	app.Writer = &out
	app.ErrWriter = &errOut
	cli.ErrWriter = &errOut

	full := append([]string{"worktree", "--entry-point", root}, args...)
	err = app.Run(full)

	return out.String(), errOut.String(), err
}

func mustRunCLI(t *testing.T, root string, args ...string) string {
	t.Helper()

	stdout, stderr, err := runCLI(t, root, args...)
	if err != nil {
		t.Fatalf("worktree %s failed: %v\n%s", strings.Join(args, " "), err, stderr)
	}

	return stdout
}

// newEntryPoint creates an entry point holding one namespace per name.
func newEntryPoint(t *testing.T, names ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range names {
		base := filepath.Join(root, name, "main")
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", base, err)
		}

		for _, args := range [][]string{
			{"init", "--initial-branch=main"},
			{"config", "user.email", "worktree@example.com"},
			{"config", "user.name", "Worktree Test"},
			{"commit", "--allow-empty", "-m", "initial commit"},
		} {
			cmd := exec.Command("git", append([]string{"-C", base}, args...)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
	}

	return root
}

func TestGetNamespaces(t *testing.T) {
	root := newEntryPoint(t, "api", "web")

	var namespaces []core.Namespace
	if err := json.Unmarshal([]byte(mustRunCLI(t, root, "get", "ns", "-o", "json")), &namespaces); err != nil {
		t.Fatalf("failed to decode the namespace listing: %v", err)
	}
	if len(namespaces) != 2 || namespaces[0].Name != "api" {
		t.Fatalf("unexpected namespaces %+v", namespaces)
	}

	if got := mustRunCLI(t, root, "get", "namespaces", "-o", "name"); got != "api\nweb\n" {
		t.Errorf("expected the bare names, got %q", got)
	}

	table := mustRunCLI(t, root, "get", "ns")
	if !strings.Contains(table, "NAMESPACE") || !strings.Contains(table, "api") {
		t.Errorf("expected a table with a header, got %q", table)
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	root := newEntryPoint(t, "api")

	if got := mustRunCLI(t, root, "create", "api/feature/login", "-o", "name"); got != "api/feature/login\n" {
		t.Fatalf("expected the created worktree to be named, got %q", got)
	}

	var worktrees []core.Worktree
	if err := json.Unmarshal([]byte(mustRunCLI(t, root, "get", "wt", "-n", "api", "-o", "json")), &worktrees); err != nil {
		t.Fatalf("failed to decode the worktree listing: %v", err)
	}
	if len(worktrees) != 2 {
		t.Fatalf("expected the trunk and one worktree, got %+v", worktrees)
	}
	if worktrees[1].Type != "feature" || worktrees[1].Name != "login" || worktrees[1].Status != "clean,merged" {
		t.Errorf("unexpected worktree %+v", worktrees[1])
	}

	path := strings.TrimSpace(mustRunCLI(t, root, "path", "api/feature/login"))
	if path != filepath.Join(root, "api", "feature", "login") {
		t.Fatalf("unexpected path %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected the worktree on disk: %v", err)
	}

	mustRunCLI(t, root, "remove", "feature/login", "-n", "api")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected the worktree directory to be gone")
	}

	if got := mustRunCLI(t, root, "get", "worktrees", "api", "-o", "name"); got != "api/main\n" {
		t.Errorf("expected only the trunk to remain, got %q", got)
	}
}

func TestCreateWithTypeFlagAndFilter(t *testing.T) {
	root := newEntryPoint(t, "api")

	mustRunCLI(t, root, "create", "login", "-n", "api", "--type", "feature")
	mustRunCLI(t, root, "create", "crash", "-n", "api", "--type", "hotfix")

	if got := mustRunCLI(t, root, "get", "wt", "-n", "api", "-t", "feature", "-o", "name"); got != "api/feature/login\n" {
		t.Errorf("expected only the feature worktree, got %q", got)
	}
	if got := mustRunCLI(t, root, "get", "wt", "-o", "name"); !strings.Contains(got, "api/hotfix/crash") {
		t.Errorf("expected every namespace to be listed, got %q", got)
	}
}

func TestGetWorktreesAcrossNamespacesShowsTheNamespaceColumn(t *testing.T) {
	root := newEntryPoint(t, "api", "web")

	all := mustRunCLI(t, root, "get", "wt")
	if !strings.HasPrefix(all, "NAMESPACE") {
		t.Errorf("expected a namespace column when listing everything, got %q", all)
	}

	one := mustRunCLI(t, root, "get", "wt", "-n", "api")
	if strings.HasPrefix(one, "NAMESPACE") {
		t.Errorf("expected no namespace column inside one namespace, got %q", one)
	}
}

func TestPruneIsADryRunByDefault(t *testing.T) {
	root := newEntryPoint(t, "api")
	mustRunCLI(t, root, "create", "api/feature/login")

	if err := os.RemoveAll(filepath.Join(root, "api", "feature", "login")); err != nil {
		t.Fatalf("failed to delete the worktree directory: %v", err)
	}

	stdout, stderr, err := runCLI(t, root, "prune", "-n", "api", "-o", "name")
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if !strings.Contains(stdout, "api/feature/login") {
		t.Errorf("expected the stale worktree to be reported, got %q", stdout)
	}
	if !strings.Contains(stderr, "Dry run") {
		t.Errorf("expected a dry run notice on stderr, got %q", stderr)
	}

	if got := mustRunCLI(t, root, "get", "wt", "-n", "api", "-o", "name"); !strings.Contains(got, "feature/login") {
		t.Errorf("expected the dry run to change nothing, got %q", got)
	}

	mustRunCLI(t, root, "prune", "-n", "api", "--yes")
	if got := mustRunCLI(t, root, "get", "wt", "-n", "api", "-o", "name"); strings.Contains(got, "feature/login") {
		t.Errorf("expected the stale record to be pruned, got %q", got)
	}
}

func TestFailuresExitNonZero(t *testing.T) {
	root := newEntryPoint(t, "api")

	cases := [][]string{
		{"get", "wt", "-n", "missing"},
		{"get", "ns", "-o", "yaml"},
		{"create"},
		{"create", "login"},
		{"create", "api/feature/login", "extra"},
		{"path", "api/feature/missing"},
		{"remove", "api/main"},
		{"nonsense"},
		{"get", "wt", "-n", "api", "web"},
		{"get", "wt", "api", "web"},
	}

	for _, args := range cases {
		if _, _, err := runCLI(t, root, args...); err == nil {
			t.Errorf("expected 'worktree %s' to fail", strings.Join(args, " "))
		}
	}
}

func TestBareCommandOpensTheUI(t *testing.T) {
	root := newEntryPoint(t, "api")

	if got := mustRunCLI(t, root); got != "ui:"+root+"\n" {
		t.Errorf("expected the UI to be opened over the entry point, got %q", got)
	}
	if got := mustRunCLI(t, root, "ui"); got != "ui:"+root+"\n" {
		t.Errorf("expected the ui command to open the UI, got %q", got)
	}
}

func TestEntryPointComesFromTheEnvironment(t *testing.T) {
	root := newEntryPoint(t, "api")
	t.Setenv(entryPointEnv, root)

	var out bytes.Buffer
	app := New("test", func(string) error { return nil })
	app.Writer = &out

	if err := app.Run([]string{"worktree", "get", "ns", "-o", "name"}); err != nil {
		t.Fatalf("get namespaces failed: %v", err)
	}
	if got := out.String(); got != "api\n" {
		t.Errorf("expected the entry point to come from %s, got %q", entryPointEnv, got)
	}

	// An explicit flag outranks the environment.
	other := newEntryPoint(t, "web")
	if got := mustRunCLI(t, other, "get", "ns", "-o", "name"); got != "web\n" {
		t.Errorf("expected --entry-point to win over %s, got %q", entryPointEnv, got)
	}
}

func TestGetEditorsListsTheConfiguration(t *testing.T) {
	root := newEntryPoint(t, "api")

	stdout := mustRunCLI(t, root, "get", "editors", "-o", "json")
	if !strings.Contains(stdout, `"cursor"`) {
		t.Errorf("expected the default editors, got %q", stdout)
	}
	if strings.Contains(stdout, `"zed"`) {
		t.Errorf("expected disabled editors to be hidden without --all, got %q", stdout)
	}
	if all := mustRunCLI(t, root, "get", "editors", "--all", "-o", "name"); !strings.Contains(all, "zed") {
		t.Errorf("expected --all to include disabled editors, got %q", all)
	}
}

// TestMain isolates the tests from the developer's git and application
// configuration, and keeps a failing command from exiting the test binary.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "worktree-cli-home")
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

	cli.OsExiter = func(int) {}

	os.Exit(m.Run())
}
