// Package cmd wires the worktree command line: a small, kubectl shaped surface
// over the namespaces and worktrees discovered under the entry point, so that
// both people and agents can drive the same operations the TUI offers.
package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"worktree/internal/config"
	"worktree/internal/core"

	"github.com/urfave/cli"
)

// entryPointEnv is the environment variable holding the directory that
// contains every repository.
const entryPointEnv = "WT_ENTRY_POINT"

// New builds the command line application. runUI launches the interactive
// terminal UI, which stays the behaviour of the bare "worktree" command.
func New(version string, runUI func(entryPoint string) error) *cli.App {
	app := cli.NewApp()
	app.Name = "worktree"
	app.Usage = "Manage git worktrees across repositories"
	app.Version = version
	app.Description = strings.TrimSpace(`
Repositories live side by side under an entry point and are addressed as
namespaces, the way kubectl addresses namespaces and pods:

  <entry-point>/<namespace>/<main|master>   the trunk worktree
  <entry-point>/<namespace>/<type>/<name>   a worktree, e.g. feature/login

Every worktree is addressed as <namespace>/<branch>, or as <branch> together
with --namespace. Running worktree with no command opens the terminal UI.`)

	app.Flags = []cli.Flag{entryPointFlag()}
	app.Commands = []cli.Command{
		getCommand(),
		createCommand(),
		removeCommand(),
		pathCommand(),
		openCommand(),
		pruneCommand(),
		uiCommand(runUI),
	}

	app.Action = func(c *cli.Context) error {
		if c.NArg() > 0 {
			return fail("unknown command %q: run 'worktree help' to see the available commands", c.Args().First())
		}

		entryPoint, err := entryPoint(c)
		if err != nil {
			return fail("%v", err)
		}

		return runUI(entryPoint)
	}

	return app
}

func getCommand() cli.Command {
	return cli.Command{
		Name:  "get",
		Usage: "List namespaces, worktrees or editors",
		Subcommands: []cli.Command{
			{
				Name:      "namespaces",
				Aliases:   []string{"namespace", "ns"},
				Usage:     "List the repositories under the entry point",
				ArgsUsage: " ",
				Flags:     append(commonFlags(), outputFlag()),
				Action:    runGetNamespaces,
			},
			{
				Name:      "worktrees",
				Aliases:   []string{"worktree", "wt"},
				Usage:     "List worktrees, across every namespace or within one",
				ArgsUsage: "[namespace]",
				Flags: append(commonFlags(),
					namespaceFlag(),
					cli.StringFlag{Name: "type, t", Usage: "Only show worktrees of this branch `TYPE` (feature, hotfix, ...)"},
					outputFlag(),
				),
				Action: runGetWorktrees,
			},
			{
				Name:      "editors",
				Aliases:   []string{"editor"},
				Usage:     "List the editors configured for the open command",
				ArgsUsage: " ",
				Flags:     []cli.Flag{outputFlag(), cli.BoolFlag{Name: "all, a", Usage: "Include editors that are disabled"}},
				Action:    runGetEditors,
			},
		},
	}
}

func createCommand() cli.Command {
	return cli.Command{
		Name:      "create",
		Aliases:   []string{"add", "new"},
		Usage:     "Create a worktree for a branch",
		ArgsUsage: "<namespace>/<branch> | <branch> --namespace <namespace>",
		Description: strings.TrimSpace(`
The branch is created from the trunk of the namespace unless --from names
another ref. An existing local branch is checked out in place instead of
being recreated.

  worktree create api/feature/login
  worktree create feature/login --namespace api
  worktree create login --namespace api --type feature --from origin/main`),
		Flags: append(commonFlags(),
			namespaceFlag(),
			cli.StringFlag{Name: "type, t", Usage: "Branch `TYPE` to prefix the name with (feature, hotfix, ...)"},
			cli.StringFlag{Name: "from, f", Usage: "`REF` to branch from (default: the namespace trunk branch)"},
			cli.BoolFlag{Name: "no-workspace", Usage: "Do not copy " + core.WorkspaceFile + " from the trunk worktree"},
			outputFlag(),
		),
		Action: runCreate,
	}
}

