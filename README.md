# Worktree - Git Worktree Manager

A terminal UI application for managing multiple Git repositories using Git's native worktree feature. Create isolated branches with separate working directories and open them in your favorite IDE.

**[View Interactive Demo](https://hamid-faridafshar.github.io/worktree-landing/)**

## Preview

**Main Screen** - Repository list with actions and worktrees:
```
┌─Repositories────────────┐┌─Actions──────────────────────────┐
│                         ││                                  │
│  my-project            ▶││  ..                              │
│  another-repo           ││  Settings                        │
│  api-service            ││  Add New Worktree                │
│                         ││  ─────────────────────────────── │
│                         ││  feature/login                   │
│                         ││  bugfix/header                   │
│                         ││  refactor/api                    │
│                         │├─Logs─────────────────────────────┤
│                         ││ Changed directory to /repos/my..│
│                         ││                                  │
└─────────────────────────┘└──────────────────────────────────┘
```

**Worktree Actions** - Select a worktree to see available IDEs:
```
┌─Actions for feature/login────────────────┐
│                                          │
│  ..                                      │
│  Open Cursor                             │
│  Open VS Code                            │
│  Remove Worktree                         │
│                                          │
└──────────────────────────────────────────┘
```

**Settings Modal** - Configure which IDEs to show:
```
┌─Settings - Toggle IDEs───────────────────┐
│                                          │
│  [x] Cursor                              │
│  [x] VS Code                             │
│  [ ] Zed                                 │
│  [ ] Sublime Text                        │
│  [ ] Neovim                              │
│                                          │
│  [Save]  [Cancel]                        │
│                                          │
└──────────────────────────────────────────┘
```

**Add Worktree Modal** - Create a new worktree:
```
┌─Add New Worktree─────────────────────────┐
│                                          │
│  Enter Branch Name: feature/new-feature  │
│                                          │
│  [Add]  [To Cancel Press Esc]            │
│                                          │
└──────────────────────────────────────────┘
```

## Features

- **Multi-repository management** - Scan and manage all repositories in a directory
- **Worktree support** - Create and remove Git worktrees for parallel branch development
- **Configurable IDE support** - Choose from Cursor, VS Code, Zed, Sublime Text, or Neovim
- **Persistent settings** - IDE preferences saved to `~/.config/worktree/config.json`
- **Interactive terminal UI** - Keyboard-driven navigation

## Installation

### Prerequisites

- Go 1.19 or later
- Git with worktree support

```sh
go version
```

### Build

```sh
git clone https://github.com/Hamid-Faridafshar/worktree.git
cd worktree
go build -o worktree
```

### Run

```sh
# Using environment variable
export WT_ENTRY_POINT=/path/to/your/repositories
./worktree

# Or using command-line flag
./worktree --entry-point /path/to/your/repositories
```

## Repository Structure

The application expects repositories to be organized in a parent directory, each with a `main` or `master` branch:

```
my-repositories/
├── project-1/
│   └── main/           # Required: main or master branch
│       └── ...
├── project-2/
│   └── master/         # Required: main or master branch
│       └── ...
└── not-a-repo/         # Ignored: no main/master directory
    └── random-files/
```

## Configuration

IDE preferences are stored in `~/.config/worktree/config.json`:

```json
{
  "editors": {
    "cursor": {
      "enabled": true,
      "command": "cursor",
      "displayName": "Cursor"
    },
    "vscode": {
      "enabled": true,
      "command": "code",
      "displayName": "VS Code"
    },
    "zed": {
      "enabled": false,
      "command": "zed",
      "displayName": "Zed"
    },
    "sublime": {
      "enabled": false,
      "command": "subl",
      "displayName": "Sublime Text"
    },
    "neovim": {
      "enabled": false,
      "command": "nvim",
      "displayName": "Neovim"
    }
  }
}
```

On first run, a default config is created with Cursor and VS Code enabled.

## Usage

### Workflow

1. **Launch** the app - see your repositories in the left panel
2. **Select a repository** - view its worktrees and actions in the right panel
3. **Open Settings** - toggle which IDEs appear in the menu
4. **Add New Worktree** - create a new branch with its own working directory
5. **Select a worktree** - choose an IDE to open the code

### Keyboard Navigation

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate lists |
| `Enter` | Select item |
| `Esc` | Cancel / Go back |
| `Tab` | Move between form fields |

## Workspace Support

If a `workspace.code-workspace` file exists in the main branch, it will be:
- Automatically copied to new worktrees
- Used when opening the worktree in VS Code or Cursor

## CLI

The same repositories can be driven from the command line, which is what makes
the tool scriptable and usable by coding agents. Repositories under the entry
point are addressed as **namespaces**, the way `kubectl` addresses namespaces
and pods, and worktrees follow the `<type>/<name>` branch convention:

```
<entry-point>/<namespace>/<main|master>   the trunk worktree
<entry-point>/<namespace>/<type>/<name>   a worktree, e.g. api/feature/login
```

Every worktree therefore has one handle: `<namespace>/<branch>`.

```
worktree get namespaces                 List the repositories under the entry point
worktree get worktrees [namespace]      List worktrees, everywhere or in one namespace
worktree get editors                    List the editors the open command can use
worktree create <namespace>/<branch>    Create a worktree for a branch
worktree remove <namespace>/<branch>    Remove a worktree and its branch
worktree path <namespace>/<branch>      Print the directory of a worktree
worktree open <namespace>/<branch>      Open a worktree in an editor
worktree prune [namespace]              Report worktrees whose directory has disappeared
worktree ui                             Open the interactive terminal UI
```

`namespaces` is aliased to `ns`, `worktrees` to `wt`, `create` to `add`, and
`remove` to `rm` and `delete`. Running `worktree` with no command opens the UI,
as before. Every worktree argument can be written either as
`<namespace>/<branch>` or as `<branch>` with `--namespace`:

```sh
worktree create api/feature/login
worktree create feature/login --namespace api
worktree create login --namespace api --type feature
```

### Listing

```sh
$ worktree get namespaces
NAMESPACE   BASE   WORKTREES   PATH
api         main   3           /repos/api
web         main   2           /repos/web

$ worktree get worktrees --namespace api
BRANCH          TYPE      STATUS         HEAD      PATH
main            base      base           1b38464   /repos/api/main
feature/login   feature   dirty          9c21ade   /repos/api/feature/login
hotfix/crash    hotfix    clean,merged   1b38464   /repos/api/hotfix/crash
```

`STATUS` summarises what the worktree is: `base` for the trunk, `dirty` when it
has uncommitted changes, `merged` once its branch is contained in the trunk,
`missing` when its directory was deleted behind git's back, plus `locked` and
`detached` where they apply. Filter with `--type feature`.

### Creating and removing

```sh
worktree create api/feature/login                    # branch off the trunk
worktree create api/feature/login --from origin/main # branch off any ref
worktree remove api/feature/login                    # remove worktree and branch
worktree remove api/feature/login --keep-branch      # remove the directory only
```

Creation checks out an existing local branch instead of recreating it, and
copies `workspace.code-workspace` from the trunk when the repository has one
(`--no-workspace` opts out).

Removal is deliberately careful: a worktree with uncommitted changes, or whose
branch is not merged into the trunk, is refused unless you pass `--force`, and
nothing is deleted when it is refused. The trunk worktree is never removed.

### Output formats

Every listing command takes `-o table` (default), `-o json`, or `-o name`.
Results go to stdout and progress messages to stderr, so output stays
parseable:

```sh
cd "$(worktree path api/feature/login)"
worktree get worktrees -o name           # api/main, api/feature/login, ...
worktree get worktrees -o json | jq '.[] | select(.dirty)'
```

### Using it from an agent

Every command is inline, exits non-zero on failure, and can emit JSON, so an
agent can discover, create, work and clean up without shelling out to git:

```sh
worktree get namespaces -o json                            # what is available
worktree create api/feature/login -o json | jq -r .path    # a worktree of its own
worktree get worktrees -o json | jq '.[] | select(.merged and (.isBase | not))'
worktree remove api/feature/login                          # once the branch landed
worktree prune                                             # report stale records; --yes to drop
```

Each worktree in JSON carries `namespace`, `branch`, `type`, `name`, `path`,
`head`, `isBase`, `detached`, `locked`, `prunable`, `missing`, `dirty`,
`merged` and `status`.

## License

This project is licensed under the MIT License.
