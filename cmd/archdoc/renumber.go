package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/ollieread/archdoc/internal/lint"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/spf13/cobra"
)

// canonical upper-cases an identifier and leaves a path alone.
func canonical(selector string) string {
	if strings.Contains(selector, "/") || strings.HasSuffix(strings.ToLower(selector), ".md") {
		return selector
	}
	return strings.ToUpper(selector)
}

func newRenumberCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "renumber <id|path> [new-id]",
		Short: "Change a document's number, following every reference to it",
		Long: "Rewrites the filename, the front matter id and the heading, and renames the document " +
			"in every list and link that refers to it.\n\n" +
			"With no target, the next free number for the type is taken. That is the form for " +
			"resolving a collision, where two pull requests branched from the same commit and both " +
			"claimed the same number.\n\n" +
			"Refuses when the document is frozen, or when a frozen document refers to it, because a " +
			"reference inside a frozen document could never be corrected afterwards.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			g, err := openGit(r)
			if err != nil {
				return err
			}

			var to string
			if len(args) == 2 {
				to = strings.ToUpper(args[1])
			}
			// Identifiers are canonically uppercase and typing one in lower
			// case is not a mistake worth an error message, but a path is
			// case-sensitive and upper-casing it destroys it.
			from := canonical(args[0])

			// NewContext rather than a literal: Frozen reads the branch
			// snapshot, and without it every document looks editable, which
			// would let this command make exactly the unrepairable edit it
			// exists to refuse.
			frozen := lint.NewContext(r, g, time.Now()).Frozen

			changed, err := repo.Renumber(r, from, to, frozen)
			if err != nil {
				return err
			}
			for _, c := range changed {
				fmt.Fprintln(cmd.OutOrStdout(), c)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "the index is now out of date; run archdoc index")
			return nil
		},
	}
}
