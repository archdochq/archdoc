package lint

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/archdochq/archdoc/internal/repo"
)

// report collects the findings of one rule. Every rule builds one, records
// against it, and returns its findings.
//
// It exists because the Finding literal was written out forty-two times, and
// each one repeated the same four field names. The cost of that was not the
// typing: a literal that omitted Severity said "warning" by accident rather
// than by decision, and nothing marked the omission.
type report struct{ findings []Finding }

// add records a finding whose message is already built.
//
// Messages that come from somewhere else arrive here rather than through one of
// the formatting methods below: a YAML parse error naming a value of "100%"
// would otherwise be formatted a second time and reach the user as
// "100%!(MISSING)".
func (r *report) add(severity Severity, path string, line int, message string) {
	r.findings = append(r.findings, Finding{
		Path: path, Line: line, Severity: severity, Message: message,
	})
}

// addf records a finding against a path rather than a document, for the two
// places L11 speaks about the repository itself or about a document that is no
// longer in the working tree.
func (r *report) addf(severity Severity, path string, line int, format string, args ...any) {
	r.add(severity, path, line, fmt.Sprintf(format, args...))
}

// errorf records an error against a document.
func (r *report) errorf(d *repo.Document, line int, format string, args ...any) {
	r.addf(Error, d.Path, line, format, args...)
}

// warnf records a warning against a document.
func (r *report) warnf(d *repo.Document, line int, format string, args ...any) {
	r.addf(Warning, d.Path, line, format, args...)
}

// atf records a finding whose severity the caller decides, which is how the
// rules that soften against a frozen document report. See severityFor.
func (r *report) atf(severity Severity, d *repo.Document, line int, format string, args ...any) {
	r.addf(severity, d.Path, line, format, args...)
}

// L01 checks that front matter parses and matches the schema for the type.
func L01(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		// Faults recorded while parsing: invalid YAML, a duplicate key, a date
		// that is not one. The message is already written, so it is added rather
		// than formatted.
		for _, p := range d.Problems {
			rep.add(Error, d.Path, p.Line, p.Message)
		}
		for _, key := range d.FrontMatter.Unknown() {
			rep.errorf(d, key.Line, "unknown front matter field %q", key.Name)
		}
		// Keys that must carry a value, as opposed to keys that must merely be
		// present. `decided` and the relationship lists are legitimately empty;
		// a document with an empty `status` would otherwise be skipped in
		// silence by every rule that branches on it.
		empty := map[string]bool{
			"id":       d.FrontMatter.ID == "",
			"title":    d.FrontMatter.Title == "",
			"status":   d.FrontMatter.Status == "",
			"created":  d.FrontMatter.Created.IsZero(),
			"verified": d.FrontMatter.Verified.IsZero(),
		}
		// A key whose value could not be parsed has already been reported
		// accurately; calling it empty as well says something the file
		// contradicts, because the decoded value is zero only because parsing
		// failed.
		unparsed := map[int]bool{}
		for _, p := range d.Problems {
			unparsed[p.Line] = true
		}

		for _, key := range repo.RequiredKeys(d.Type) {
			if !d.FrontMatter.Has(key) {
				rep.errorf(d, 1, "missing required front matter key %q", key)
				continue
			}
			if blank, checked := empty[key]; checked && blank && !unparsed[d.FrontMatter.LineOf(key)] {
				rep.errorf(d, d.FrontMatter.LineOf(key), "front matter key %q is present but empty", key)
			}
		}
		if d.Type.HasLifecycle() {
			if s := d.FrontMatter.Status; s != "" && !validStatus(s) {
				rep.errorf(d, 1, "status %q is not one of draft, proposed, accepted, rejected, withdrawn", s)
			}
		}
	}
	return rep.findings
}

func validStatus(s repo.Status) bool {
	return slices.Contains(repo.Statuses, s)
}

