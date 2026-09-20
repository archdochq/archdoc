package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/archdochq/archdoc/internal/glossary"
	"github.com/archdochq/archdoc/internal/repo"
	"github.com/archdochq/archdoc/internal/template"
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
	var from []string
	cmd := &cobra.Command{
		Use:   "add <term> <definition>",
		Short: "Insert a term, in alphabetical order",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			// The arguments are settled before anything is written, so that a
			// usage error does not leave a glossary behind.
			term, definition, err := askForTerm(cmd, args, true, "add")
			if err != nil {
				return err
			}
			if r, err = ensureGlossary(r); err != nil {
				return err
			}
			if from, err = askForIncludes(cmd, from, acceptedDocuments(r)); err != nil {
				return err
			}

			source, err := glossary.Add(r, term, definition)
			if err != nil {
				return err
			}
			return writeGlossary(cmd, r, source, from, fmt.Sprintf("added %q", term))
		},
	}
	cmd.Flags().StringSliceVar(&from, "from", nil, "accepted documents this term comes from, added to the page's includes")
	return cmd
}

func termRename() *cobra.Command {
	var from []string
	cmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Retitle a term, recording the previous name",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			old, replacement, err := askForRename(cmd, args, termNames(r))
			if err != nil {
				return err
			}
			if from, err = askForIncludes(cmd, from, acceptedDocuments(r)); err != nil {
				return err
			}
			source, err := glossary.Rename(r, old, replacement)
			if err != nil {
				return err
			}
			return writeGlossary(cmd, r, source, from, fmt.Sprintf("renamed %q to %q", old, replacement))
		},
	}
	cmd.Flags().StringSliceVar(&from, "from", nil, "accepted documents this rename comes from")
	return cmd
}

func termRemove() *cobra.Command {
	var from []string
	cmd := &cobra.Command{
		Use:   "remove <term>",
		Short: "Delete a term",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}
			term, _, err := askForTerm(cmd, args, false, "remove")
			if err != nil {
				return err
			}
			// --from writes to the page's front matter, so the interaction rule
			// covers it here exactly as it does for add and rename. This was
			// the one of the three that never asked.
			if from, err = askForIncludes(cmd, from, acceptedDocuments(r)); err != nil {
				return err
			}
			source, err := glossary.Remove(r, term)
			if err != nil {
				return err
			}
			return writeGlossary(cmd, r, source, from, fmt.Sprintf("removed %q", term))
		},
	}
	cmd.Flags().StringSliceVar(&from, "from", nil, "accepted documents this removal comes from")
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
		return nil, fmt.Errorf("this repository has no spec/%s.md", repo.GlossaryPage)
	}
	return found, nil
}

// ensureGlossary creates spec/glossary.md from the template when the
// repository has none, because adding the first term is how a glossary starts.
// The repository is reopened so the new page is discovered.
//
// The kernel performs the existence test, as it does in `new`. Asking the
// in-memory index instead was wrong on any filesystem that folds case: the
// index is keyed on the page name taken verbatim from the filename, so
// spec/Glossary.md was invisible to the lookup while naming the same file on
// disk, and the write truncated a glossary the user could plainly see.
func ensureGlossary(r *repo.Repo) (*repo.Repo, error) {
	if r.ByPage(repo.GlossaryPage) != nil {
		return r, nil
	}
	rendered, err := template.Render("glossary.md", template.Data{})
	if err != nil {
		return nil, err
	}
	path := "spec/" + repo.GlossaryPage + ".md"
	full := r.File(path)
	if err := repo.Contains(r.Config().RootDir(), full); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	if err := writeNew(full, rendered); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("a glossary already exists at %s, but archdoc cannot read it as one; check the filename's case", path)
		}
		return nil, err
	}
	return openRepo()
}

// writeGlossary applies any --from identifiers to the same source, so the page
// is written once rather than twice.
func writeGlossary(cmd *cobra.Command, r *repo.Repo, source []byte, from []string, what string) error {
	if len(from) > 0 {
		extended, err := glossary.Include(r, source, from)
		if err != nil {
			return err
		}
		source = extended
	}
	page := r.ByPage(repo.GlossaryPage)
	if page == nil {
		return fmt.Errorf("this repository has no spec/%s.md", repo.GlossaryPage)
	}
	if err := r.WriteFile(page.Path, source); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", page.Path, what)
	return nil
}

// termNames is every term in the glossary, for a chooser to offer.
func termNames(r *repo.Repo) []string {
	found, ok := r.Glossary()
	if !ok {
		return nil
	}
	names := make([]string, len(found))
	for i, e := range found {
		names[i] = e.Term
	}
	return names
}

// acceptedDocuments is what a --from selection may offer.
func acceptedDocuments(r *repo.Repo) []*repo.Document {
	var accepted []*repo.Document
	for _, d := range r.Documents() {
		if d.Type.Normative() && d.FrontMatter.Status == repo.StatusAccepted {
			accepted = append(accepted, d)
		}
	}
	return accepted
}
