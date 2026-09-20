package repo

import (
	"regexp"
	"strings"
)

// GlossaryPage is the spec page name the glossary always has.
const GlossaryPage = "glossary"

// FormerlyPattern matches the line a rename leaves behind. It is not a
// paragraph, and several may accumulate as a term is renamed repeatedly.
//
// Exported because internal/glossary asks the same question when it decides
// where to put a new note. A second spelling of the rule there could drift from
// this one with nothing to notice, which is why the link patterns are exported
// too.
var FormerlyPattern = regexp.MustCompile(`^Formerly \*(.+)\*\.$`)

// GlossaryEntry is one term in spec/glossary.md.
type GlossaryEntry struct {
	Term   string
	Anchor string
	// Line is 1-based within the file, at the term's heading.
	Line int
	// End is the 1-based line just past the entry: the next heading of level
	// two or less, or 0 when the entry runs to the end of the file.
	//
	// It is recorded here rather than derived from the next entry's Line,
	// because those two answers differ whenever an ordinary H1 section sits
	// among the entries, and the difference deleted that section.
	End int
	// Paragraphs holds the entry's prose. A well-formed entry has exactly one;
	// L15 reports the rest.
	Paragraphs []string
	// Formerly holds the previous names recorded by `Formerly *Old Term*.`
	// lines, which are not paragraphs.
	Formerly []string
}

// Glossary parses spec/glossary.md. ok is false when the page does not exist,
// which is permitted: a repository need not have one until a term is added.
func (r *Repo) Glossary() (entries []GlossaryEntry, ok bool) {
	d := r.ByPage(GlossaryPage)
	if d == nil {
		return nil, false
	}
	return d.glossaryEntries(), true
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

// GlossaryIn parses glossary entries out of arbitrary source, so that a
// rewritten page can be read back without being written to disk first.
func GlossaryIn(source []byte) ([]GlossaryEntry, bool) {
	d := &Document{Type: TypeSpec, Page: GlossaryPage, Source: source}
	block, body, line, ok := splitFrontMatter(source)
	if !ok {
		return nil, false
	}
	d.FrontMatter, _ = parseFrontMatter(TypeSpec, block)
	d.Body, d.BodyLine = string(body), line
	d.headings = parseHeadings(d.Body, d.BodyLine)
	return d.glossaryEntries(), true
}

// glossaryEntries reads the H2 entries of a page. Anything before the first H2
// is preamble and is ignored. Sections are taken by heading index, so two
// entries sharing a term are read apart.
func (d *Document) glossaryEntries() []GlossaryEntry {
	if d.glossary != nil {
		return d.glossary
	}
	entries := []GlossaryEntry{}

	for i, h := range d.headings {
		if h.Level != 2 {
			continue
		}
		paragraphs, formerly := entryBlocks(d.sectionAt(i).masked)
		end := 0
		for _, next := range d.headings[i+1:] {
			if next.Level <= h.Level {
				end = next.Line
				break
			}
		}
		entries = append(entries, GlossaryEntry{
			Term:       h.Text,
			Anchor:     h.Anchor,
			Line:       h.Line,
			End:        end,
			Paragraphs: paragraphs,
			Formerly:   formerly,
		})
	}
	d.glossary = entries
	return entries
}

// BlankLinePattern separates paragraphs. It matches CRLF as well as LF, because
// a file saved on Windows would otherwise read as one unbroken paragraph and
// L15 would pass anything. Exported for the same reason as FormerlyPattern:
// internal/glossary refuses a definition that contains one.
var BlankLinePattern = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)

// entryBlocks splits an entry into its paragraphs and its recorded former
// names. Formerly lines are pulled out line by line rather than as blocks, so
// that renames accumulated on consecutive lines do not read as prose. HTML
// comments and whitespace count as nothing, as they do everywhere else.
func entryBlocks(masked string) (paragraphs, formerly []string) {
	// The masked text comes from the document's own walk, not from a fresh
	// scan of this fragment: a comment opened before the entry and closed
	// inside it is already blanked there, and re-scanning from a clean state
	// would read the stray closing marker as a paragraph.
	var kept []string
	for _, line := range strings.Split(masked, "\n") {
		if m := FormerlyPattern.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			formerly = append(formerly, m[1])
			continue
		}
		kept = append(kept, line)
	}

	for _, block := range BlankLinePattern.Split(strings.Join(kept, "\n"), -1) {
		if trimmed := strings.TrimSpace(block); trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs, formerly
}
