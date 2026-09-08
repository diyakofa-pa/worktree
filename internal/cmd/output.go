package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"worktree/internal/config"
	"worktree/internal/core"
)

// Format is an output encoding selected with --output.
type Format string

const (
	// FormatTable is the human readable column layout.
	FormatTable Format = "table"
	// FormatJSON is the machine readable form, meant for agents and scripts.
	FormatJSON Format = "json"
	// FormatName prints one identifier per line, for shell pipelines.
	FormatName Format = "name"
)

// formats lists the supported output encodings for flag help and validation.
var formats = []Format{FormatTable, FormatJSON, FormatName}

// ParseFormat validates the --output value.
func ParseFormat(value string) (Format, error) {
	if value == "" {
		return FormatTable, nil
	}

	for _, format := range formats {
		if strings.EqualFold(string(format), value) {
			return format, nil
		}
	}

	return "", fmt.Errorf("unknown output format %q: expected one of %s", value, formatList())
}

func formatList() string {
	var names []string
	for _, format := range formats {
		names = append(names, string(format))
	}

	return strings.Join(names, ", ")
}

// printNamespaces renders the namespaces of an entry point.
func printNamespaces(w io.Writer, format Format, namespaces []core.Namespace) error {
	switch format {
	case FormatJSON:
		return printJSON(w, namespaces)
	case FormatName:
		for _, ns := range namespaces {
			fmt.Fprintln(w, ns.Name)
		}
		return nil
	}

	if len(namespaces) == 0 {
		fmt.Fprintln(w, "No namespaces found.")
		return nil
	}

	return printTable(w, []string{"NAMESPACE", "BASE", "WORKTREES", "PATH"}, func(table *tabwriter.Writer) {
		for _, ns := range namespaces {
			fmt.Fprintf(table, "%s\t%s\t%d\t%s\n", ns.Name, ns.Base, ns.Worktrees, ns.Path)
		}
	})
}

// printWorktrees renders a worktree listing. The namespace column is dropped
// when every row belongs to the namespace the user already named.
func printWorktrees(w io.Writer, format Format, worktrees []core.Worktree, showNamespace bool) error {
	switch format {
	case FormatJSON:
		return printJSON(w, worktrees)
	case FormatName:
		for _, worktree := range worktrees {
			fmt.Fprintln(w, worktreeRef(worktree))
		}
		return nil
	}

	if len(worktrees) == 0 {
		fmt.Fprintln(w, "No worktrees found.")
		return nil
	}

	headers := []string{"BRANCH", "TYPE", "STATUS", "HEAD", "PATH"}
	if showNamespace {
		headers = append([]string{"NAMESPACE"}, headers...)
	}

	return printTable(w, headers, func(table *tabwriter.Writer) {
		for _, worktree := range worktrees {
			if showNamespace {
				fmt.Fprintf(table, "%s\t", worktree.Namespace)
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
				worktree.Branch, dash(worktree.Type), dash(worktree.Status), shortHead(worktree.Head), worktree.Path)
		}
	})
}

// printWorktree renders a single worktree, as returned by create.
func printWorktree(w io.Writer, format Format, worktree core.Worktree) error {
	if format == FormatJSON {
		return printJSON(w, worktree)
	}

	return printWorktrees(w, format, []core.Worktree{worktree}, false)
}

// printEditors renders the configured editors.
func printEditors(w io.Writer, format Format, editors []config.NamedEditor) error {
	switch format {
	case FormatJSON:
		return printJSON(w, editors)
	case FormatName:
		for _, editor := range editors {
			fmt.Fprintln(w, editor.Key)
		}
		return nil
	}

	if len(editors) == 0 {
		fmt.Fprintln(w, "No editors configured.")
		return nil
	}

	return printTable(w, []string{"EDITOR", "COMMAND", "ENABLED", "NAME"}, func(table *tabwriter.Writer) {
		for _, editor := range editors {
			fmt.Fprintf(table, "%s\t%s\t%t\t%s\n", editor.Key, editor.Command, editor.Enabled, editor.DisplayName)
		}
	})
}

func printJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(value)
}

func printTable(w io.Writer, headers []string, rows func(*tabwriter.Writer)) error {
	table := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(table, strings.Join(headers, "\t"))
	rows(table)

	return table.Flush()
}

// worktreeRef is the <namespace>/<branch> handle every command accepts.
func worktreeRef(worktree core.Worktree) string {
	return worktree.Namespace + "/" + worktree.Branch
}

func shortHead(head string) string {
	if len(head) > 7 {
		return head[:7]
	}

	return dash(head)
}

func dash(value string) string {
	if value == "" {
		return "-"
	}

	return value
}
