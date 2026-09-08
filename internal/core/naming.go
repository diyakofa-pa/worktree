package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BaseBranches are the branch (and therefore directory) names accepted as the
// trunk of a namespace. A directory only counts as a namespace when one of
// them is present.
var BaseBranches = []string{"main", "master"}

// KnownTypes are the conventional branch prefixes. They are documented and
// used for filtering, never enforced: any prefix is accepted.
var KnownTypes = []string{"feature", "bugfix", "hotfix", "release", "chore", "refactor", "docs", "test"}

// BaseType is the reported type of the trunk worktree of a namespace.
const BaseType = "base"

// SplitBranch splits a branch name into its conventional type and description,
// following the <type>/<description> convention:
//
//	feature/login  -> "feature", "login"
//	main           -> "base",    "main"
//	spike          -> "",        "spike"
//
// Only the first segment is treated as the type, so feature/auth/login yields
// the description "auth/login".
func SplitBranch(branch string) (branchType, name string) {
	if IsBaseBranch(branch) {
		return BaseType, branch
	}

	if prefix, rest, found := strings.Cut(branch, "/"); found && prefix != "" && rest != "" {
		return prefix, rest
	}

	return "", branch
}

// IsBaseBranch reports whether branch is one of the trunk branch names.
func IsBaseBranch(branch string) bool {
	for _, base := range BaseBranches {
		if branch == base {
			return true
		}
	}
	return false
}

// JoinBranch builds a branch name from a conventional type and description.
// An empty type yields the bare description.
func JoinBranch(branchType, name string) string {
	branchType = strings.Trim(branchType, "/")
	name = strings.Trim(name, "/")

	if branchType == "" || name == "" {
		return name
	}
	if strings.HasPrefix(name, branchType+"/") {
		return name
	}

	return branchType + "/" + name
}

// ValidateBranch rejects branch names that are empty, unsafe as a relative
// directory, or refused by git. It is deliberately stricter than git: the
// branch name doubles as the worktree directory under the namespace.
func ValidateBranch(branch string) error {
	switch {
	case branch == "":
		return fmt.Errorf("branch name cannot be empty")
	case strings.ContainsAny(branch, " \t\n"):
		return fmt.Errorf("branch name %q cannot contain whitespace", branch)
	case strings.HasPrefix(branch, "/"), strings.HasSuffix(branch, "/"):
		return fmt.Errorf("branch name %q cannot start or end with %q", branch, "/")
	case strings.HasPrefix(branch, "-"):
		return fmt.Errorf("branch name %q cannot start with %q", branch, "-")
	case strings.Contains(branch, "//"):
		return fmt.Errorf("branch name %q cannot contain an empty path segment", branch)
	case filepath.Clean(branch) != branch:
		return fmt.Errorf("branch name %q is not a valid relative path", branch)
	case IsBaseBranch(branch):
		return fmt.Errorf("%q is the trunk of the namespace and already has a worktree", branch)
	}

	for _, segment := range strings.Split(branch, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("branch name %q cannot contain %q segments", branch, segment)
		}
	}

	return nil
}

// ParseTarget splits a "<namespace>/<branch>" reference. When namespace is
// already known (from the --namespace flag) the reference is taken as the
// branch name verbatim, which keeps branches that share a name with a
// namespace unambiguous.
func ParseTarget(ref, namespace string) (ns, branch string, err error) {
	ref = strings.Trim(ref, "/")
	if ref == "" {
		return "", "", fmt.Errorf("no worktree given")
	}

	if namespace != "" {
		return namespace, ref, nil
	}

	ns, branch, found := strings.Cut(ref, "/")
	if !found || branch == "" {
		return "", "", fmt.Errorf("%q is missing a namespace: use <namespace>/<branch> or --namespace", ref)
	}

	return ns, branch, nil
}