// L02 checks that the identifier matches the type and filename, and that the
// filename has the required shape.
func L02(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		if !d.Type.Numbered() {
			continue
		}
		if d.ID == "" {
			rep.errorf(d, 0, "filename does not match <NNNN>-<slug>.md, so no identifier can be derived")
			continue
		}
		if d.Number == 0 {
			rep.errorf(d, 0, "filename number is 0000, but sequences start at 1")
		}
		if got := d.FrontMatter.ID; got != d.ID {
			rep.errorf(d, 1, "id is %q but the filename gives %q", got, d.ID)
		}
	}
	return rep.findings
}

// L03 checks that no identifier appears on more than one document.
func L03(ctx Context) []Finding {
	byID := map[string][]*repo.Document{}
	for _, d := range ctx.Repo().Documents() {
		if d.ID != "" {
			byID[d.ID] = append(byID[d.ID], d)
		}
	}
	var rep report
	for id, docs := range byID {
		if len(docs) < 2 {
			continue
		}
		for _, d := range docs {
			var others []string
			for _, other := range docs {
				if other != d {
					others = append(others, other.Path)
				}
			}
			rep.errorf(d, 0, "identifier %s is also used by %s", id, strings.Join(others, ", "))
		}
	}
	return rep.findings
}

// reference is one entry of one forward relationship, named so that findings
// can say which list the fault is in.
type reference struct {
	field string
	id    string
}

// references lists every forward relationship a document declares.
func references(d *repo.Document) []reference {
	fm := d.FrontMatter
	var out []reference
	for _, group := range []struct {
		field string
		ids   []string
	}{
		{"depends", fm.Depends},
		{"includes", fm.Includes},
		{"obsoletes", fm.Obsoletes},
		{"updates", fm.Updates},
	} {
		for _, id := range group.ids {
			out = append(out, reference{field: group.field, id: id})
		}
	}
	return out
}

// L04 checks that every reference resolves to an existing document of a type
// the field permits.
func L04(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		for _, ref := range references(d) {
			target := ctx.Repo().ByID(ref.id)
			if target == nil {
				rep.errorf(d, 1, "%s references %s, which does not exist", ref.field, ref.id)
				continue
			}
			// depends and includes name designs and decisions. Refs are
			// informational and are cited from the body instead.
			if ref.field == "depends" || ref.field == "includes" {
				if !target.Type.Normative() {
					rep.errorf(d, 1, "%s references %s, but only RFCs and ADRs are permitted there", ref.field, ref.id)
				}
			}
		}
	}
	return rep.findings
}

// L05 checks updates and obsoletes: same type, accepted, not the document
// itself, and not both at once.
func L05(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		// Driven from the document's own list rather than a map, so findings
		// come out in the order the identifiers were written. Findings that
		// share a path, a line and a rule are ties under the output sort, so a
		// map here would make lint output differ between runs.
		for _, id := range d.FrontMatter.Updates {
			if !slices.Contains(d.FrontMatter.Obsoletes, id) {
				continue
			}
			rep.errorf(d, 1, "%s appears in both updates and obsoletes", id)
		}

		for _, ref := range references(d) {
			if ref.field != "updates" && ref.field != "obsoletes" {
				continue
			}
			if ref.id == d.ID {
				rep.errorf(d, 1, "%s names itself in %s", ref.id, ref.field)
				continue
			}
			target := ctx.Repo().ByID(ref.id)
			if target == nil {
				continue // L04 reports it
			}
			if target.Type != d.Type {
				rep.errorf(d, 1, "%s references %s, which is a %s: only documents of the same type may be updated or obsoleted",
					ref.field, ref.id, target.Type)
				continue
			}
			if s := target.FrontMatter.Status; s != repo.StatusAccepted && s != "" {
				// Not when the status is empty: L01 reports the missing value,
				// and this rule would only render it into a broken sentence.
				rep.errorf(d, 1, "%s references %s, which is %s: only an accepted document is in force and can be amended or replaced",
					ref.field, ref.id, s)
			}
		}
	}
	return rep.findings
}

