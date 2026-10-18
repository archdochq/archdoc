package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"archdoc.dev/internal/lifecycle"
	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/template"
	"github.com/spf13/cobra"
)

// backfillOptions groups the flags that only mean anything together. Four
// positional strings would be transposable at the call site; named fields are
// not, which is the only help the compiler can give for parameters of one type.
type backfillOptions struct {
	On      bool
	Status  string
	Created string
	Decided string
}

func newNewCommand() *cobra.Command {
	var opts backfillOptions

	cmd := &cobra.Command{
		Use:   "new <rfc|adr|ref> <title>",
		Short: "Create the next-numbered document of a type",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := openRepo()
			if err != nil {
				return err
			}

			// What the command line gave is checked before anything is prompted
			// for. A prompted type is one of those offered, and never a ref
			// under --backfill.
			if len(args) > 0 {
				t, ok := repo.ParseType(args[0])
				if !ok {
					return fmt.Errorf("%q is not a document type; use rfc, adr or ref", args[0])
				}
				if opts.On && t == repo.TypeRef {
					return fmt.Errorf("a ref has no status, so it cannot be backfilled")
				}
			}
			if len(args) > 1 {
				if err := checkTitle(args[1]); err != nil {
					return err
				}
			}
			if !opts.On && (opts.Status != "" || opts.Created != "" || opts.Decided != "") {
				return fmt.Errorf("--status, --created and --decided only apply with --backfill")
			}
			if err := checkBackfill(opts, time.Now()); err != nil {
				return err
			}

			kind, title, err := askForNew(cmd, args, &opts)
			if err != nil {
				return err
			}
			t, _ := repo.ParseType(kind)
			data, err := newData(opts, time.Now())
			if err != nil {
				return err
			}
			id, path, err := r.Next(t, title)
			if err != nil {
				return err
			}
			data.ID, data.Title = id, title

			rendered, err := template.Render(string(t)+".md", data)
			if err != nil {
				return err
			}
			if repo.Status(data.Status) == repo.StatusRejected {
				// The document is created already rejected, so it needs the
				// section a rejection would have added. Without it, a documented
				// flag combination wrote a document its own linter reported as
				// missing a required section.
				rendered = lifecycle.AppendRationale(rendered)
			}
			if err := repo.Contains(r.Config().RootDir(), r.File(path)); err != nil {
				return err
			}
			if err := writeNew(r.File(path), rendered); err != nil {
				if errors.Is(err, fs.ErrExist) {
					return fmt.Errorf("%s already exists", path)
				}
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.On, "backfill", false, "create the document already terminal, recording a decision taken earlier")
	cmd.Flags().StringVar(&opts.Status, "status", "", "with --backfill, the terminal status: accepted, rejected or withdrawn; accepted if not given")
	cmd.Flags().StringVar(&opts.Created, "created", "", "with --backfill, the date the work began; required unless prompted for")
	cmd.Flags().StringVar(&opts.Decided, "decided", "", "with --backfill, the date the decision was taken; --created if not given")
	return cmd
}

// checkTitle refuses a title the document could not carry. One check rather
// than a list of forbidden characters: render the heading the document will
// carry, read it back, and refuse if it is not what was asked for. A line
// break, a trailing hash and trailing whitespace all fail it, and so will
// whatever markdown syntax is discovered next. It judges a title given as an
// argument before anything is prompted for, and a prompted one as it is typed.
func checkTitle(title string) error {
	if strings.ContainsAny(title, "\r\n") {
		return fmt.Errorf("a title cannot contain a line break")
	}
	if !repo.HeadingSurvives(title) {
		return fmt.Errorf("%q is not usable as a title: markdown would read the heading back as something else", title)
	}
	if repo.Slug(title) == "" {
		return fmt.Errorf("%q contains nothing that can be used in a filename", title)
	}
	return nil
}

// writeNew creates a document, refusing to overwrite one. O_EXCL makes the
// kernel perform the test, so two concurrent runs cannot both believe they won.
func writeNew(full string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(contents)
	return err
}

// newData assembles the substitutions, defaulting the dates so that an ordinary
// document is created today and a backfilled one records when it was written
// separately from when it was decided. It refuses anything L07 would report,
// rather than writing a document the tool's own linter fails.
func newData(opts backfillOptions, now time.Time) (template.Data, error) {
	today := now.Format(repo.DateLayout)
	if !opts.On {
		return template.Data{Date: today, Status: string(repo.StatusDraft)}, nil
	}

	if opts.Status == "" {
		opts.Status = string(repo.StatusAccepted)
	}
	if opts.Created == "" {
		// Never today: a backfilled document records a decision taken before
		// it was written, and is frozen with whatever date it carries.
		return template.Data{}, fmt.Errorf("--backfill needs --created, the date the work began")
	}
	if opts.Decided == "" {
		opts.Decided = opts.Created
	}
	if err := checkBackfill(opts, now); err != nil {
		return template.Data{}, err
	}
	return template.Data{Date: opts.Created, Status: opts.Status, Decided: opts.Decided, Backfilled: today}, nil
}

// checkBackfill refuses whichever of the status and dates is wrong, judging
// only those set. The command line is checked with it before anything is
// prompted for, and each prompted answer as it is given, so nobody fills in a
// form only to have a flag refused after it.
func checkBackfill(opts backfillOptions, now time.Time) error {
	if opts.Status != "" && !repo.Status(opts.Status).Terminal() {
		return fmt.Errorf("--status %q is not terminal; backfilling records a decision already taken", opts.Status)
	}
	// Two variables rather than a map keyed on the flag names: a mistyped key
	// there yields a zero time.Time rather than a compile error, and ranging a
	// map would report whichever flag the runtime reached first.
	var createdOn, decidedOn time.Time
	var err error
	if opts.Created != "" {
		if createdOn, err = time.Parse(repo.DateLayout, opts.Created); err != nil {
			return fmt.Errorf("--created %q is not an ISO 8601 date", opts.Created)
		}
	}
	if opts.Decided != "" {
		if decidedOn, err = time.Parse(repo.DateLayout, opts.Decided); err != nil {
			return fmt.Errorf("--decided %q is not an ISO 8601 date", opts.Decided)
		}
	}
	// Without a created date createdOn is the zero time, which nothing precedes.
	if opts.Decided != "" && decidedOn.Before(createdOn) {
		return fmt.Errorf("--decided %s is before --created %s", opts.Decided, opts.Created)
	}
	// Compared as dates. Against a wall-clock instant this refused today's date
	// for the first hours of every local day east of UTC, and accepted
	// tomorrow's west of it.
	today := repo.StartOfDay(now)
	if createdOn.After(today) {
		return fmt.Errorf("--created %s is in the future, so the work cannot already have begun", opts.Created)
	}
	if decidedOn.After(today) {
		return fmt.Errorf("--decided %s is in the future, so it cannot already have been recorded", opts.Decided)
	}
	return nil
}