func removeCommand() cli.Command {
	return cli.Command{
		Name:      "remove",
		Aliases:   []string{"rm", "delete", "del"},
		Usage:     "Remove a worktree and its branch",
		ArgsUsage: "<namespace>/<branch> | <branch> --namespace <namespace>",
		Description: strings.TrimSpace(`
A worktree with uncommitted changes, or whose branch is not merged into the
trunk, is refused unless --force is given. The trunk worktree is never
removed.`),
		Flags: append(commonFlags(),
			namespaceFlag(),
			cli.BoolFlag{Name: "force, f", Usage: "Remove even when the worktree is dirty or the branch is unmerged"},
			cli.BoolFlag{Name: "keep-branch", Usage: "Remove the working directory but keep the branch"},
		),
		Action: runRemove,
	}
}

func pathCommand() cli.Command {
	return cli.Command{
		Name:        "path",
		Usage:       "Print the directory of a worktree",
		ArgsUsage:   "<namespace>/<branch> | <branch> --namespace <namespace>",
		Description: "Prints nothing but the path, so it composes: cd \"$(worktree path api/feature/login)\"",
		Flags:       append(commonFlags(), namespaceFlag()),
		Action:      runPath,
	}
}

func openCommand() cli.Command {
	return cli.Command{
		Name:      "open",
		Usage:     "Open a worktree in an editor",
		ArgsUsage: "<namespace>/<branch> | <branch> --namespace <namespace>",
		Flags: append(commonFlags(),
			namespaceFlag(),
			cli.StringFlag{Name: "editor, e", Usage: "`EDITOR` to open with (default: the first enabled one)"},
		),
		Action: runOpen,
	}
}

func pruneCommand() cli.Command {
	return cli.Command{
		Name:      "prune",
		Usage:     "Report worktrees whose directory has disappeared",
		ArgsUsage: "[namespace]",
		Description: strings.TrimSpace(`
Lists the stale worktree records git still tracks after a directory was
deleted by hand. Nothing is removed unless --yes is given.`),
		Flags: append(commonFlags(),
			namespaceFlag(),
			cli.BoolFlag{Name: "yes, y", Usage: "Actually drop the stale records instead of only reporting them"},
			outputFlag(),
		),
		Action: runPrune,
	}
}

func uiCommand(runUI func(entryPoint string) error) cli.Command {
	return cli.Command{
		Name:      "ui",
		Aliases:   []string{"tui"},
		Usage:     "Open the interactive terminal UI",
		ArgsUsage: " ",
		Flags:     commonFlags(),
		Action: func(c *cli.Context) error {
			entryPoint, err := entryPoint(c)
			if err != nil {
				return fail("%v", err)
			}

			return runUI(entryPoint)
		},
	}
}

func runGetNamespaces(c *cli.Context) error {
	entryPoint, format, err := entryPointAndFormat(c)
	if err != nil {
		return err
	}

	namespaces, err := core.ListNamespaces(entryPoint)
	if err != nil {
		return fail("%v", err)
	}

	return write(c, func(w io.Writer) error { return printNamespaces(w, format, namespaces) })
}

func runGetWorktrees(c *cli.Context) error {
	entryPoint, format, err := entryPointAndFormat(c)
	if err != nil {
		return err
	}

	// The namespace may be given either as a flag or as the single argument,
	// so that "get worktrees api" reads as naturally as "get worktrees -n api".
	name, err := namespaceArg(c)
	if err != nil {
		return err
	}

	var namespaces []core.Namespace
	if name != "" {
		ns, err := core.FindNamespace(entryPoint, name)
		if err != nil {
			return fail("%v", err)
		}
		namespaces = []core.Namespace{ns}
	} else if namespaces, err = core.ListNamespaces(entryPoint); err != nil {
		return fail("%v", err)
	}

	branchType := strings.TrimSpace(c.String("type"))
	worktrees := make([]core.Worktree, 0)
	for _, ns := range namespaces {
		found, err := core.ListWorktreesWithStatus(ns)
		if err != nil {
			return fail("%v", err)
		}

		for _, worktree := range found {
			if branchType == "" || strings.EqualFold(worktree.Type, branchType) {
				worktrees = append(worktrees, worktree)
			}
		}
	}

	return write(c, func(w io.Writer) error { return printWorktrees(w, format, worktrees, name == "") })
}

