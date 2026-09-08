package main

import (
	"fmt"
	"os"

	"worktree/internal/cmd"
	"worktree/internal/layout"

	"github.com/rivo/tview"
)

// version is reported by "worktree --version".
const version = "0.2.0"

func main() {
	app := cmd.New(version, runUI)

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runUI opens the interactive terminal UI over an entry point, which is what
// the bare "worktree" command still does.
func runUI(entryPoint string) error {
	l := layout.NewLayout(tview.NewApplication(), entryPoint)
	l.SetupLayoutContentMenus(entryPoint)

	return l.SetRoot()
}
