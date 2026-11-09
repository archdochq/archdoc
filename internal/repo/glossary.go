package repo

import (
	"iter"
	"os"
	"path/filepath"
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
	// Paragraphs holds the definition's paragraphs as written, with comments
	// removed. Other counts the blocks beneath the heading that are not
	// paragraphs, and Above is the line of the first prose above it, or 0. A
	// well-formed entry has exactly one paragraph and nothing else; L15
	// reports the rest.
	Paragraphs []string
	Other      int
	Above      int
	// Definition is what is published: every line from the H1 to the end of
	// the file as written, comments cut out, with LF line endings. It holds
	// the blocks L15 reports too, rather than dropping them unseen.
	Definition string
	// Formerly is the term file's `formerly`: its previous names, oldest
	// first.
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
			entries = append(entries, d.TermEntry())
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

// TermEntry reads one term file. The term is the front matter title rather
// than the heading, because the title is what the filename and every lookup
// key derive from; L10 already holds the two to each other.
func (d *Document) TermEntry() GlossaryEntry {
	e := GlossaryEntry{
		Term:     d.FrontMatter.Title,
		Path:     d.Path,
		NamedBy:  d.FrontMatter.NamedBy,
		Formerly: d.FrontMatter.Formerly,
	}
	e.Anchor = Anchor(e.Term)
	for _, h := range d.headings {
		if h.Level == 1 {
			e.Line = h.Line
			e.Paragraphs, e.Other = d.entryBlocks(h.index + 1)
			e.Definition = d.definition(h.index + 1)
			masked := d.maskedLines()
			for j := range h.index {
				if strings.TrimSpace(masked[j]) != "" {
					e.Above = d.BodyLine + j
					break
				}
			}
			break
		}
	}
	return e
}

// Defined reports whether the entry is a single paragraph of definition and
// nothing else.
func (e GlossaryEntry) Defined() bool {
	return len(e.Paragraphs) == 1 && e.Other == 0 && e.Above == 0
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

// entryBlocks reads the blocks from body line from to the end of the body,
// subsections included, from the document's own walk: a comment opened before
// the definition and closed inside it is already blanked there. A line holding
// only a comment, like a blank one, separates blocks.
//
// A paragraph runs over the lines that continue it; a heading, a thematic
// break, a list, a blockquote, a table, or a code or HTML block is another
// block. It ends at a setext underline and a table's header row as well, as
// the block walk's does, and a lone carriage return ends a line.
func (d *Document) entryBlocks(from int) (paragraphs []string, other int) {
	lines, masked := d.bodyLines(), d.maskedLines()
	// What holds the lines of the current block: none after a blank line.
	const (
		none = iota
		paragraph
		container
		leaf
	)
	current, kind := none, blockProse
	// comment is whether a comment that began a line is still open.
	comment := false
	var text []string
	// Each line split at its lone carriage returns, so a header row can be
	// read with the line after it.
	type piece struct {
		raw, masked string
		line        int
	}
	var pieces []piece
	for i := from; i < len(lines); i++ {
		for raw, m := range crLines(strings.TrimSuffix(lines[i], "\r"), strings.TrimSuffix(masked[i], "\r")) {
			pieces = append(pieces, piece{raw, m, i})
		}
	}
	// A table's header row is read as one outside any container, as the
	// definition is, in the paragraph the line continues if any.
	var outside blockWalk
	end := func() {
		if current == paragraph {
			paragraphs = append(paragraphs, strings.TrimSpace(strings.Join(text, "\n")))
		}
		current, text = none, nil
	}
	for p, piece := range pieces {
		raw, m, i := piece.raw, piece.masked, piece.line
		outside.paragraph = current == paragraph
		// A comment beginning a line opens an HTML block, as CommonMark
		// has it, and the text after the marker closing it is the
		// block's. Inside a code block the marker is code.
		if t := strings.TrimLeft(raw, " "); d.literal[i] == blockProse && len(raw)-len(t) <= 3 && strings.HasPrefix(t, commentOpen) {
			comment = true
		}
		html := comment
		comment = comment && !strings.Contains(raw, commentClose)
		switch {
		case d.literal[i] != blockProse:
			if current != leaf || kind != d.literal[i] {
				end()
				current, kind = leaf, d.literal[i]
				other++
			}
		// A line is blank or a break as written, so one a comment holds whole
		// inside a paragraph continues it. A heading is the section parse's.
		case strings.TrimSpace(raw) == "" || strings.TrimSpace(m) == "" && !d.continues[i]:
			end()
		case html || IsHeading(m) || thematicPattern.MatchString(raw) || setextPattern.MatchString(raw):
			end()
			other++
		case d.quoted[i] || p+1 < len(pieces) && outside.tableStarts(raw, pieces[p+1].raw) || interruptsWithList(m, current == paragraph):
			if current != container {
				end()
				current = container
				other++
			}
		case current == none || current == leaf:
			end()
			current = paragraph
			fallthrough
		case current == paragraph:
			if strings.TrimSpace(m) != "" {
				text = append(text, withoutComments(raw, m))
			}
		default:
			// A line continuing a list or a blockquote lazily stays in it.
		}
	}
	end()
	return paragraphs, other
}

// crLines splits a line as written and its masked twin at each lone carriage
// return, which CommonMark reads as a line ending, at the same offsets in
// both.
func crLines(raw, masked string) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for {
			i := strings.IndexByte(raw, '\r')
			if i < 0 {
				yield(raw, masked)
				return
			}
			if !yield(raw[:i], masked[:i]) {
				return
			}
			raw, masked = raw[i+1:], masked[i+1:]
		}
	}
}

// definition is the body from line from to its end as written, with the
// comments cut out. A line left blank is written empty, and a lone carriage
// return, which CommonMark reads as a line ending, as a line feed.
func (d *Document) definition(from int) string {
	lines, masked := d.bodyLines(), d.maskedLines()
	out := make([]string, 0, len(lines)-from)
	for i := from; i < len(lines); i++ {
		line := withoutComments(strings.TrimSuffix(lines[i], "\r"), masked[i])
		if strings.TrimSpace(line) == "" {
			// A line a comment holds whole inside a paragraph leaves no gap
			// in it, as a blank line would.
			if d.continues[i] {
				continue
			}
			line = ""
		}
		out = append(out, strings.ReplaceAll(line, "\r", "\n"))
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}

// interruptsWithList reports whether a line opens a list item. Within a
// paragraph only a bullet or an ordered item numbered 1 does, as CommonMark
// has it, so a line beginning "2025. " continues the sentence before it.
func interruptsWithList(line string, inParagraph bool) bool {
	m := listMarkerPattern.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return !inParagraph || m[3] == "" || m[3] == "1"
}

// withoutComments is a line as written with the comments its masked twin
// blanks cut out. A comment starts and ends with a byte masking changed, so
// each blanked run is cut from its first changed byte to its last, and the
// spaces written around it are kept.
func withoutComments(line, masked string) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		if line[i] == masked[i] {
			b.WriteByte(line[i])
			i++
			continue
		}
		last := i
		for j := i; j < len(line) && masked[j] == ' '; j++ {
			if line[j] != ' ' {
				last = j
			}
		}
		i = last + 1
	}
	return b.String()
}
