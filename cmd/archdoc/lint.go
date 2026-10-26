package main

import (
	"time"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/lint"
	"github.com/spf13/cobra"
)

func newLintCommand() *cobra.Command {
	var asJSON, strictWarnings, merge bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Check the repository against the rules in PROCESS.md",
		Long: "Runs every L rule over the working tree.\n\n" +
			"With --merge, also runs the merge rules, M01 to M04, which judge a pull request checked out " +
			"merged into the configured branch against that branch: no draft lands, a number already taken " +
			"there is renumbered, each change of status is one the lifecycle permits from the branch, and a " +
			"document proposed there is withdrawn rather than deleted.",
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

			ctx := lint.NewContext(r, g, time.Now())
			if merge {
				base, err := changes.ReadBase(r, g)
				if err != nil {
					return err
				}
				ctx = ctx.WithBase(base)
			}
			findings := lint.Run(ctx)
			printFindings(cmd.OutOrStdout(), findings, asJSON)

			if code := exitFor(findings, strictWarnings); code != exitOK {
				return silentExit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit findings as a JSON array")
	cmd.Flags().BoolVar(&strictWarnings, "strict-warnings", false, "treat warnings as failures")
	cmd.Flags().BoolVar(&merge, "merge", false, "also judge the working tree as a pull request into the configured branch")
	return cmd
}
