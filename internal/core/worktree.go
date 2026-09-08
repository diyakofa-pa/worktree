package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WorkspaceFile is copied from the trunk worktree into every new worktree when
// present, so editors open the same multi-root workspace everywhere.
const WorkspaceFile = "workspace.code-workspace"

// Worktree is a single git worktree inside a namespace.
type Worktree struct {
	Namespace string `json:"namespace"`
	Branch    string `json:"branch"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Head      string `json:"head"`
	IsBase    bool   `json:"isBase"`
	Detached  bool   `json:"detached"`
	Locked    bool   `json:"locked"`
	Prunable  bool   `json:"prunable"`
	Missing   bool   `json:"missing"`
	Dirty     bool   `json:"dirty"`
	Merged    bool   `json:"merged"`
	Status    string `json:"status"`
}

// CreateOptions tunes how a new worktree is created.
type CreateOptions struct {
	// From is the ref the branch is cut from. Defaults to the namespace trunk
	// branch; pass something like origin/main to branch from a fetched ref.
	From string
	// NoCopyWorkspace skips copying the trunk workspace file.
	NoCopyWorkspace bool
}

// RemoveOptions tunes how a worktree is removed.
type RemoveOptions struct {
	// Force removes a worktree with uncommitted changes and deletes its branch
	// even when it holds unmerged commits.
	Force bool
	// KeepBranch removes the working directory but leaves the branch behind.
	KeepBranch bool
}

// ListWorktrees returns every worktree of a namespace, trunk included, sorted
// with the trunk first and the rest by branch name. It does not inspect
// working tree contents; use ListWorktreesWithStatus for that.
func ListWorktrees(ns Namespace) ([]Worktree, error) {
	out, err := git(ns.BasePath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees of %q: %w", ns.Name, err)
	}

	worktrees := parseWorktreeList(ns, out)
	sort.Slice(worktrees, func(i, j int) bool {
		if worktrees[i].IsBase != worktrees[j].IsBase {
			return worktrees[i].IsBase
		}
		return worktrees[i].Branch < worktrees[j].Branch
	})

	return worktrees, nil
}

// ListWorktreesWithStatus is ListWorktrees plus the working tree state of each
// entry: whether it is missing, dirty, or already merged into the trunk.
func ListWorktreesWithStatus(ns Namespace) ([]Worktree, error) {
	worktrees, err := ListWorktrees(ns)
	if err != nil {
		return nil, err
	}

	merged := mergedBranches(ns)
	for i := range worktrees {
		worktrees[i].loadStatus(merged)
	}

	return worktrees, nil
}

// FindWorktree resolves a single worktree of a namespace by branch name.
func FindWorktree(ns Namespace, branch string) (Worktree, error) {
	worktrees, err := ListWorktreesWithStatus(ns)
	if err != nil {
		return Worktree{}, err
	}

	for _, worktree := range worktrees {
		if worktree.Branch == branch || worktree.Name == branch {
			return worktree, nil
		}
	}

	return Worktree{}, fmt.Errorf("worktree %q not found in namespace %q", branch, ns.Name)
}

// CreateWorktree adds a worktree for branch under the namespace directory,
// following the <namespace>/<type>/<name> convention. An existing local branch
// is checked out in place; otherwise the branch is cut from opts.From.
func CreateWorktree(ns Namespace, branch string, opts CreateOptions) (Worktree, error) {
	if err := ValidateBranch(branch); err != nil {
		return Worktree{}, err
	}

	path, err := worktreePath(ns, branch)
	if err != nil {
		return Worktree{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return Worktree{}, fmt.Errorf("path %q already exists", path)
	}

	from := opts.From
	if from == "" {
		from = ns.Base
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Worktree{}, fmt.Errorf("failed to create parent directory for %q: %w", path, err)
	}

	if branchExists(ns, branch) {
		if opts.From != "" {
			return Worktree{}, fmt.Errorf("branch %q already exists, so it cannot be created from %q: "+
				"check it out as it is by dropping --from", branch, opts.From)
		}
		_, err = git(ns.BasePath, "worktree", "add", path, branch)
	} else {
		_, err = git(ns.BasePath, "worktree", "add", "-b", branch, path, from)
	}
	if err != nil {
		// Leave no empty <type> directory behind after a failed attempt.
		pruneEmptyParents(ns, path)
		return Worktree{}, fmt.Errorf("failed to create worktree %q: %w", branch, err)
	}

	if !opts.NoCopyWorkspace {
		if err := copyWorkspaceFile(ns, path); err != nil {
			return Worktree{}, err
		}
	}

	return FindWorktree(ns, branch)
}

// RemoveWorktree deletes the worktree of branch and, unless opts.KeepBranch is
// set, its branch. Unmerged or dirty worktrees are refused without opts.Force.
func RemoveWorktree(ns Namespace, branch string, opts RemoveOptions) error {
	worktree, err := FindWorktree(ns, branch)
	if err != nil {
		return err
	}
	if worktree.IsBase {
		return fmt.Errorf("refusing to remove %q: it is the trunk of namespace %q", worktree.Branch, ns.Name)
	}

	// The unmerged check happens before anything is deleted, so a refused
	// removal leaves the worktree exactly as it was.
	if !opts.Force && !opts.KeepBranch && !worktree.Detached && !worktree.Merged && branchExists(ns, worktree.Branch) {
		return fmt.Errorf("refusing to remove %q: its branch has commits that are not merged into %s; "+
			"re-run with --force to delete it, or --keep-branch to keep the branch", worktree.Branch, ns.Base)
	}

	args := []string{"worktree", "remove"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, worktree.Path)

	if worktree.Missing {
		// The directory is already gone; drop the stale administrative entry.
		if _, err := git(ns.BasePath, "worktree", "prune"); err != nil {
			return fmt.Errorf("failed to prune worktree %q: %w", worktree.Branch, err)
		}
	} else if _, err := git(ns.BasePath, args...); err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w", worktree.Branch, err)
	}

	pruneEmptyParents(ns, worktree.Path)

	if opts.KeepBranch || worktree.Detached || !branchExists(ns, worktree.Branch) {
		return nil
	}

	if _, err := git(ns.BasePath, "branch", "-D", worktree.Branch); err != nil {
		return fmt.Errorf("failed to delete branch %q: %w", worktree.Branch, err)
	}

	return nil
}

// PruneWorktrees reports the worktrees whose directory has disappeared and,
// when apply is set, drops their administrative entries. It is a dry run by
// default so an agent can look before it deletes.
func PruneWorktrees(ns Namespace, apply bool) ([]Worktree, error) {
	worktrees, err := ListWorktreesWithStatus(ns)
	if err != nil {
		return nil, err
	}

	var stale []Worktree
	for _, worktree := range worktrees {
		if worktree.Missing || worktree.Prunable {
			stale = append(stale, worktree)
		}
	}

	if apply && len(stale) > 0 {
		if _, err := git(ns.BasePath, "worktree", "prune"); err != nil {
			return nil, fmt.Errorf("failed to prune worktrees of %q: %w", ns.Name, err)
		}
	}

	return stale, nil
}

// parseWorktreeList reads the porcelain form of "git worktree list", whose
// records are blank line separated key/value blocks.
func parseWorktreeList(ns Namespace, out string) []Worktree {
	var worktrees []Worktree
	var current *Worktree

	flush := func() {
		if current != nil {
			current.finalise(ns)
			worktrees = append(worktrees, *current)
			current = nil
		}
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		key, value, _ := strings.Cut(line, " ")

		switch key {
		case "worktree":
			flush()
			current = &Worktree{Namespace: ns.Name, Path: filepath.Clean(value)}
		case "":
			flush()
		}

		if current == nil {
			continue
		}

		switch key {
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "detached":
			current.Detached = true
		case "locked":
			current.Locked = true
		case "prunable":
			current.Prunable = true
		}
	}
	flush()

	return worktrees
}

// finalise fills in the fields derived from the raw git output.
func (w *Worktree) finalise(ns Namespace) {
	w.IsBase = w.Path == ns.BasePath

	if w.Branch == "" {
		// A detached worktree has no branch; name it after its location so it
		// still has a stable handle.
		w.Branch = relativeName(ns, w.Path)
	}

	w.Type, w.Name = SplitBranch(w.Branch)
	if w.IsBase {
		w.Type = BaseType
	}

}

// loadStatus inspects the working tree behind a worktree.
func (w *Worktree) loadStatus(merged map[string]bool) {
	if info, err := os.Stat(w.Path); err != nil || !info.IsDir() {
		w.Missing = true
		w.Status = "missing"
		return
	}

	if out, err := git(w.Path, "status", "--porcelain"); err == nil {
		w.Dirty = out != ""
	}
	w.Merged = merged[w.Branch]

	var labels []string
	if w.IsBase {
		labels = append(labels, BaseType)
	}
	if w.Dirty {
		labels = append(labels, "dirty")
	} else if !w.IsBase {
		labels = append(labels, "clean")
	}
	if w.Merged && !w.IsBase {
		labels = append(labels, "merged")
	}
	if w.Locked {
		labels = append(labels, "locked")
	}
	if w.Detached {
		labels = append(labels, "detached")
	}

	w.Status = strings.Join(labels, ",")
}

// mergedBranches returns the branches already merged into the namespace trunk.
func mergedBranches(ns Namespace) map[string]bool {
	merged := map[string]bool{}

	out, err := git(ns.BasePath, "branch", "--merged", ns.Base, "--format=%(refname:short)")
	if err != nil {
		return merged
	}

	for _, line := range strings.Split(out, "\n") {
		if branch := strings.TrimSpace(line); branch != "" {
			merged[branch] = true
		}
	}

	return merged
}

// worktreePath maps a branch name to its directory, refusing anything that
// would escape the namespace.
func worktreePath(ns Namespace, branch string) (string, error) {
	path := filepath.Join(ns.Path, filepath.FromSlash(branch))

	rel, err := filepath.Rel(ns.Path, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("branch name %q would place the worktree outside namespace %q", branch, ns.Name)
	}

	return path, nil
}

// relativeName names a worktree by its location inside the namespace, used for
// detached worktrees that carry no branch.
func relativeName(ns Namespace, path string) string {
	if rel, err := filepath.Rel(ns.Path, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}

	return filepath.ToSlash(path)
}

func branchExists(ns Namespace, branch string) bool {
	return gitOK(ns.BasePath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
}

// pruneEmptyParents removes the directories a nested branch name left behind,
// so removing feature/login does not leave an empty feature directory.
func pruneEmptyParents(ns Namespace, path string) {
	for dir := filepath.Dir(path); strings.HasPrefix(dir, ns.Path+string(filepath.Separator)); dir = filepath.Dir(dir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

// copyWorkspaceFile copies the trunk workspace file into a new worktree when
// the repository ships one.
func copyWorkspaceFile(ns Namespace, dest string) error {
	src := filepath.Join(ns.BasePath, WorkspaceFile)
	if _, err := os.Stat(src); err != nil {
		return nil
	}

	return copyFile(src, filepath.Join(dest, WorkspaceFile))
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %q: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy %q to %q: %w", src, dst, err)
	}

	return nil
}
