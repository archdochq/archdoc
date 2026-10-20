package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"archdoc.dev/internal/changes"
	"github.com/spf13/cobra"
)

func newChangesCommand() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Summarise what the working tree changes in the record against the configured branch",
		Long: "Lists each document that arrives, changes status, is edited or is removed, compared with the " +
			"configured branch, as Markdown for a job summary or a pull request comment, or as JSON. " +
			"A rename is a removal and an arrival. Exits 0 whatever it finds: lint --merge judges.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			g, err := openGit(r)
			if err != nil {
				return err
			}
			base, err := changes.ReadBase(r, g)
			if err != nil {
				return err
			}
			list, err := changes.Of(r, g, base)
			if err != nil {
				return err
			}
			if asJSON {
				return printChangesJSON(cmd.OutOrStdout(), base.Branch, list)
			}
			printChanges(cmd.OutOrStdout(), base.Branch, list)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the changes as a JSON object")
	return cmd
}

func printChangesJSON(w io.Writer, branch string, list []changes.Change) error {
	if list == nil {
		list = []changes.Change{} // [] rather than null
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Base    string           `json:"base"`
		Changes []changes.Change `json:"changes"`
	}{branch, list})
}

func printChanges(w io.Writer, branch string, list []changes.Change) {
	fmt.Fprintln(w, "## Changes to the record")
	fmt.Fprintln(w)
	if len(list) == 0 {
		fmt.Fprintf(w, "The record is unchanged against `%s`.\n", branch)
		return
	}
	fmt.Fprintln(w, "| Document | Change |")
	fmt.Fprintln(w, "|---|---|")
	for _, c := range list {
		fmt.Fprintf(w, "| %s | %s |\n", cell(changeName(c)), cell(changeText(c, branch)))
	}
}

// changeName is a numbered document's identifier and title, or any other
// document's path.
func changeName(c changes.Change) string {
	if c.ID != "" && c.Title != "" {
		return c.ID + ": " + c.Title
	}
	if c.ID != "" {
		return c.ID
	}
	return "`" + c.Path + "`"
}

func changeText(c changes.Change, branch string) string {
	switch c.Change {
	case changes.Arrives:
		return withStatus("Arrives", string(c.To))
	case changes.Removed:
		return withStatus("Removed", string(c.From))
	case changes.Status:
		return string(c.From) + " → " + string(c.To)
	default:
		if c.Terminal != nil && *c.Terminal {
			return "Edited, terminal on `" + branch + "`"
		}
		return "Edited"
	}
}

func withStatus(text, status string) string {
	if status == "" {
		return text
	}
	return text + ", " + status
}

// cell keeps a title from ending its table cell, or its row, early.
func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}
