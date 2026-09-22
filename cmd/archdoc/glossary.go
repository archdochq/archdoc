package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"archdoc.dev/internal/glossary"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
	"github.com/spf13/cobra"
)

func newGlossaryCommand() *cobra.Command {
	var check, asJSON bool

	cmd := &cobra.Command{
		Use:   "glossary",
		Short: "Regenerate GLOSSARY.md",
		Long: "Regenerate GLOSSARY.md from the files under term/.\n\n" +
			"Every term's former names keep an anchor on the page, so a link written before a rename " +
			"still resolves, including one inside a frozen document that could never be corrected.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			generated := glossary.Generate(r)
			path := filepath.Join(r.Config().RootDir(), glossary.File)

			if !check {
				if err := repo.WriteFile(path, generated, glossary.File); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), glossary.File)
				return nil
			}

			// A missing glossary differs from an out-of-date one only in the
			// diff, so both are reported the same way.
			committed, err := os.ReadFile(path)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if bytes.Equal(committed, generated) {
				return nil
			}

			if asJSON {
				printFindings(cmd.OutOrStdout(), []lint.Finding{{
					Path: glossary.File, Severity: lint.Error, Rule: "glossary",
					Message: "out of date; run archdoc glossary",
				}}, true)
			} else if diff := unifiedDiff(glossary.File, string(committed), string(generated)); diff != "" {
				fmt.Fprint(cmd.OutOrStdout(), diff)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: out of date; it differs only in the final newline\n", glossary.File)
			}
			return silentExit(exitFindings)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "fail if the committed glossary is out of date")
	cmd.Flags().BoolVar(&asJSON, "json", false, "with --check, report the difference as a JSON array")
	return cmd
}
