package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Namespace is a repository managed under the entry point. It borrows the
// kubectl vocabulary: the entry point holds namespaces, a namespace holds
// worktrees.
//
//	<entry-point>/<namespace>/<main|master>      the trunk worktree
//	<entry-point>/<namespace>/<type>/<name>      a worktree, e.g. feature/login
type Namespace struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Base      string `json:"base"`
	BasePath  string `json:"basePath"`
	Worktrees int    `json:"worktrees"`
}

// ResolveEntryPoint turns the configured entry point into an absolute path and
// checks that it is a readable directory.
func ResolveEntryPoint(entryPoint string) (string, error) {
	if entryPoint == "" {
		entryPoint = "."
	}

	abs, err := filepath.Abs(entryPoint)
	if err != nil {
		return "", fmt.Errorf("failed to resolve entry point %q: %w", entryPoint, err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("failed to read entry point %q: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("entry point %q is not a directory", abs)
	}

	return abs, nil
}

// ListNamespaces returns every repository directly under the entry point that
// follows the convention, sorted by name. Directories without a main or master
// worktree are ignored rather than reported as errors.
func ListNamespaces(entryPoint string) ([]Namespace, error) {
	root, err := ResolveEntryPoint(entryPoint)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to list entry point %q: %w", root, err)
	}

	var namespaces []Namespace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ns, err := newNamespace(root, entry.Name())
		if err != nil {
			continue
		}

		if worktrees, err := ListWorktrees(ns); err == nil {
			ns.Worktrees = len(worktrees)
		}

		namespaces = append(namespaces, ns)
	}

	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].Name < namespaces[j].Name })

	return namespaces, nil
}

// FindNamespace resolves a single namespace by name under the entry point.
func FindNamespace(entryPoint, name string) (Namespace, error) {
	root, err := ResolveEntryPoint(entryPoint)
	if err != nil {
		return Namespace{}, err
	}

	if name == "" {
		return Namespace{}, fmt.Errorf("no namespace given")
	}
	if name != filepath.Base(name) {
		return Namespace{}, fmt.Errorf("namespace %q must be a single directory name", name)
	}

	ns, err := newNamespace(root, name)
	if err != nil {
		return Namespace{}, err
	}

	if worktrees, err := ListWorktrees(ns); err == nil {
		ns.Worktrees = len(worktrees)
	}

	return ns, nil
}

// newNamespace builds a namespace from a directory name, verifying that it
// holds a main or master git worktree.
func newNamespace(root, name string) (Namespace, error) {
	path := filepath.Join(root, name)

	for _, base := range BaseBranches {
		basePath := filepath.Join(path, base)
		if !isGitWorktree(basePath) {
			continue
		}

		return Namespace{
			Name:     name,
			Path:     path,
			Base:     base,
			BasePath: basePath,
		}, nil
	}

	return Namespace{}, fmt.Errorf("%q is not a namespace: no %s git worktree found in %s",
		name, joinOr(BaseBranches), path)
}

// isGitWorktree reports whether path is a git working tree. A worktree created
// by "git worktree add" carries a .git file rather than a directory, so both
// are accepted.
func isGitWorktree(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false
	}

	return true
}

func joinOr(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	}

	out := ""
	for i, value := range values {
		switch {
		case i == 0:
			out = value
		case i == len(values)-1:
			out += " or " + value
		default:
			out += ", " + value
		}
	}

	return out
}
