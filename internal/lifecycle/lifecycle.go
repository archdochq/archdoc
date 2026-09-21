// Package lifecycle moves a document through its statuses and rewrites the
// file. Nothing here prompts, prints or commits.
package lifecycle

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"time"

	"archdoc.dev/internal/repo"
)

// rationale is appended to a document when it is rejected. Lint reports the
// section as empty until the prompt is replaced, which is the intended nudge to
// finish the document before committing it.
const rationale = "## " + repo.RejectionRationale + `

<!-- Considered on its merits and turned down. Why? -->
`

// AppendRationale adds the rejection rationale section to a document, as
// rejecting one does. `archdoc new --status=rejected` creates a document that
// is already rejected, so it needs the same section and the same prompt in it;
// this is exported so that there is one of each rather than a second copy in
// the templates.
func AppendRationale(source []byte) []byte { return appendSection(source, rationale) }

// openQuestions must be empty before a document is accepted, and notGated are
// the sections a freeze does not require content in.
const openQuestions = "Open questions"

var notGated = []string{openQuestions, "Changelog"}

// Permitted lists the statuses a document may move to from this one, in the
// order a chooser should offer them. A terminal status has none: to change what
// an accepted document says, write a new one that updates or obsoletes it.
func Permitted(from repo.Status, strict bool) []repo.Status {
	switch from {
	case repo.StatusDraft:
		permitted := []repo.Status{repo.StatusProposed, repo.StatusWithdrawn}
		if !strict {
			// A document that was never proposed is a different kind of thing
			// from one proposed and accepted the same day, so this is off by
			// default and the record says which happened.
			permitted = append(permitted, repo.StatusAccepted, repo.StatusRejected)
		}
		return permitted
	case repo.StatusProposed:
		return []repo.Status{repo.StatusAccepted, repo.StatusRejected, repo.StatusWithdrawn}
	default:
		return nil
	}
}

// Apply moves a document to a new status and returns the rewritten file. The
// document on disk is not touched; writing it is the caller's business.
func Apply(d *repo.Document, to repo.Status, now time.Time) ([]byte, error) {
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
	permitted := Permitted(from, strict)
	if !slices.Contains(permitted, to) {
		return nil, transitionError(from, to, permitted)
	}
	if err := gate(d, to); err != nil {
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
	if to == repo.StatusRejected {
		out = appendSection(out, rationale)
	}
	return out, nil
}

// gate refuses a transition that would freeze an unfinished document. It runs
// before anything is written, and before reject appends its own prompt, so the
// tool never fails on text it has just produced. Withdrawal gates nothing: a
// document abandoned before a verdict is empty by nature.
func gate(d *repo.Document, to repo.Status) error {
	if to != repo.StatusAccepted && to != repo.StatusRejected {
		return nil
	}
	// The H1, because L10 compares it against the front matter, reports an
	// error, and does not soften against a frozen document. Every check here
	// exists for that reason: what the gate lets through, L11 then forbids
	// anyone repairing.
	want := d.FrontMatter.Title
	if d.Type.Numbered() {
		want = d.FrontMatter.ID + ": " + d.FrontMatter.Title
	}
	switch h1, ok := d.H1(); {
	case !ok:
		return fmt.Errorf("the document has no H1; a document is finished before it is frozen")
	case h1.Text != want:
		return fmt.Errorf("the H1 reads %q but the front matter gives %q; a document is finished before it is frozen",
			h1.Text, want)
	}

	titles := d.SectionTitles()
	position := map[string]int{}
	for i, title := range titles {
		if _, seen := position[title]; !seen {
			position[title] = i
		}
	}

	previous, previousName := -1, ""
	for _, title := range d.RequiredSections() {
		// Presence is required of every section, including the two whose
		// contents are not gated: a document accepted without a Changelog at
		// all was reported missing it by L08 forever.
		sections := d.Sections(title)
		if len(sections) == 0 {
			return fmt.Errorf("section %q is missing; a document is finished before it is frozen", title)
		}

		// Order, because L08 checks it and reports an error.
		if at := position[title]; at < previous {
			return fmt.Errorf("section %q appears before %q, but must come after it; a document is finished before it is frozen",
				title, previousName)
		} else if at > previous {
			previous, previousName = at, title
		}

		// Open questions must be empty to accept, checked below; a Changelog is
		// empty by definition on a document accepted the day it was proposed.
		if slices.Contains(notGated, title) {
			continue
		}
		for _, section := range sections {
			if section.Empty() {
				return fmt.Errorf("section %q is empty; a document is finished before it is frozen", title)
			}
		}
	}
	if links := d.WikiLinks(); len(links) > 0 {
		return fmt.Errorf("unresolved wiki link [[%s]] on line %d; run archdoc link first",
			links[0].Name, links[0].Line)
	}
	if to == repo.StatusAccepted {
		for _, section := range d.Sections(openQuestions) {
			if !section.Empty() {
				return fmt.Errorf("the %q section is not empty; anything left goes to a follow-up RFC", openQuestions)
			}
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
