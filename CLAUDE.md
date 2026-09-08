# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Worktree manages multiple Git repositories through Git's native worktree feature.
It has two surfaces: an interactive terminal UI, and a CLI shaped like kubectl
(`worktree get worktrees -n api -o json`) that people and agents can script.

## Build Commands

```bash
# Build the application
go build -o worktree

# Run the tests (they drive real git repositories in a temp directory)
go test ./...

# Point it at a directory of repositories
WT_ENTRY_POINT=/path/to/repos ./worktree            # terminal UI
./worktree --entry-point /path/to/repos get wt      # CLI
```

## Architecture

```
main.go                    Entry point: builds the CLI, injects the TUI runner
internal/
  cmd/app.go               CLI commands (urfave/cli): get, create, remove, path, open, prune, ui
  cmd/output.go            Output encodings: table, json, name
  core/namespace.go        Namespace (repository) discovery under the entry point
  core/worktree.go         Worktree listing, creation, removal, pruning
  core/naming.go           <type>/<name> branch convention, target parsing, validation
  core/git.go              git command runner (git -C, 60 second timeout)
  core/editor.go           Launching an editor on a worktree
  config/config.go         ~/.config/worktree/config.json (editor preferences)
  layout/layout.go         tview widgets, layout and interactions
  utils/utils.go           Git and filesystem helpers used by the TUI
```

**Control flow (CLI):** `main.go` → `cmd.New()` → command action → `core`
**Control flow (TUI):** `main.go` → `cmd` default action → `runUI()` → `layout` → `utils`

`internal/core` backs the CLI and `internal/utils` backs the TUI. They overlap,
because the CLI was added without disturbing the TUI while other work was in
flight; migrating `layout` onto `core` and retiring `utils` is the intended next
step, not a second implementation to keep in sync.

## The model

```
<entry-point>/<namespace>/<main|master>   the trunk worktree of a repository
<entry-point>/<namespace>/<type>/<name>   a worktree, e.g. api/feature/login
```

A namespace is a directory holding a `main` or `master` git worktree; anything
else under the entry point is ignored. Every worktree is addressed as
`<namespace>/<branch>`, or as `<branch>` with `--namespace`.

## Key Patterns

- **core is the only place the CLI touches git.** Do not add git invocations to
  `cmd`; add them to `core` and call them.
- **No working directory changes in core.** Every git call is `git -C <dir>`, so
  operations are independent and safe to call from anywhere. (`utils`, which the
  TUI uses, still navigates with `os.Chdir`.)
- **stdout is data, stderr is chatter.** CLI results go to `c.App.Writer`,
  progress and diagnostics to `warn()`, failures through `fail()` so the process
  exits non-zero.
- **Destructive operations check first.** Removal refuses dirty or unmerged
  worktrees without `--force` and leaves them untouched; `prune` is a dry run
  unless `--yes` is given; the trunk worktree is never removed.
- **Branch names are also directory names,** so `core.ValidateBranch` rejects
  anything that would escape the namespace.

## Dependencies

- `github.com/rivo/tview` - Terminal UI framework
- `github.com/gdamore/tcell/v2` - Terminal rendering
- `github.com/urfave/cli` - CLI argument parsing (v1)

## Requirements

- Repositories must be in the same parent directory (the entry point)
- Each repository must have a main or master worktree
- Git with worktree support required
