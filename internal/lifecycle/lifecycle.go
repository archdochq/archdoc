// Package lifecycle moves a document through its statuses and rewrites the
// file. Nothing here prompts, prints or commits.
package lifecycle

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"time"

	"archdoc.dev/internal/git"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
)

// rationale is appended to a document when it is rejected. Lint reports the
// section as empty until the prompt is replaced, which is the intended nudge to
// finish the document before committing it.
const rationale = "## " + repo.RejectionRationale + "\n\n<!-- Considered on its merits and turned down. Why? -->\n"

// writtenRationale stands in for the rationale when a rejection is judged. The
// prompt alone is an empty section, so a rejection judged exactly as written
// would always be refused for the section reject itself appends.
const writtenRationale = "## " + repo.RejectionRationale + "\n\nWritten by its author.\n"

// AppendRationale adds the rejection rationale section to a document, as
// rejecting one does. `archdoc new --status=rejected` creates a document that
// is already rejected, so it needs the same section and the same prompt in it;
// this is exported so that there is one of each rather than a second copy in
// the templates.
func AppendRationale(source []byte) []byte { return appendSection(source, rationale) }

// Apply moves a document to a new status and returns the rewritten file. The
// document on disk is not touched; writing it is the caller's business. g is
// the repository lint compares frozen documents against, nil outside git.
func Apply(d *repo.Document, to repo.Status, now time.Time, g git.Repository) ([]byte, error) {
	if !d.Type.HasLifecycle() {
		return nil, fmt.Errorf("a %s has no lifecycle, so it cannot become %s", d.Type, to)
	}
	strict := d.Repo().Config().Strict
	from := d.FrontMatter.Status
	if !slices.Contains(repo.Statuses, from) {
		// Asked before the graph, because the graph answers "nothing" for any
		// status it does not know, and "nothing permitted" used to be read as
		// "the document is terminal". An empty, absent or miscapitalised status
		// is a one-line front matter fault, not a frozen document.
		return nil, fmt.Errorf("status %q is not one of %s, so no transition applies; fix the front matter",
			from, strings.Join(statusNames(repo.Statuses), ", "))
	}
	permitted := from.Permitted(strict)
	if !slices.Contains(permitted, to) {
		return nil, transitionError(from, to, permitted)
	}
	if err := openQuestions(d, to); err != nil {
		return nil, err
	}
	if to.Terminal() && !d.FrontMatter.Created.IsZero() && repo.StartOfDay(now).Before(d.FrontMatter.Created) {
		// The decided date about to be written would sit before the created
		// date, which L07 reports and L11 then forbids anyone repairing: the
		// tool would be writing the value that makes the document fail its own
		// linter permanently. Reachable without hand-editing, because `new` and
		// `accept` run in whatever zone each machine is in.
		return nil, fmt.Errorf("%s was created on %s, which is later than today, so it cannot be decided yet",
			d.Path, d.FrontMatter.Created.Format(repo.DateLayout))
	}
	if to == repo.StatusRejected && d.Unclosed() {
		// The rationale is appended to the end of the file, and when the body
		// ends inside an unterminated comment or an unclosed fence that
		// position is not prose: the section would be written inside the
		// comment, so the document would still be missing the section the
		// command had just reported writing.
		return nil, fmt.Errorf(
			"%s ends inside an unterminated <!-- comment or an unclosed code fence, "+
				"so the rejection rationale cannot be appended where it would be read; close the marker first", d.Path)
	}

	out, err := repo.SetField(d.Source, "status", d.FrontMatter.LineOf("status"), string(to))
	if err != nil {
		return nil, err
	}
	if to.Terminal() {
		out, err = repo.SetField(out, "decided", d.FrontMatter.LineOf("decided"), now.Format(repo.DateLayout))
		if err != nil {
			return nil, err
		}
	}
	judged := out
	if to == repo.StatusRejected {
		judged = appendSection(out, writtenRationale)
		out = appendSection(out, rationale)
	}
	if to == repo.StatusAccepted || to == repo.StatusRejected {
		if err := refuseWhatLintReports(d, to, judged, g, now); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Unfinished refuses a transition that would freeze a document lint reports
// errors about. It carries the findings rather than a rendering of them, so a
// caller presents them as it presents lint's own.
type Unfinished struct {
	ID       string
	To       repo.Status
	Findings []lint.Finding
}

func (e *Unfinished) Error() string {
	return fmt.Sprintf("%s cannot become %s while lint reports errors about it", e.ID, e.To)
}

// refuseWhatLintReports judges the document as the transition writes it, in a
// copy of the repository holding it, and refuses while lint reports an error
// about it. What it lets through, L11 then forbids anyone repairing. Every
// error is returned, so a document is not refused once per fault. L03 is left
// out: a number another document also claims is corrected by renumbering that
// one, which stays possible once this one is frozen.
func refuseWhatLintReports(d *repo.Document, to repo.Status, judged []byte, g git.Repository, now time.Time) error {
	var blocking []lint.Finding
	for _, f := range lint.Run(lint.NewContext(d.Repo().With(d.Path, judged), g, now)) {
		if f.Path == d.Path && f.Severity == lint.Error && f.Rule != "L03" {
			blocking = append(blocking, f)
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return &Unfinished{ID: d.ID, To: to, Findings: blocking}
}

// openQuestions refuses to accept a document whose Open questions section holds
// anything, before lint is asked, so the refusal is for the open questions and
// not for their shape, and names where an answered one goes.
func openQuestions(d *repo.Document, to repo.Status) error {
	if to != repo.StatusAccepted {
		return nil
	}
	for _, section := range d.Sections(repo.OpenQuestions) {
		if !section.Empty() {
			return fmt.Errorf("the %q section is not empty; move each question to %q with its answer, and remove anything else",
				repo.OpenQuestions, repo.ResolvedQuestions)
		}
	}
	return nil
}

func transitionError(from, to repo.Status, permitted []repo.Status) error {
	// Asked of the status, not of the graph's answer. Apply has already refused
	// a status the graph does not know, so an empty permitted set here means
	// the document really is frozen.
	if from.Terminal() {
		return fmt.Errorf("%s is terminal, so it cannot become %s: write a new document that updates or obsoletes it", from, to)
	}
	return fmt.Errorf("%s cannot become %s; from %s a document may become %s",
		from, to, from, strings.Join(statusNames(permitted), ", "))
}

// statusNames renders statuses for a message.
func statusNames(statuses []repo.Status) []string {
	names := make([]string, len(statuses))
	for i, s := range statuses {
		names[i] = string(s)
	}
	return names
}

// appendSection adds a section to the end of the body, with exactly one blank
// line before it however the file happened to end, and in the file's own line
// ending so a CRLF document does not come back mixed.
func appendSection(source []byte, section string) []byte {
	ending := repo.LineEnding(source)
	trimmed := bytes.TrimRight(source, "\r\n")
	rendered := strings.ReplaceAll(section, "\n", ending)
	return []byte(string(trimmed) + ending + ending + rendered)
}
