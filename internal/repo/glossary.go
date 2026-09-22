package repo

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// GlossaryPage is the spec page name the glossary always has.
const GlossaryPage = "glossary"

// GlossaryEntry is one term, read from its file under term/.
type GlossaryEntry struct {
	Term   string
	Anchor string
	// Path is the term's file, relative to root and slash-separated.
	Path string
	// NamedBy is the document that introduced the term, or empty.
	NamedBy string
	// Line is 1-based within the file, at the term's heading.
	Line int
	// Paragraphs holds the entry's prose. A well-formed entry has exactly one;
	// L15 reports the rest.
	Paragraphs []string
	// Formerly holds the previous names recorded by `Formerly *Old Term*.`
	// lines, which are not paragraphs.
	Formerly []string
}

// Glossary lists every term, in ascending case-insensitive order of the term,
// which is the order GLOSSARY.md is generated in. ok is false when the
// repository has no term directory, which is permitted: a repository need not
// have a glossary at all.
//
// Terms are files rather than sections of one page, so uniqueness is the
// filesystem's answer and ordering is decided here rather than checked.
func (r *Repo) Glossary() (entries []GlossaryEntry, ok bool) {
	if _, err := os.Stat(filepath.Join(r.config.RootDir(), string(TypeTerm))); err != nil {
		return nil, false
	}
	entries = []GlossaryEntry{}
	for _, d := range r.Documents() {
		if d.Type == TypeTerm {
			entries = append(entries, d.termEntry())
		}
	}
	slices.SortFunc(entries, func(a, b GlossaryEntry) int {
		if c := strings.Compare(strings.ToLower(a.Term), strings.ToLower(b.Term)); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	return entries, true
}

// termEntry reads one term file. The term is the front matter title rather
// than the heading, because the title is what the filename and every lookup
// key derive from; L10 already holds the two to each other.
func (d *Document) termEntry() GlossaryEntry {
	e := GlossaryEntry{
		Term:     d.FrontMatter.Title,
		Path:     d.Path,
		NamedBy:  d.FrontMatter.NamedBy,
		Formerly: d.FrontMatter.Formerly,
	}
	e.Anchor = Anchor(e.Term)
	for i, h := range d.headings {
		if h.Level == 1 {
			e.Line = h.Line
			e.Paragraphs = entryBlocks(d.sectionAt(i).masked)
			break
		}
	}
	return e
}

// Term finds a glossary entry by name, case-insensitively as uniqueness is
// defined. It is a map lookup rather than a scan, because link asks once per
// candidate on every prose line.
func (r *Repo) Term(name string) (GlossaryEntry, bool) {
	if r.terms == nil {
		r.terms = map[string]GlossaryEntry{}
		if entries, ok := r.Glossary(); ok {
			for _, e := range entries {
				key := strings.ToLower(e.Term)
				if _, seen := r.terms[key]; !seen {
					r.terms[key] = e
				}
			}
		}
	}
	entry, ok := r.terms[strings.ToLower(name)]
	return entry, ok
}

// BlankLinePattern separates paragraphs. It matches CRLF as well as LF, because
// a file saved on Windows would otherwise read as one unbroken paragraph and
// L15 would pass anything. Exported because internal/glossary refuses a
// definition that contains one.
var BlankLinePattern = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)

// entryBlocks splits a definition into paragraphs. HTML comments and
// whitespace count as nothing, as they do everywhere else.
func entryBlocks(masked string) (paragraphs []string) {
	// The masked text comes from the document's own walk, not from a fresh
	// scan of this fragment: a comment opened before the definition and closed
	// inside it is already blanked there, and re-scanning from a clean state
	// would read the stray closing marker as a paragraph.
	for _, block := range BlankLinePattern.Split(masked, -1) {
		if trimmed := strings.TrimSpace(block); trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs
}
