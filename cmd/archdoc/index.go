package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"archdoc.dev/internal/index"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
	"github.com/spf13/cobra"
)

// indexFile is the generated index, written under root.
const indexFile = "INDEX.md"

func newIndexCommand() *cobra.Command {
	var check, asJSON bool

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Regenerate INDEX.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			generated := index.Generate(r)
			path := filepath.Join(r.Config().RootDir(), indexFile)

			if !check {
				if err := repo.WriteFile(path, generated, indexFile); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), indexFile)
				return nil
			}

			// A missing index differs from an out-of-date one only in the
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
					Path: indexFile, Severity: lint.Error, Rule: "index",
					Message: "out of date; run archdoc index",
				}}, true)
			} else if diff := unifiedDiff(indexFile, string(committed), string(generated)); diff != "" {
				fmt.Fprint(cmd.OutOrStdout(), diff)
			} else {
				// The lines match, so the difference is the trailing newline.
				// Exiting 2 in silence is what success looks like.
				fmt.Fprintf(cmd.OutOrStdout(), "%s: out of date; it differs only in the final newline\n", indexFile)
			}
			return silentExit(exitFindings)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "fail if the committed index is out of date")
	cmd.Flags().BoolVar(&asJSON, "json", false, "with --check, report the difference as a JSON array")
	return cmd
}