// L06 checks that a spec page includes only accepted documents.
func L06(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		if d.Type != repo.TypeSpec {
			continue
		}
		for _, id := range d.FrontMatter.Includes {
			target := ctx.Repo().ByID(id)
			if target == nil || target.FrontMatter.Status == repo.StatusAccepted {
				continue // L04 reports one that does not exist
			}
			if target.FrontMatter.Status == "" {
				// A ref has no status by design, and L04 already reports that a
				// spec page may not include one. Saying "which is :" here adds
				// nothing but a malformed sentence.
				continue
			}
			rep.errorf(d, 1, "includes %s, which is %s: a spec page reflects accepted documents only",
				id, target.FrontMatter.Status)
		}
	}
	return rep.findings
}

// L07 checks decided against the status and the created date.
func L07(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		if !d.Type.HasLifecycle() {
			continue
		}
		fm := d.FrontMatter
		terminal := fm.Status.Terminal()
		switch {
		case terminal && fm.Decided.IsZero():
			rep.errorf(d, 1, "status is %s but decided is not set", fm.Status)
		case !terminal && !fm.Decided.IsZero():
			rep.errorf(d, 1, "decided is set but status is %s, which is not terminal", fm.Status)
		}
		if !fm.Backfilled.IsZero() {
			if !terminal {
				rep.errorf(d, fm.LineOf("backfilled"),
					"backfilled is set but status is %s: backfilling records a decision already taken", fm.Status)
			}
			if !fm.Decided.IsZero() && fm.Backfilled.Before(fm.Decided) {
				rep.errorf(d, fm.LineOf("backfilled"),
					"backfilled %s is before decided %s: a record cannot predate what it records",
					fm.Backfilled.Format(repo.DateLayout), fm.Decided.Format(repo.DateLayout))
			}
		}
		if !fm.Decided.IsZero() && !fm.Created.IsZero() && fm.Decided.Before(fm.Created) {
			rep.errorf(d, 1, "decided %s is before created %s",
				fm.Decided.Format(repo.DateLayout), fm.Created.Format(repo.DateLayout))
		}
	}
	return rep.findings
}

// L08 checks that the required sections are present, correctly titled and in
// the required relative order, and that Rejection rationale is present exactly
// when the document is rejected.
func L08(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		required := d.RequiredSections()
		if len(required) == 0 {
			continue
		}
		titles := d.SectionTitles()
		position := map[string]int{}
		for i, title := range titles {
			if _, seen := position[title]; !seen {
				position[title] = i
			}
		}

		// Present, and in the required relative order.
		previous, previousName := -1, ""
		for _, want := range required {
			at, present := position[want]
			if !present {
				rep.errorf(d, 0, "missing required section %q", want)
				continue
			}
			if at < previous {
				rep.errorf(d, 0, "section %q appears before %q, but must come after it", want, previousName)
			}
			if at > previous {
				previous, previousName = at, want
			}
		}

		// The closing section, where the type has one. Checked only when it is
		// actually present: reporting that a missing section is also in the
		// wrong place is two findings for one fault, the second contradicting
		// the first.
		if closing, wanted := d.ClosingSection(); wanted {
			_, present := position[closing]
			if present && len(titles) > 0 && titles[len(titles)-1] != closing {
				rep.errorf(d, 0, "%q must be the last H2, but %q follows it", closing, titles[len(titles)-1])
			}
		}

		// The other half of this rule, a rejected document without the section,
		// is enforced by RequiredSections adding it.
		if d.Type.HasLifecycle() {
			_, present := position[repo.RejectionRationale]
			if rejected := d.FrontMatter.Status == repo.StatusRejected; !rejected && present {
				rep.errorf(d, 0, "%q is present but the status is %s", repo.RejectionRationale, d.FrontMatter.Status)
			}
		}
	}
	return rep.findings
}

