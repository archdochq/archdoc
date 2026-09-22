package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"archdoc.dev/internal/glossary"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
	"github.com/spf13/cobra"
)

func newTermCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "term",
		Short: "Manage the glossary",
	}
	cmd.AddCommand(termAdd(), termRename(), termRemove(), termList(), termShow())
	return cmd
}

func termAdd() *cobra.Command {
	var namedBy string
	cmd := &cobra.Command{
		Use:   "add <term> <definition>",
		Short: "Write a term under term/",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			// The arguments are settled before anything is written, so that a
			// usage error does not leave a term directory behind.
			term, definition, err := askForTerm(cmd, args, true, "add")
			if err != nil {
				return err
			}
			docPath, source, err := glossary.Add(r, term, definition, namedBy)
			if err != nil {
				return err
			}
			return writeTerm(cmd, r, "", docPath, source, fmt.Sprintf("added %q", term))
		},
	}
	cmd.Flags().StringVar(&namedBy, "named-by", "", "the document that introduced this term")
	return cmd
}

func termRename() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Retitle a term, recording the previous name",
		Long: "Retitle a term, recording the previous name.\n\n" +
			"The old name keeps an anchor in the generated glossary, so a link written before the " +
			"rename still resolves, including one inside a frozen document that could never be corrected.",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			old, replacement, err := askForRename(cmd, args, termNames(r))
			if err != nil {
				return err
			}
			was, docPath, source, err := glossary.Rename(r, old, replacement)
			if err != nil {
				return err
			}
			return writeTerm(cmd, r, was, docPath, source,
				fmt.Sprintf("renamed %q to %q", old, replacement))
		},
	}
	return cmd
}

func termRemove() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <term>",
		Short: "Delete a term",
		Long: "Delete a term.\n\n" +
			"Refuses when a frozen document links to it: the link would be left pointing at an anchor " +
			"that no longer exists, and the document holding it could never be corrected.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			g, err := openGit(r)
			if err != nil {
				return err
			}
			term, _, err := askForTerm(cmd, args, false, "remove")
			if err != nil {
				return err
			}
			// NewContext rather than a literal, for the reason renumber gives:
			// without the branch snapshot every document looks editable, and
			// the guard this command exists for would never fire.
			frozen := lint.NewContext(r, g, time.Now()).Frozen
			docPath, err := glossary.Remove(r, term, frozen)
			if err != nil {
				return err
			}
			if err := os.Remove(r.File(docPath)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: removed %q\n", docPath, term)
			return nil
		},
	}
	return cmd
}

func termList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print every term, one per line",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := entries()
			if err != nil {
				return err
			}
			for _, e := range entries {
				fmt.Fprintln(cmd.OutOrStdout(), e.Term)
			}
			return nil
		},
	}
}

func termShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show <term>",
		Short: "Print one term and its definition",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			found, err := entries()
			if err != nil {
				return err
			}
			wanted, _, err := askForTerm(cmd, args, false, "show")
			if err != nil {
				return err
			}
			for _, e := range found {
				if !strings.EqualFold(e.Term, wanted) {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "## %s\n\n%s\n", e.Term, strings.Join(e.Paragraphs, "\n\n"))
				for _, name := range e.Formerly {
					fmt.Fprintf(cmd.OutOrStdout(), "\nFormerly *%s*.\n", name)
				}
				return nil
			}
			return fmt.Errorf("the glossary does not define %q", wanted)
		},
	}
}

func entries() ([]repo.GlossaryEntry, error) {
	r, err := openRepo()
	if err != nil {
		return nil, err
	}
	found, ok := r.Glossary()
	if !ok {
		return nil, fmt.Errorf("this repository has no %s directory", repo.TypeTerm)
	}
	return found, nil
}

// writeTerm writes a term file, removing the file it replaced when a rename
// moved it. The removal comes second: a failed write leaves the old file in
// place rather than losing the term entirely.
func writeTerm(cmd *cobra.Command, r *repo.Repo, was, docPath string, source []byte, what string) error {
	full := r.File(docPath)
	if err := repo.Contains(r.Config().RootDir(), full); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := r.WriteFile(docPath, source); err != nil {
		return err
	}
	if was != "" && was != docPath {
		if err := os.Remove(r.File(was)); err != nil {
			return err
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", docPath, what)
	return nil
}

// termNames lists every term, for the prompt that offers them.
func termNames(r *repo.Repo) []string {
	entries, _ := r.Glossary()
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Term)
	}
	return names
}
