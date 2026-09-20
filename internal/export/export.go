// Package export renders a repository as JSON for consumers that want the
// documents and the relationships ArchDoc derives, without reimplementing the
// front matter schema, the identifier rules or the reverse-relationship graph.
//
// The shape is a contract. Like the front matter schema it describes, it may
// only ever gain optional fields: a consumer pinned to an older Schema must
// keep working against a newer ArchDoc.
package export

import (
	_ "embed"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/ollieread/archdoc/internal/repo"
)

// schemePattern matches a URI scheme at the start of a destination, per RFC
// 3986: a letter followed by letters, digits, plus, minus or dot, then a colon.
var schemePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// Schema is the version of the shape below. It changes only for a break, which
// the rule above is meant to prevent ever being necessary.
const Schema = 1

// JSONSchema is the published schema document, so a consumer can validate
// against something rather than inferring the shape from an example.
//
//go:embed schema.json
var JSONSchema []byte

// Repository is the whole export.
type Repository struct {
	Schema    int        `json:"schema"`
	Name      string     `json:"name"`
	Documents []Document `json:"documents"`
	// Glossary is the parsed spec/glossary.md, absent when the page is. The
	// page itself still appears among the documents; this is the same content
	// as data, so a consumer does not re-parse it.
	Glossary []Term `json:"glossary,omitempty"`
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

	EffectivelyObsolete bool `json:"effectively_obsolete"`
	Implemented         bool `json:"implemented"`
	Stale               bool `json:"stale"`

	// RequiredSections names the sections this document's type calls for, in
	// order. They are checked by lint rather than guaranteed, so any of them
	// may be absent from Sections: a draft part-way through being written is
	// the ordinary case.
	RequiredSections []RequiredSection `json:"required_sections"`
	// Sections is keyed by anchor, which is unique within a document because
	// a repeated heading gains a -1 suffix. Keying by title would silently
	// drop one of them.
	Sections map[string]Section `json:"sections"`

	Body  string `json:"body,omitempty"`
	Links []Link `json:"links"`
}

// RequiredSection is a section the type calls for, named both ways so a
// consumer can render a heading for one that turned out to be missing.
type RequiredSection struct {
	Title  string `json:"title"`
	Anchor string `json:"anchor"`
}

// Section is one H2 and everything under it, subsections included. Every
// section here is level two, so the level is not recorded.
type Section struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Link is a relative markdown link found in a body, with what it resolves to,
// so a consumer can rewrite destinations into its own URL scheme without
// parsing markdown.
type Link struct {
	Text        string `json:"text"`
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

// Term is one glossary entry.
type Term struct {
	Name       string   `json:"name"`
	Anchor     string   `json:"anchor"`
	Definition string   `json:"definition"`
	Formerly   []string `json:"formerly,omitempty"`
}

// Options selects what to include. The zero value exports everything.
type Options struct {
	// NoBodies drops document and section bodies, leaving metadata, the
	// relationship graph and the section titles. An index page wants this and
	// none of the prose.
	NoBodies bool
}

// Build renders a repository. It never fails: a repository that does not lint
// clean still exports, because refusing would make the command useless exactly
// when someone is part-way through writing something.
func Build(r *repo.Repo, opts Options) Repository {
	out := Repository{Schema: Schema, Name: r.Config().Name}
	for _, d := range r.Documents() {
		out.Documents = append(out.Documents, document(r, d, opts))
	}
	if entries, ok := r.Glossary(); ok {
		for _, e := range entries {
			out.Glossary = append(out.Glossary, Term{
				Name:       e.Term,
				Anchor:     e.Anchor,
				Definition: strings.Join(e.Paragraphs, "\n\n"),
				Formerly:   e.Formerly,
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

		Depends:   list(fm.Depends),
		Updates:   list(fm.Updates),
		Obsoletes: list(fm.Obsoletes),
		Includes:  list(fm.Includes),

		UpdatedBy:    list(d.UpdatedBy),
		ObsoletedBy:  list(d.ObsoletedBy),
		DependedOnBy: list(d.DependedOnBy),
		IncludedIn:   list(d.IncludedIn),

		EffectivelyObsolete: d.EffectivelyObsolete,
		Implemented:         d.Implemented,
		Stale:               d.Stale,

		// Initialised rather than left nil so every one of these is an
		// array in the JSON. A spec page has no required sections and a
		// document may have no links; a consumer should not have to handle
		// both an empty array and null for the same field.
		RequiredSections: []RequiredSection{},
		Sections:         map[string]Section{},
		Links:            []Link{},
	}
	if !opts.NoBodies {
		out.Body = d.Body
	}

	anchors := map[string]string{} // title -> anchor, first occurrence
	for _, h := range d.Headings() {
		if h.Level != 2 {
			continue
		}
		if _, seen := anchors[h.Text]; !seen {
			anchors[h.Text] = h.Anchor
		}
		section := Section{Title: h.Text}
		if !opts.NoBodies {
			for _, s := range d.Sections(h.Text) {
				if s.Heading.Anchor == h.Anchor {
					section.Body = s.Text
					break
				}
			}
		}
		out.Sections[h.Anchor] = section
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

// link resolves a destination against the document holding it, so a consumer
// rewriting URLs does not have to know where the document sits.
func link(r *repo.Repo, d *repo.Document, l repo.Link) Link {
	out := Link{Text: l.Text, Destination: l.Target, Line: l.Line}
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
