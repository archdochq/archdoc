package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ollieread/archdoc/internal/lifecycle"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/template"
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

			kind, title, err := askForNew(cmd, args)
			if err != nil {
				return err
			}
			t, ok := repo.ParseType(kind)
			if !ok {
				return fmt.Errorf("%q is not a document type; use rfc, adr or ref", kind)
			}
			// One check rather than a list of forbidden characters: render the
			// heading the document will carry, read it back, and refuse if it
			// is not what was asked for. A line break, a trailing hash and
			// trailing whitespace all fail it, and so will whatever markdown
			// syntax is discovered next.
			if strings.ContainsAny(title, "\r\n") {
				return fmt.Errorf("a title cannot contain a line break")
			}
			if !repo.HeadingSurvives(title) {
				return fmt.Errorf("%q is not usable as a title: markdown would read the heading back as something else", title)
			}
			if opts.On && t == repo.TypeRef {
				return fmt.Errorf("a ref has no status, so it cannot be backfilled")
			}
			if !opts.On && (opts.Status != "" || opts.Created != "" || opts.Decided != "") {
				return fmt.Errorf("--status, --created and --decided only apply with --backfill")
			}

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
	cmd.Flags().StringVar(&opts.Status, "status", "", "with --backfill, the terminal status: accepted, rejected or withdrawn")
	cmd.Flags().StringVar(&opts.Created, "created", "", "with --backfill, the date the work began")
	cmd.Flags().StringVar(&opts.Decided, "decided", "", "with --backfill, the date the decision was taken")
	return cmd
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

	status := opts.Status
	if status == "" {
		status = string(repo.StatusAccepted)
	}
	if !repo.Status(status).Terminal() {
		return template.Data{}, fmt.Errorf("--status %q is not terminal; backfilling records a decision already taken", status)
	}

	created, decided := opts.Created, opts.Decided
	if created == "" {
		created = today
	}
	if decided == "" {
		decided = created
	}

	// Two variables rather than a map keyed on the flag names: a mistyped key
	// there yields a zero time.Time rather than a compile error, and ranging a
	// map would report whichever flag the runtime reached first.
	createdOn, err := time.Parse(repo.DateLayout, created)
	if err != nil {
		return template.Data{}, fmt.Errorf("--created %q is not an ISO 8601 date", created)
	}
	decidedOn, err := time.Parse(repo.DateLayout, decided)
	if err != nil {
		return template.Data{}, fmt.Errorf("--decided %q is not an ISO 8601 date", decided)
	}
	if decidedOn.Before(createdOn) {
		return template.Data{}, fmt.Errorf("--decided %s is before --created %s", decided, created)
	}
	// Compared as dates. Against a wall-clock instant this refused today's date
	// for the first hours of every local day east of UTC, and accepted
	// tomorrow's west of it.
	if decidedOn.After(repo.StartOfDay(now)) {
		return template.Data{}, fmt.Errorf("--decided %s is in the future, so it cannot already have been recorded", decided)
	}

	return template.Data{Date: created, Status: status, Decided: decided, Backfilled: today}, nil
}
