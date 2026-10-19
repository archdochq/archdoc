// Package export renders a repository as JSON for consumers that want the
// documents and the relationships ArchDoc derives, without reimplementing the
// front matter schema, the identifier rules or the reverse-relationship graph.
//
// The shape is a contract, held to ADR-0019. Before ArchDoc 1.0 it may break: a
// change that removes a field, or changes a field's type or meaning, moves
// Schema to the next integer, at most once between two tagged releases. From
// 1.0 it only gains optional fields.
package export

import (
	_ "embed"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"archdoc.dev/internal/repo"
)

// schemePattern matches a URI scheme at the start of a destination, per RFC
// 3986: a letter followed by letters, digits, plus, minus or dot, then a colon.
var schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// Schema is the version of the shape below. It changes only for a break.
const Schema = 3

// JSONSchema is the published schema document, so a consumer can validate
// against something rather than inferring the shape from an example.
//
//go:embed schema.json
var JSONSchema []byte

// Repository is the whole export.
type Repository struct {
	Schema int `json:"schema"`
	// Commit is the revision this export describes, as the caller gave it.
	// Taken as written: a revision read from git would be attached to a working
	// tree that may hold uncommitted edits, stamping the output with one that
	// does not describe its contents.
	Commit string `json:"commit,omitempty"`
	Config Config `json:"config"`
	// HasGlossary is present even where Glossary is not, because index.json
	// carries no glossary and a consumer reading only that still has to know
	// whether one exists.
	HasGlossary bool       `json:"has_glossary"`
	Documents   []Document `json:"documents"`
	// Groups is every group, in order of slug, and an empty array where there
	// are none. A group is a file under group/ but is published only here.
	Groups []Group `json:"groups"`
	// Glossary is every term, absent when the repository has none. The
	// page itself still appears among the documents; this is the same content
	// as data, so a consumer does not re-parse it.
	Glossary []Term `json:"glossary,omitempty"`
}

// Config is every setting archdoc.json holds. The location it was loaded from
// is not among them: it describes the machine that ran the command rather than
// the repository, and this file is published.
type Config struct {
	Name         string `json:"name"`
	Branch       string `json:"branch"`
	Root         string `json:"root"`
	Strict       bool   `json:"strict"`
	RefStaleDays int    `json:"ref_stale_days"`
}

