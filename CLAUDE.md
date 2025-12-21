# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Worktree is a Go-based interactive terminal UI application for managing multiple Git repositories using Git's native worktree feature. It allows developers to discover repositories, create worktrees (isolated branches with separate working directories), and open them in editors.

## Build Commands

```bash
# Build the application
go build -o worktree

# Run with custom entry point
WT_ENTRY_POINT=/path/to/repos ./worktree
# Or use --entry-point flag
./worktree --entry-point /path/to/repos
```

## Architecture

```
main.go                    CLI entry point (urfave/cli), initializes tview app
internal/
  layout/layout.go         UI layer - tview components, layout management, user interactions
  utils/utils.go           Business logic - git operations, file system utilities, editor integration
```

**Control flow:** `main.go` → `layout.NewLayout()` → `SetupLayoutContentMenus()` → scans repos → renders UI

**UI Structure:**
- Left panel (40%): Repository list
- Right panel: Action list (top 66%) + Log view (bottom 33%)

## Key Patterns

- **Logger callbacks:** Git operations accept `func(format string, args ...interface{})` for async UI logging
- **Modal overlays:** User input collected via tview Form modals
- **Command execution:** All git commands wrapped with 60-second timeouts in `utils.go`
- **Worktree workflow:** Creates worktrees from main/master branch, copies `workspace.code-workspace` if present

## Dependencies

- `github.com/rivo/tview` - Terminal UI framework
- `github.com/gdamore/tcell/v2` - Terminal rendering
- `github.com/urfave/cli` - CLI argument parsing

## Requirements

- Repositories must be in the same parent directory
- Each repository must have a main or master branch
- Git with worktree support required