func runGetEditors(c *cli.Context) error {
	format, err := ParseFormat(c.String("output"))
	if err != nil {
		return fail("%v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		warn(c, "%v", err)
	}

	editors := cfg.Enabled()
	if c.Bool("all") {
		editors = cfg.All()
	}

	return write(c, func(w io.Writer) error { return printEditors(w, format, editors) })
}

func runCreate(c *cli.Context) error {
	entryPoint, format, err := entryPointAndFormat(c)
	if err != nil {
		return err
	}

	ns, branch, err := target(c, entryPoint)
	if err != nil {
		return err
	}
	branch = core.JoinBranch(c.String("type"), branch)

	worktree, err := core.CreateWorktree(ns, branch, core.CreateOptions{
		From:            c.String("from"),
		NoCopyWorkspace: c.Bool("no-workspace"),
	})
	if err != nil {
		return fail("%v", err)
	}

	warn(c, "Created worktree %s at %s", worktreeRef(worktree), worktree.Path)

	return write(c, func(w io.Writer) error { return printWorktree(w, format, worktree) })
}

func runRemove(c *cli.Context) error {
	entryPoint, err := entryPoint(c)
	if err != nil {
		return fail("%v", err)
	}

	ns, branch, err := target(c, entryPoint)
	if err != nil {
		return err
	}

	worktree, err := core.FindWorktree(ns, branch)
	if err != nil {
		return fail("%v", err)
	}

	if err := core.RemoveWorktree(ns, worktree.Branch, core.RemoveOptions{
		Force:      c.Bool("force"),
		KeepBranch: c.Bool("keep-branch"),
	}); err != nil {
		return fail("%v", err)
	}

	warn(c, "Removed worktree %s", worktreeRef(worktree))

	return nil
}

func runPath(c *cli.Context) error {
	entryPoint, err := entryPoint(c)
	if err != nil {
		return fail("%v", err)
	}

	ns, branch, err := target(c, entryPoint)
	if err != nil {
		return err
	}

	worktree, err := core.FindWorktree(ns, branch)
	if err != nil {
		return fail("%v", err)
	}

	return write(c, func(w io.Writer) error {
		_, err := fmt.Fprintln(w, worktree.Path)
		return err
	})
}

func runOpen(c *cli.Context) error {
	entryPoint, err := entryPoint(c)
	if err != nil {
		return fail("%v", err)
	}

	ns, branch, err := target(c, entryPoint)
	if err != nil {
		return err
	}

	worktree, err := core.FindWorktree(ns, branch)
	if err != nil {
		return fail("%v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		warn(c, "%v", err)
	}

	editor, err := cfg.Find(c.String("editor"))
	if err != nil {
		return fail("%v", err)
	}

	if err := core.OpenIn(editor.Command, worktree.Path); err != nil {
		return fail("%v", err)
	}

	warn(c, "Opening %s in %s", worktreeRef(worktree), editor.DisplayName)

	return nil
}

func runPrune(c *cli.Context) error {
	entryPoint, format, err := entryPointAndFormat(c)
	if err != nil {
		return err
	}

	name, err := namespaceArg(c)
	if err != nil {
		return err
	}

	var namespaces []core.Namespace
	if name != "" {
		ns, err := core.FindNamespace(entryPoint, name)
		if err != nil {
			return fail("%v", err)
		}
		namespaces = []core.Namespace{ns}
	} else if namespaces, err = core.ListNamespaces(entryPoint); err != nil {
		return fail("%v", err)
	}

	apply := c.Bool("yes")
	stale := make([]core.Worktree, 0)
	for _, ns := range namespaces {
		found, err := core.PruneWorktrees(ns, apply)
		if err != nil {
			return fail("%v", err)
		}
		stale = append(stale, found...)
	}

	switch {
	case len(stale) == 0:
		warn(c, "Nothing to prune.")
	case apply:
		warn(c, "Pruned %d stale worktree(s).", len(stale))
	default:
		warn(c, "Dry run: %d stale worktree(s) would be pruned. Re-run with --yes to remove them.", len(stale))
	}

	return write(c, func(w io.Writer) error { return printWorktrees(w, format, stale, name == "") })
}

// namespaceArg resolves the optional namespace a listing command takes, which
// may be written as a flag or as the single argument. An empty result means
// every namespace.
func namespaceArg(c *cli.Context) (string, error) {
	flag := c.String("namespace")

	switch {
	case flag != "" && c.NArg() > 0:
		return "", fail("namespace given twice: --namespace %s and %q", flag, c.Args().First())
	case c.NArg() > 1:
		return "", fail("expected at most one namespace, got %d arguments", c.NArg())
	case flag != "":
		return flag, nil
	}

	return c.Args().First(), nil
}

// target resolves the single "<namespace>/<branch>" argument a command takes.
func target(c *cli.Context, entryPoint string) (core.Namespace, string, error) {
	if c.NArg() == 0 {
		return core.Namespace{}, "", fail("no worktree given: pass <namespace>/<branch>, or <branch> with --namespace")
	}
	if c.NArg() > 1 {
		return core.Namespace{}, "", fail("expected one worktree, got %d arguments", c.NArg())
	}

	name, branch, err := core.ParseTarget(c.Args().First(), c.String("namespace"))
	if err != nil {
		return core.Namespace{}, "", fail("%v", err)
	}

	ns, err := core.FindNamespace(entryPoint, name)
	if err != nil {
		return core.Namespace{}, "", fail("%v", err)
	}

	return ns, branch, nil
}

// entryPointFlag is the application level flag, which also reads the
// environment. The per command copy deliberately does not, so that an explicit
// "worktree --entry-point X" is never overridden by the environment.
func entryPointFlag() cli.Flag {
	return cli.StringFlag{
		Name:   "entry-point",
		Usage:  "`DIRECTORY` that holds all the repositories",
		EnvVar: entryPointEnv,
	}
}

func namespaceFlag() cli.Flag {
	return cli.StringFlag{Name: "namespace, n", Usage: "`NAMESPACE` (repository) to act on"}
}

func outputFlag() cli.Flag {
	return cli.StringFlag{Name: "output, o", Usage: "Output `FORMAT`: " + formatList(), Value: string(FormatTable)}
}

// commonFlags are repeated on every command so that the entry point can be
// given after the command as well as before it.
func commonFlags() []cli.Flag {
	return []cli.Flag{cli.StringFlag{
		Name:  "entry-point",
		Usage: "`DIRECTORY` that holds all the repositories",
	}}
}

// entryPoint resolves the entry point from the command flag, the global flag,
// the environment, or the working directory, in that order.
func entryPoint(c *cli.Context) (string, error) {
	value := c.String("entry-point")
	if value == "" {
		value = c.GlobalString("entry-point")
	}
	if value == "" {
		value = os.Getenv(entryPointEnv)
	}

	return core.ResolveEntryPoint(value)
}

func entryPointAndFormat(c *cli.Context) (string, Format, error) {
	format, err := ParseFormat(c.String("output"))
	if err != nil {
		return "", "", fail("%v", err)
	}

	path, err := entryPoint(c)
	if err != nil {
		return "", "", fail("%v", err)
	}

	return path, format, nil
}

// write sends command output to the application writer, which keeps stdout
// free of anything but the result.
func write(c *cli.Context, render func(io.Writer) error) error {
	w := c.App.Writer
	if w == nil {
		w = os.Stdout
	}

	if err := render(w); err != nil {
		return fail("%v", err)
	}

	return nil
}

// warn prints progress and diagnostics to stderr so that stdout stays
// parseable by whatever is consuming the command.
func warn(c *cli.Context, format string, args ...interface{}) {
	w := c.App.ErrWriter
	if w == nil {
		w = os.Stderr
	}

	fmt.Fprintf(w, format+"\n", args...)
}

// fail turns an error into a non zero exit, which is what an agent checks.
func fail(format string, args ...interface{}) error {
	return cli.NewExitError("Error: "+fmt.Sprintf(format, args...), 1)
}
