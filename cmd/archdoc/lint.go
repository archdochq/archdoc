package main

import (
	"time"

	"github.com/archdochq/archdoc/internal/lint"
	"github.com/spf13/cobra"
)

func newLintCommand() *cobra.Command {
	var asJSON, strictWarnings bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Check the repository against the rules in PROCESS.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			g, err := openGit(r)
			if err != nil {
				return err
			}

			findings := lint.Run(lint.NewContext(r, g, time.Now()))
			printFindings(cmd.OutOrStdout(), findings, asJSON)

			if code := exitFor(findings, strictWarnings); code != exitOK {
				return silentExit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit findings as a JSON array")
	cmd.Flags().BoolVar(&strictWarnings, "strict-warnings", false, "treat warnings as failures")
	return cmd
}
