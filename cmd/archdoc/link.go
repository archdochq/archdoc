package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ollieread/archdoc/internal/link"
	"github.com/ollieread/archdoc/internal/lint"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/spf13/cobra"
)

func newLinkCommand() *cobra.Command {
	var suggest, apply, asJSON bool

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Resolve wiki links, and optionally suggest new ones",
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

			// NewContext, not a literal: Resolve asks whether a document is
			// frozen, and that answer comes from the branch snapshot.
			changes, findings := link.Resolve(lint.NewContext(r, g, time.Now()))

			// Every document that could be resolved is written, including when
			// another could not. Resolve already excludes the failing one, and
			// the spec leaves that file unchanged, not the whole repository.
			if err := writeChanges(cmd, r, changes); err != nil {
				return err
			}

			suggested := false
			if suggest {
				// Reopened: the documents just rewritten are stale in r, and
				// suggesting against those bytes would revert the resolution.
				fresh, err := openRepo()
				if err != nil {
					return err
				}
				more, offered, err := runSuggestions(cmd, fresh, apply)
				if err != nil {
					return err
				}
				findings = append(findings, more...)
				suggested = offered
			}

			printFindings(cmd.OutOrStdout(), findings, asJSON)
			if exitFor(findings, false) != exitOK || suggested {
				return silentExit(exitFindings)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&suggest, "suggest", false, "also propose links over bare identifiers and glossary terms")
	cmd.Flags().BoolVar(&apply, "apply", false, "with --suggest, accept every suggestion without prompting")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit findings as a JSON array")
	return cmd
}

// writeChanges puts rewritten documents back on disk, in a fixed order so that
// the notices do not vary between runs, and reports every failure rather than
// stopping at the first and leaving the rest unexplained.
func writeChanges(cmd *cobra.Command, r *repo.Repo, changes link.Changes) error {
	var failed []string
	for _, path := range slices.Sorted(maps.Keys(changes)) {
		if err := r.WriteFile(path, changes[path]); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		// A progress notice is not a result, so it goes to stderr and leaves
		// stdout to the findings, which --json requires be the only thing there.
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: wiki links resolved\n", path)
	}
	if len(failed) > 0 {
		return fmt.Errorf("could not write %s", strings.Join(failed, "; "))
	}
	return nil
}

// runSuggestions applies the interaction rule to suggestions. offered reports
// that suggestions were printed rather than applied, which the spec makes a
// failure so that a repository can be held to them.
func runSuggestions(cmd *cobra.Command, r *repo.Repo, apply bool) (findings []lint.Finding, offered bool, err error) {
	suggestions := link.Suggest(r)
	if len(suggestions) == 0 {
		return nil, false, nil
	}

	switch {
	case apply:
		return nil, false, applySuggestions(cmd, r, suggestions)
	case !interactive(cmd):
		for _, s := range suggestions {
			findings = append(findings, lint.Finding{
				Path: s.Path, Line: s.Line, Severity: lint.Warning, Rule: "link",
				Message: fmt.Sprintf("%q could link to %s", s.Text, s.Replacement),
			})
		}
		return findings, true, nil
	}

	accepted, err := choose(cmd, suggestions)
	if err != nil {
		return nil, false, err
	}
	return nil, false, applySuggestions(cmd, r, accepted)
}

// choose walks the suggestions with the reader, a line at a time. Input comes
// from the command so that a test can drive the whole exchange.
func choose(cmd *cobra.Command, suggestions []link.Suggestion) ([]link.Suggestion, error) {
	out := cmd.OutOrStdout()
	input := bufio.NewReader(cmd.InOrStdin())

	var accepted []link.Suggestion
	skipped := map[string]bool{}
	all := map[string]bool{}

	for _, s := range suggestions {
		if skipped[s.Path] {
			continue
		}
		if all[s.Path] {
			accepted = append(accepted, s)
			continue
		}

		show(out, s)
		answer, err := input.ReadString('\n')
		if err != nil && answer == "" {
			return accepted, nil
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y":
			accepted = append(accepted, s)
		case "a":
			all[s.Path] = true
			accepted = append(accepted, s)
		case "s":
			skipped[s.Path] = true
		case "q":
			// Quitting keeps what was accepted so far, as specified.
			return accepted, nil
		}
	}
	return accepted, nil
}

// show prints one suggestion with the match marked out, so the reader can see
// which occurrence is in question.
func show(out io.Writer, s link.Suggestion) {
	fmt.Fprintf(out, "\n%s:%d\n  %s\n  %s%s\n  %s would become %s\n",
		s.Path, s.Line, s.LineText,
		strings.Repeat(" ", s.Col), strings.Repeat("^", len(s.Text)),
		s.Text, s.Replacement)
	fmt.Fprint(out, "[y]es [n]o [a]ll in file [s]kip file [q]uit: ")
}

// applySuggestions rewrites each document once, with the suggestions for it.
//
// A document is rebuilt from the bytes read when the repository was opened, and
// in the interactive path that reading happened before the first prompt. So the
// file is re-read here and left alone if it has moved on: the whole prompting
// session is a window in which an edit made elsewhere would otherwise be
// discarded, and discarding it silently is the worst of the options.
func applySuggestions(cmd *cobra.Command, r *repo.Repo, accepted []link.Suggestion) error {
	byPath := map[string][]link.Suggestion{}
	for _, s := range accepted {
		byPath[s.Path] = append(byPath[s.Path], s)
	}
	for _, path := range slices.Sorted(maps.Keys(byPath)) {
		d := r.ByPath(path)
		if d == nil {
			continue
		}
		current, err := os.ReadFile(r.File(path))
		if err != nil {
			return fmt.Errorf("re-reading %s: %w", path, err)
		}
		if !bytes.Equal(current, d.Source) {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s: changed since it was read, so it was left alone\n", path)
			continue
		}
		if err := r.WriteFile(path, link.Apply(path, d.Source, byPath[path])); err != nil {
			return err
		}
	}
	return nil
}
