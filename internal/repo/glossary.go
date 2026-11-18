package repo

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"archdoc.dev/internal/markdown"
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
	// the file as written, with LF line endings. It holds the blocks L15
	// reports too, rather than dropping them unseen.
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
			e.Paragraphs, e.Other = d.entryBlocks(h.last + 1)
			e.Definition = d.definition(h.last + 1)
			for _, b := range d.blocksIn(0, h.index) {
				if !isCommentBlock(b) {
					e.Above = d.BodyLine + d.lineAt(b.Start)
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

// entryBlocks reads the top-level blocks from body line from to the end of
// the body, subsections included: each paragraph as written with its
// comments removed, and a count of the other blocks, comment blocks and link
// reference definitions passed over.
func (d *Document) entryBlocks(from int) (paragraphs []string, other int) {
	for _, b := range d.blocksIn(from, len(d.bodyLines())) {
		switch {
		case b.Type == markdown.Paragraph:
			paragraphs = append(paragraphs, paragraphText(d.Body, b))
		case !passedOver(b):
			other++
		}
	}
	return paragraphs, other
}

// paragraphText is a paragraph's lines as written, from the start of each
// line, joined by a line feed, with each HTML comment in it removed, a line
// left holding nothing dropped, and its ends trimmed.
func paragraphText(body string, p *markdown.Node) string {
	var comments []markdown.Span
	markdown.Walk(p, func(n *markdown.Node) bool {
		if n.Type == markdown.RawHTML && strings.HasPrefix(n.Text, "<!--") {
			comments = append(comments, n.Span)
		}
		return true
	})
	var lines []string
	next := 0 // the first comment that may reach this line, as they are in order
	for _, l := range p.Lines {
		from := strings.LastIndexAny(body[:l.Start], "\r\n") + 1
		for next < len(comments) && comments[next].End <= from {
			next++
		}
		// Within the line, a comment is cut from its first byte that is not
		// a space or a tab to its last, so the spaces around it are kept.
		var b strings.Builder
		at := from
		for _, c := range comments[next:] {
			if c.Start >= l.End {
				break
			}
			start, end := max(c.Start, from), min(c.End, l.End)
			for start < end && (body[start] == ' ' || body[start] == '\t') {
				start++
			}
			for end > start && (body[end-1] == ' ' || body[end-1] == '\t') {
				end--
			}
			if start < end {
				b.WriteString(body[at:max(at, start)])
				at = max(at, end)
			}
		}
		b.WriteString(body[at:l.End])
		if line := strings.TrimRight(b.String(), "\r\n"); strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// definition is the body from line from to its end as written, comments
// included, except a comment block left open at the end, which on the
// generated page would run on over the terms after it. A line left blank is
// written empty, and a lone carriage return, which CommonMark reads as a line
// ending, ends a line.
func (d *Document) definition(from int) string {
	lines := d.bodyLines()
	if from >= len(lines) {
		return ""
	}
	start := 0
	if from > 0 {
		start = d.starts[from-1]
	}
	end := len(d.Body)
	if blocks := d.blocksIn(from, len(lines)); len(blocks) > 0 {
		if last := blocks[len(blocks)-1]; isCommentBlock(last) && !last.Closed {
			end = last.Start
		}
	}
	text := strings.ReplaceAll(d.Body[start:max(start, end)], "\r\n", "\n")
	out := strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n")
	for i, line := range out {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
		}
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}

// opensCommentBlock reports whether a line, read outside any container, begins
// with a comment at most three spaces in.
func opensCommentBlock(line string) bool {
	t := strings.TrimLeft(line, " ")
	return len(line)-len(t) <= 3 && strings.HasPrefix(t, commentOpen)
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