// Document is one RFC, ADR, spec page or ref.
type Document struct {
	Type   string `json:"type"`
	ID     string `json:"id,omitempty"`
	Number int    `json:"number,omitempty"`
	Page   string `json:"page,omitempty"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Status string `json:"status,omitempty"`

	Created    string `json:"created,omitempty"`
	Decided    string `json:"decided,omitempty"`
	Backfilled string `json:"backfilled,omitempty"`
	Verified   string `json:"verified,omitempty"`
	AsOf       string `json:"as_of,omitempty"`

	// Forward relationships, as written in the front matter.
	Depends   []string `json:"depends"`
	Updates   []string `json:"updates"`
	Obsoletes []string `json:"obsoletes"`
	Includes  []string `json:"includes"`

	// Reverse relationships, derived. These are the reason this export exists:
	// nothing in a single document records what happened to it later.
	UpdatedBy    []string `json:"updated_by"`
	ObsoletedBy  []string `json:"obsoleted_by"`
	DependedOnBy []string `json:"depended_on_by"`
	IncludedIn   []string `json:"included_in"`
	// Groups holds the slugs of the groups listing this document, derived
	// from the groups' own lists.
	Groups []string `json:"groups"`

	EffectivelyObsolete bool `json:"effectively_obsolete"`
	Implemented         bool `json:"implemented"`
	Stale               bool `json:"stale"`

	// RequiredSections names the level two headings this document calls for,
	// in order. They are checked by lint rather than guaranteed, so any of them
	// may be absent from Contents: a draft part-way through being written is
	// the ordinary case. A rejected document's Rejection rationale is absent
	// from Contents whenever it is present, unless every heading was asked
	// for, and is in RejectionRationale either way.
	RequiredSections []RequiredSection `json:"required_sections"`
	// Contents is every heading in document order, each with the prose beneath
	// it, except the question sections, the Changelog and the Rejection
	// rationale, which are left out unless every heading was asked for and are
	// published below as data of their own either way. It
	// replaces an object keyed by anchor, which could not carry the order a
	// document is in and named level two headings only.
	Contents []Entry `json:"contents"`
	// Questions are the document's open and resolved questions. They record
	// matters requiring action rather than the design, so a consumer handles
	// them apart from Contents.
	Questions Questions `json:"questions"`
	// Changelog is every entry of every Changelog section, in document order.
	Changelog []ChangelogEntry `json:"changelog"`
	// RejectionRationale is every Rejection rationale section, in document
	// order. It records the verdict rather than the design, and is empty
	// exactly where the document has no such section.
	RejectionRationale []RejectionRationaleSection `json:"rejection_rationale"`
	// Source is the file exactly as it is on disk, front matter included, and
	// only when it was asked for. It is the one field that reproduces the
	// document: Contents discards a heading's raw spelling, and anything before
	// the first heading belongs to no entry.
	Source string `json:"source,omitempty"`
	Links  []Link `json:"links"`
}

// Entry is one heading and the prose beneath it, up to the next heading of any
// level. No entry holds another's prose, so walking them reads every line of a
// body exactly once.
type Entry struct {
	// Level is carried rather than implied by position, because heading levels
	// need not descend one at a time and a document that skips one has no
	// honest depth.
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
	// Line is 1-based in the file. The body begins on the next line.
	Line int `json:"line"`
	// Body is a pointer so that a heading directly followed by another, whose
	// body is genuinely empty, is distinguishable from an export that carries
	// no bodies at all. A plain string with omitempty conflates the two.
	Body *string `json:"body,omitempty"`
}

// Questions are a document's questions, open and resolved.
type Questions struct {
	Open     []OpenQuestion     `json:"open"`
	Resolved []ResolvedQuestion `json:"resolved"`
}

// OpenQuestion is a level three heading in an Open questions section.
type OpenQuestion struct {
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
	// Line is the heading's, 1-based in the file.
	Line int `json:"line"`
	// Body is the question's detail, the lines as written, absent wherever
	// contents bodies are.
	Body *string `json:"body,omitempty"`
}

// ResolvedQuestion is a level three heading in a Resolved questions section,
// with the answer it ends on.
type ResolvedQuestion struct {
	Text   string  `json:"text"`
	Anchor string  `json:"anchor"`
	Line   int     `json:"line"`
	Body   *string `json:"body,omitempty"`
	// Answer is absent where the question does not end with a blockquote,
	// which lint reports.
	Answer *Answer `json:"answer,omitempty"`
}

// Answer is the blockquote a resolved question ends with.
type Answer struct {
	// Line is the blockquote's first line, where its body starts.
	Line int `json:"line"`
	// Body is the blockquote without its markers, so it is Markdown without
	// the quote.
	Body *string `json:"body,omitempty"`
}

// ChangelogEntry is one entry of a Changelog section.
type ChangelogEntry struct {
	Line int `json:"line"`
	// Date is absent where the entry is undated.
	Date string  `json:"date,omitempty"`
	Body *string `json:"body,omitempty"`
}

// RejectionRationaleSection is one Rejection rationale section.
type RejectionRationaleSection struct {
	Anchor string `json:"anchor"`
	// Line is the heading's. The body begins on the next line.
	Line int `json:"line"`
	// Body is the section as written, subsections included, absent wherever
	// contents bodies are.
	Body *string `json:"body,omitempty"`
}

// RequiredSection is a section the type calls for, named both ways so a
// consumer can render a heading for one that turned out to be missing.
type RequiredSection struct {
	Title  string `json:"title"`
	Anchor string `json:"anchor"`
}

// Link is a relative markdown link found in a body, with what it resolves to,
// so a consumer can rewrite destinations into its own URL scheme without
// parsing markdown.
type Link struct {
	// Text is a definition's label, where the link is one.
	Text string `json:"text"`
	// Type is the form the link was written in. An image resolves like any
	// other link and renders as something else entirely, so a consumer
	// rewriting destinations has to tell them apart.
	Type        string `json:"type"`
	Destination string `json:"destination"`
	// Path is the destination resolved against the repository root, empty when
	// it leaves the repository or names nothing.
	Path string `json:"path,omitempty"`
	// ResolvesTo is the identifier of the document at Path, or a spec page's
	// name. Empty when the destination is not a document.
	ResolvesTo string `json:"resolves_to,omitempty"`
	Anchor     string `json:"anchor,omitempty"`
	Line       int    `json:"line"`
}

// Group is one group: documents that belong together, listed by the group
// rather than by its members, so a frozen document is grouped without an edit.
type Group struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Path  string `json:"path"`
	// Description is the body after the group's H1, as written, absent
	// wherever contents bodies are.
	Description *string `json:"description,omitempty"`
	// Documents are the members as written, whether or not they resolve.
	Documents []string `json:"documents"`
	Links     []Link   `json:"links"`
}

// Term is one glossary entry. A term is a document on disk but is published
// only here: it appears in no other list, so a consumer has one representation
// of it rather than two that could disagree.
type Term struct {
	Name   string `json:"name"`
	Anchor string `json:"anchor"`
	// Path is the term's file, relative to root. The anchor is on the
	// generated glossary, which is what every link to a term targets.
	Path       string   `json:"path"`
	Definition string   `json:"definition"`
	Formerly   []string `json:"formerly,omitempty"`
	// NamedBy is the document that introduced the term. Not an inclusion: a
	// term is defined by a document, not implemented by one.
	NamedBy string `json:"named_by,omitempty"`
}

// Options selects what to include. The zero value exports everything.
type Options struct {
	// NoBodies drops every contents body, and the bodies of questions, answers,
	// Changelog entries and Rejection rationale sections, leaving metadata, the
	// relationship graph and the full outline. An index page wants this and
	// none of the prose.
	NoBodies bool
	// AllHeadings keeps every heading in Contents, the question sections, the
	// Changelog and the Rejection rationale included, for a consumer rendering
	// a document as written.
	AllHeadings bool
	// Source adds each document's file as it is on disk.
	Source bool
	// Commit is the revision the export describes, recorded as given.
	Commit string
}

// Build renders a repository. It never fails: a repository that does not lint
// clean still exports, because refusing would make the command useless exactly
// when someone is part-way through writing something.
func Build(r *repo.Repo, opts Options) Repository {
	c := r.Config()
	out := Repository{
		Schema: Schema,
		Commit: opts.Commit,
		Config: Config{
			Name:         c.Name,
			Branch:       c.Branch,
			Root:         c.Root,
			Strict:       c.Strict,
			RefStaleDays: c.RefStaleDays,
		},
	}
	_, out.HasGlossary = r.Glossary()
	out.Groups = []Group{}
	for _, d := range r.Documents() {
		switch d.Type {
		case repo.TypeTerm:
			// Terms are published in Glossary alone.
		case repo.TypeGroup:
			out.Groups = append(out.Groups, group(r, d, opts))
		default:
			out.Documents = append(out.Documents, document(r, d, opts))
		}
	}
	// By slug, not by the path groups are read in: "a-b.md" sorts before
	// "a.md", while the slug "a" sorts before "a-b".
	slices.SortFunc(out.Groups, func(a, b Group) int { return strings.Compare(a.Slug, b.Slug) })
	if entries, ok := r.Glossary(); ok {
		for _, e := range entries {
			out.Glossary = append(out.Glossary, Term{
				Name:       e.Term,
				Anchor:     e.Anchor,
				Path:       e.Path,
				Definition: strings.Join(e.Paragraphs, "\n\n"),
				Formerly:   e.Formerly,
				NamedBy:    e.NamedBy,
			})
		}
	}
	return out
}

func document(r *repo.Repo, d *repo.Document, opts Options) Document {
	fm := d.FrontMatter
	out := Document{
		Type:   string(d.Type),
		ID:     d.ID,
		Number: d.Number,
		Page:   d.Page,
		Path:   d.Path,
		Title:  fm.Title,
		Status: string(fm.Status),

		Created:    date(fm.Created),
		Decided:    date(fm.Decided),
		Backfilled: date(fm.Backfilled),
		Verified:   date(fm.Verified),
		AsOf:       date(fm.AsOf),

		Depends:   list(fm.Depends),
		Updates:   list(fm.Updates),
		Obsoletes: list(fm.Obsoletes),
		Includes:  list(fm.Includes),

		UpdatedBy:    list(d.UpdatedBy),
		ObsoletedBy:  list(d.ObsoletedBy),
		DependedOnBy: list(d.DependedOnBy),
		IncludedIn:   list(d.IncludedIn),
		Groups:       list(d.Groups),

		EffectivelyObsolete: d.EffectivelyObsolete,
		Implemented:         d.Implemented,
		Stale:               d.Stale,

		// Initialised rather than left nil so every one of these is an
		// array in the JSON. A spec page has no required sections and a
		// document may have no links; a consumer should not have to handle
		// both an empty array and null for the same field.
		RequiredSections:   []RequiredSection{},
		Contents:           []Entry{},
		Questions:          Questions{Open: []OpenQuestion{}, Resolved: []ResolvedQuestion{}},
		Changelog:          []ChangelogEntry{},
		RejectionRationale: []RejectionRationaleSection{},
		Links:              []Link{},
	}
	if opts.Source {
		out.Source = string(d.Source)
	}

	body := func(text string) *string {
		if opts.NoBodies {
			return nil
		}
		return &text
	}

	anchors := map[string]string{} // title -> anchor, first occurrence
	structured := false            // inside a section published as data
	for _, e := range d.Contents() {
		h := e.Heading
		if h.Level <= 2 {
			structured = h.Level == 2 && (h.Text == repo.OpenQuestions || h.Text == repo.ResolvedQuestions ||
				h.Text == repo.Changelog || h.Text == repo.RejectionRationale)
		}
		if h.Level == 2 {
			if _, seen := anchors[h.Text]; !seen {
				anchors[h.Text] = h.Anchor
			}
		}
		if structured && !opts.AllHeadings {
			continue
		}
		out.Contents = append(out.Contents, Entry{Level: h.Level, Text: h.Text, Anchor: h.Anchor, Line: h.Line, Body: body(e.Text)})
	}

	questions := d.Questions()
	for _, q := range questions.Open {
		out.Questions.Open = append(out.Questions.Open, OpenQuestion{
			Text: q.Heading.Text, Anchor: q.Heading.Anchor, Line: q.Heading.Line, Body: body(q.Detail),
		})
	}
	for _, q := range questions.Resolved {
		resolved := ResolvedQuestion{Text: q.Heading.Text, Anchor: q.Heading.Anchor, Line: q.Heading.Line, Body: body(q.Detail)}
		if q.Answer != nil {
			resolved.Answer = &Answer{Line: q.Answer.Line, Body: body(q.Answer.Text)}
		}
		out.Questions.Resolved = append(out.Questions.Resolved, resolved)
	}
	for _, e := range d.Changelog() {
		out.Changelog = append(out.Changelog, ChangelogEntry{Line: e.Line, Date: date(e.Date), Body: body(e.Text)})
	}
	for _, s := range d.Sections(repo.RejectionRationale) {
		out.RejectionRationale = append(out.RejectionRationale, RejectionRationaleSection{
			Anchor: s.Heading.Anchor, Line: s.Heading.Line, Body: body(s.Text),
		})
	}
	for _, title := range d.RequiredSections() {
		anchor, ok := anchors[title]
		if !ok {
			// Absent: name the anchor it would have had, so a consumer can
			// still render a heading and say it is missing.
			anchor = repo.Anchor(title)
		}
		out.RequiredSections = append(out.RequiredSections, RequiredSection{Title: title, Anchor: anchor})
	}

	for _, l := range d.Links() {
		out.Links = append(out.Links, link(r, d, l))
	}
	return out
}

func group(r *repo.Repo, d *repo.Document, opts Options) Group {
	out := Group{
		Slug:      d.Page,
		Title:     d.FrontMatter.Title,
		Path:      d.Path,
		Documents: list(d.FrontMatter.Documents),
		Links:     []Link{},
	}
	if !opts.NoBodies {
		description := d.Description()
		out.Description = &description
	}
	for _, l := range d.Links() {
		out.Links = append(out.Links, link(r, d, l))
	}
	return out
}

// link resolves a destination against the document holding it, so a consumer
// rewriting URLs does not have to know where the document sits.
func link(r *repo.Repo, d *repo.Document, l repo.Link) Link {
	out := Link{Text: l.Text, Type: string(l.Type), Destination: l.Target, Line: l.Line}
	target, anchor, _ := strings.Cut(l.Target, "#")
	out.Anchor = anchor
	// Anything carrying a scheme is somewhere else entirely, and an absolute
	// path is not ours to resolve. "://" alone is not the test: mailto: and
	// tel: have no authority component and were being resolved as filenames.
	if target == "" || schemePattern.MatchString(target) || strings.HasPrefix(target, "/") {
		return out
	}
	resolved := path.Clean(path.Join(path.Dir(d.Path), target))
	if strings.HasPrefix(resolved, "..") {
		return out
	}
	out.Path = resolved
	if other := r.ByPath(resolved); other != nil {
		out.ResolvesTo = other.ID
		if out.ResolvesTo == "" {
			out.ResolvesTo = other.Page
		}
	}
	return out
}

func date(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(repo.DateLayout)
}

// list never returns nil, so every relationship field is an array in the JSON
// rather than null. A consumer should not have to handle both.
func list(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