// L09 checks that an accepted document leaves no open questions. Keyed on the
// section being present rather than on the document type, so it still holds if
// an ADR carries one as an extra section.
func L09(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		if d.FrontMatter.Status != repo.StatusAccepted {
			continue
		}
		for _, section := range d.Sections("Open questions") {
			if section.Empty() {
				continue
			}
			rep.errorf(d, section.Heading.Line, `an accepted document must have an empty "Open questions" section`)
		}
	}
	return rep.findings
}

// L10 checks the H1 against the front matter.
func L10(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		want := d.FrontMatter.Title
		if d.Type.Numbered() {
			want = d.FrontMatter.ID + ": " + d.FrontMatter.Title
		}
		h1, ok := d.H1()
		if !ok {
			rep.errorf(d, 0, "the first heading must be an H1 reading %q", want)
			continue
		}
		if h1.Text != want {
			rep.errorf(d, h1.Line, "H1 is %q but the front matter gives %q", h1.Text, want)
		}
	}
	return rep.findings
}

// L12 checks that a spec page includes nothing that is effectively obsolete.
// Obsolescence claimed by a document that is not accepted has no effect, so a
// page is stale only once the replacement has actually been decided.
func L12(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		if d.Type != repo.TypeSpec || !d.Stale {
			continue
		}
		var obsolete []string
		for _, id := range d.FrontMatter.Includes {
			if target := ctx.Repo().ByID(id); target != nil && target.EffectivelyObsolete {
				obsolete = append(obsolete, id)
			}
		}
		rep.warnf(d, 1, "stale: includes %s, which %s been obsoleted",
			strings.Join(obsolete, ", "), plural(len(obsolete), "has", "have"))
	}
	return rep.findings
}

// L13 checks that a ref has been verified recently enough. A ref_stale_days of
// zero disables the rule.
func L13(ctx Context) []Finding {
	days := ctx.Repo().Config().RefStaleDays
	if days == 0 {
		return nil
	}
	var rep report
	// Dates, not instants. verified is parsed to midnight UTC, so subtracting
	// the window from a wall-clock time made the answer depend on the hour the
	// run happened and on the zone it happened in: a ref exactly ref_stale_days
	// old was silent at midnight UTC and stale by breakfast in London.
	cutoff := repo.StartOfDay(ctx.Now()).AddDate(0, 0, -days)
	for _, d := range ctx.Repo().Documents() {
		if d.Type != repo.TypeRef || d.FrontMatter.Verified.IsZero() {
			continue
		}
		if !d.FrontMatter.Verified.Before(cutoff) {
			continue
		}
		rep.warnf(d, 1, "verified %s, more than %d days ago",
			d.FrontMatter.Verified.Format(repo.DateLayout), days)
	}
	return rep.findings
}

// L14 checks that no required section was left empty. It does not apply to
// withdrawn documents, whose sections are empty by the nature of abandonment,
// and it exempts the two sections that are legitimately empty: Open questions,
// which L09 requires to be empty before acceptance, and Changelog, which stays
// empty for a document never edited while proposed.
func L14(ctx Context) []Finding {
	exempt := map[string]bool{"Open questions": true, "Changelog": true}

	var rep report
	for _, d := range ctx.Repo().Documents() {
		status := d.FrontMatter.Status
		if status == repo.StatusWithdrawn {
			continue
		}
		var severity Severity
		switch {
		case d.Type == repo.TypeRef:
			// A ref has no lifecycle and is always editable, so its Sources
			// section is checked from the moment it exists.
			severity = Error
		case status == repo.StatusProposed:
			severity = Warning
		case status.Terminal():
			severity = severityFor(ctx, d)
		default:
			continue // a draft may be as empty as it likes
		}

		for _, title := range d.RequiredSections() {
			if exempt[title] {
				continue
			}
			// Every instance, not the first: a second heading with the same
			// title is legal, and the accept gate already reads them all.
			for _, section := range d.Sections(title) {
				if section.Empty() {
					rep.atf(severity, d, section.Heading.Line, "section %q is empty", title)
				}
			}
		}
	}
	return rep.findings
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// gitPrefix is the path from the git repository root to archdoc's root, which
// is how a path relative to root becomes a path git understands. Both sides are
// resolved first, because a repository reached through a symlink would
// otherwise produce a relative path that climbs out of the tree.
func gitPrefix(ctx Context) string {
	top, err := ctx.Git().RepoRoot()
	if err != nil {
		return ""
	}
	local := ctx.Repo().Config().RootDir()
	if resolved, err := filepath.EvalSymlinks(local); err == nil {
		local = resolved
	}
	rel, err := filepath.Rel(top, local)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// L11 checks that a frozen document is unchanged. A document is frozen once it
// has reached the branch in a terminal status; until then the working tree may
// differ freely, which is what makes the freezing commit legal.
func L11(ctx Context) []Finding {
	var rep report
	if ctx.Git() == nil {
		rep.add(Warning, ".", 0, "skipped: not inside a git repository, so frozen documents cannot be compared")
		return rep.findings
	}
	branch := ctx.Repo().Config().Branch
	exists, err := ctx.Git().BranchExists(branch)
	if err != nil {
		rep.add(Error, ".", 0, err.Error())
		return rep.findings
	}
	if !exists {
		// Before the first commit there is no branch and nothing is frozen, so
		// there is nothing this rule could miss: `archdoc init` followed by
		// `archdoc lint` is the first thing anyone does and should not fail.
		// A branch that is absent from a repository that *does* have commits is
		// a different matter: it means the configured name is wrong, and
		// passing quietly would disable the freeze check altogether.
		if committed, err := ctx.Git().HasCommits(); err == nil && !committed {
			rep.add(Warning, ".", 0,
				"skipped: the repository has no commits yet, so no document can be frozen")
			return rep.findings
		}
		rep.addf(Error, ".", 0, "branch %q does not exist, so no document can be checked against it", branch)
		return rep.findings
	}

	prefix := gitPrefix(ctx)
	inTree := map[string]bool{}

	// Which documents are frozen is decided from the committed bytes; whether
	// one has *changed* is decided by git, in one question below. The two used
	// to be the same byte comparison, which was wrong: git applies checkout
	// filters between the blob it stores and the file on disk, so under
	// end-of-line normalisation every frozen document differed from its blob
	// while git reported the tree clean.
	frozen := make([]string, 0)
	frozenDocs := make(map[string]*repo.Document)
	for _, d := range ctx.Repo().Documents() {
		inTree[d.Path] = true
		on, onBranch := ctx.OnBranch(d.Path)
		switch {
		case !onBranch:
			continue // new on the branch, so not yet frozen
		case on.Err != nil:
			// Swallowing this would report the document as editable, which is
			// the opposite of what a failure to read the branch implies.
			rep.errorf(d, 0, "cannot read this document on %s: %v", branch, on.Err)
		case on.Status.Terminal():
			full := path.Join(prefix, d.Path)
			frozen = append(frozen, full)
			frozenDocs[full] = d
		}
	}

	changed, err := ctx.Git().Changed(branch, frozen)
	if err != nil {
		rep.add(Error, ".", 0, err.Error())
		return rep.findings
	}
	for _, full := range frozen {
		if changed[full] {
			rep.errorf(frozenDocs[full], 0,
				"changed since it was frozen on %s: a terminal document is never modified", branch)
		}
	}

	// A document deleted from the working tree is invisible to discovery, so
	// the branch has to be enumerated to notice it.
	files, err := ctx.Git().ListFiles(branch)
	if err != nil {
		rep.add(Error, ".", 0, err.Error())
		return rep.findings
	}

	// Collected first, then read in one batch. This was the last caller reading
	// a document at a time, and it is exactly the case the batch does not cover
	// otherwise: the snapshot is built from what discovery found in the working
	// tree, and these are the paths that are not there. A `git rm` of 200
	// documents cost 200 subprocesses and about three seconds.
	gone := make([]string, 0)
	docPaths := make(map[string]string, len(files))
	for _, file := range files {
		docPath := file
		if prefix != "" {
			var inRoot bool
			if docPath, inRoot = strings.CutPrefix(file, prefix+"/"); !inRoot {
				continue
			}
		}
		if _, isDoc := repo.TypeOf(docPath); !isDoc || inTree[docPath] {
			continue
		}
		gone = append(gone, file)
		docPaths[file] = docPath
	}

	committed, err := ctx.Git().FilesAt(branch, gone)
	if err != nil {
		// Recorded against each of them, as the per-document read did: a
		// failure to read the branch must not read as "these are all fine".
		for _, file := range gone {
			rep.addf(Error, docPaths[file], 0, "cannot read this document on %s: %v", branch, err)
		}
		return rep.findings
	}
	for _, file := range gone {
		content, ok := committed[file]
		if !ok || !repo.StatusIn(content).Terminal() {
			continue
		}
		rep.addf(Error, docPaths[file], 0, "frozen on %s but deleted from the working tree", branch)
	}
	return rep.findings
}

// L15 checks the glossary: unique terms, in ascending case-insensitive order,
// one paragraph each.
func L15(ctx Context) []Finding {
	entries, ok := ctx.Repo().Glossary()
	if !ok {
		return nil
	}
	page := ctx.Repo().ByPage(repo.GlossaryPage)

	var rep report

	// Every heading from the first entry onwards must be an entry. spec/spec/lint.md
	// says so and nothing checked it: a page with an ordinary H1 among the
	// entries linted clean while the prose beneath it was silently not a term,
	// and that is the same shape that used to make `term remove` destructive.
	if len(entries) > 0 {
		for _, h := range page.Headings() {
			if h.Line < entries[0].Line || h.Level == 2 {
				continue // preamble, including the title, or an entry
			}
			rep.errorf(page, h.Line,
				"%q is not a term entry, but everything from the first entry onwards must be one", h.Text)
		}
	}

	seen := map[string]int{}
	previous := ""
	for _, e := range entries {
		key := strings.ToLower(e.Term)
		if line, duplicate := seen[key]; duplicate {
			rep.errorf(page, e.Line, "term %q repeats the entry on line %d", e.Term, line)
		} else {
			seen[key] = e.Line
		}
		if previous != "" && key < previous {
			rep.errorf(page, e.Line, "term %q is out of alphabetical order", e.Term)
		}
		previous = max(previous, key)

		if n := len(e.Paragraphs); n != 1 {
			rep.errorf(page, e.Line, "term %q has %d paragraphs, want exactly one", e.Term, n)
		}
	}
	return rep.findings
}

// severityFor is error in an editable document and warning in a frozen one,
// where no permitted edit could clear the finding.
func severityFor(ctx Context, d *repo.Document) Severity {
	if ctx.Frozen(d) {
		return Warning
	}
	return Error
}

// L16 checks that no [[...]] wiki link is left unexpanded.
func L16(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		for _, link := range d.WikiLinks() {
			rep.atf(severityFor(ctx, d), d, link.Line, "unresolved wiki link [[%s]]: run archdoc link", link.Name)
		}
	}
	return rep.findings
}

// L17 checks that every relative link resolves to a file, and to a heading when
// it carries an anchor.
func L17(ctx Context) []Finding {
	root := ctx.Repo().Config().RootDir()
	var rep report

	// Every link target was read and re-parsed on each reference, which on a
	// repository of any size dominated the whole rule set. A document the
	// repository already holds is never re-read at all.
	cached := map[string][]string{}
	anchorsOf := func(resolved string, source []byte) []string {
		if known := ctx.Repo().ByPath(resolved); known != nil {
			anchors := make([]string, 0, len(known.Headings()))
			for _, h := range known.Headings() {
				anchors = append(anchors, h.Anchor)
			}
			return anchors
		}
		if anchors, ok := cached[resolved]; ok {
			return anchors
		}
		anchors := repo.AnchorsIn(source)
		cached[resolved] = anchors
		return anchors
	}

	for _, d := range ctx.Repo().Documents() {
		severity := severityFor(ctx, d)
		for _, link := range d.Links() {
			target, anchor, _ := strings.Cut(link.Target, "#")
			if strings.Contains(target, "://") || strings.HasPrefix(link.Target, "mailto:") {
				continue
			}

			source := d.Source
			resolved := d.Path
			if target != "" {
				resolved = path.Join(path.Dir(d.Path), target)
				if resolved == ".." || strings.HasPrefix(resolved, "../") {
					// path.Join cleans a leading ".." away only as far as the
					// document's own directory; beyond that the path really
					// does leave root, and reading it was neither wanted nor
					// this tool's business. Existence alone is an answer about
					// a file the repository does not own.
					rep.atf(severity, d, link.Line, "link target %q leaves the repository", target)
					continue
				}
				full := filepath.Join(root, filepath.FromSlash(resolved))

				// The file was read from disk once per reference to it, before
				// anything asked whether its contents were even wanted, so the
				// cost was one read per link rather than one per document. The
				// anchor cache below only ever saved the parse.
				switch known := ctx.Repo().ByPath(resolved); {
				case cached[resolved] != nil:
					// Already read and parsed for an earlier reference to the
					// same target. The cache below only ever saved the parse,
					// so a target linked from many places was re-read from disk
					// every time. This skips the per-reference existence check
					// for that target, which is the assumption the
					// repository-held arm already makes.
				case known != nil:
					// Already read and already parsed, and the repository
					// holding it is proof that it exists.
					source = known.Source
				case anchor == "" || path.Ext(target) != ".md":
					// Only existence is in question, so do not read the body.
					if _, err := os.Stat(full); err != nil {
						rep.atf(severity, d, link.Line, "link target %q does not exist", target)
						continue
					}
				default:
					contents, err := os.ReadFile(full)
					if err != nil {
						rep.atf(severity, d, link.Line, "link target %q does not exist", target)
						continue
					}
					source = contents
				}
			}
			if anchor == "" {
				continue
			}
			if target != "" && path.Ext(target) != ".md" {
				continue // not markdown, so there are no headings to check
			}
			if !slices.Contains(anchorsOf(resolved, source), anchor) {
				where := target
				if where == "" {
					where = "this document"
				}
				rep.atf(severity, d, link.Line, "link to %s#%s: no heading with that anchor", where, anchor)
			}
		}
	}
	return rep.findings
}

// L18 checks that a document does not rest on something less settled than
// itself. Two proposed documents may depend on one another, but an accepted
// document resting on a draft is suspicious.
func L18(ctx Context) []Finding {
	var rep report
	for _, d := range ctx.Repo().Documents() {
		for _, id := range d.FrontMatter.Depends {
			target := ctx.Repo().ByID(id)
			if target == nil {
				continue // L04 reports it
			}
			status := target.FrontMatter.Status
			if status == repo.StatusAccepted {
				continue
			}
			if status == "" {
				// A ref has no status, so there is nothing less settled to
				// report. L04 already reports that depends may not name one,
				// and rendering the empty status gave "which is , while".
				continue
			}
			if !d.FrontMatter.Status.Terminal() && status == d.FrontMatter.Status {
				continue
			}
			if d.FrontMatter.Status == "" {
				// The referencing side, for the same reason as the referenced
				// one: with no status there is nothing to compare, and the
				// message would end "while this document is ".
				continue
			}
			rep.warnf(d, 1, "depends on %s, which is %s, while this document is %s",
				id, status, d.FrontMatter.Status)
		}
	}
	return rep.findings
}
