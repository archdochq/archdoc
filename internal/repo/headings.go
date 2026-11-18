package repo

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"archdoc.dev/internal/markdown"
)

// MaxBody is the length in bytes past which a body is not read. goldmark
// reads some shapes in time growing with the square of their length, and at
// this length the slowest of them takes about half a second.
const MaxBody = 131072

// maxBody is MaxBody, which a test of a current reader's time raises to read
// the body it times.
var maxBody = MaxBody

// readHeadings reads the body's outline for its headings and their anchors,
// or marks the body unread where it is longer than MaxBody.
func (d *Document) readHeadings() {
	if len(d.Body) > maxBody {
		d.unread = true
		return
	}
	d.headings, d.anchors = headingsOf(d.Body, d.BodyLine)
}

// Read reports whether the body was read: false where it is longer than
// MaxBody, when every reader finds nothing in it.
func (d *Document) Read() bool { return !d.unread }

// InHeading reports whether the file's line number holds a top-level
// heading, a setext heading's underline included. An edit inside one changes
// the text the section rules compare.
func (d *Document) InHeading(number int) bool {
	i := number - d.BodyLine
	// The headings are in document order, so the one that could hold the line
	// is the last starting at or before it.
	at := sort.Search(len(d.headings), func(j int) bool { return d.headings[j].index > i }) - 1
	return at >= 0 && i <= d.headings[at].last
}

// Unread lists the paths of the documents whose body is not read, which a
// command reading every document's links cannot see into.
func (r *Repo) Unread() []string {
	var paths []string
	for _, d := range r.Documents() {
		if !d.Read() {
			paths = append(paths, d.Path)
		}
	}
	return paths
}

// text is the body as its readers read it, which is nothing where it is not
// read.
func (d *Document) text() string {
	if d.unread {
		return ""
	}
	return d.Body
}

// headingsOf parses body and returns its top-level headings, which bound
// sections, and the anchor of every heading, in the order GitHub assigns them.
func headingsOf(body string, bodyLine int) (top []Heading, anchors []string) {
	doc := markdown.Parse([]byte(body))
	var starts []int
	for i := range len(body) {
		if body[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	// lineOf is the index of the body line holding offset at.
	lineOf := func(at int) int { return sort.SearchInts(starts, at+1) }

	numbers := footnoteNumbers(doc)
	topLevel := map[*markdown.Node]bool{}
	for _, n := range doc.Children {
		topLevel[n] = true
	}
	taken := map[string]int{}
	for _, n := range anchorOrder(doc, numbers) {
		var rendered strings.Builder
		render(&rendered, n, numbers)
		a := anchor(rendered.String(), taken)
		base := anchor(rendered.String(), map[string]int{})
		anchors = append(anchors, a)
		if !topLevel[n] {
			continue
		}
		top = append(top, Heading{
			Level:  n.Level,
			Text:   headingText(body, n),
			Line:   bodyLine + lineOf(n.Start),
			Anchor: a,
			base:   base,
			index:  lineOf(n.Start),
			last:   lineOf(n.End - 1),
		})
	}
	return top, anchors
}

// footnoteNumbers numbers each footnote label in the order it is first
// referenced, as GitHub numbers the references it renders.
func footnoteNumbers(doc *markdown.Node) map[string]int {
	numbers := map[string]int{}
	markdown.Walk(doc, func(n *markdown.Node) bool {
		if n.Type == markdown.FootnoteReference {
			if _, ok := numbers[n.Label]; !ok {
				numbers[n.Label] = len(numbers) + 1
			}
		}
		return true
	})
	return numbers
}

// anchorOrder lists the headings in the order GitHub renders them, which is
// the order their anchors are assigned: every heading outside a footnote
// definition, then those of each definition in the order its label is first
// referenced. A definition that nothing references is not rendered.
func anchorOrder(doc *markdown.Node, numbers map[string]int) []*markdown.Node {
	var order []*markdown.Node
	definitions := map[string]*markdown.Node{}
	markdown.Walk(doc, func(n *markdown.Node) bool {
		switch n.Type {
		case markdown.FootnoteDefinition:
			if _, ok := definitions[n.Label]; !ok {
				definitions[n.Label] = n
			}
			return false
		case markdown.Heading:
			order = append(order, n)
		}
		return true
	})
	labels := make([]string, len(numbers))
	for label, number := range numbers {
		labels[number-1] = label
	}
	for _, label := range labels {
		if def := definitions[label]; def != nil {
			markdown.Walk(def, func(n *markdown.Node) bool {
				if n.Type == markdown.Heading {
					order = append(order, n)
				}
				return n.Type != markdown.FootnoteDefinition || n == def
			})
		}
	}
	return order
}

// headingText is a heading's content, its lines joined by a line feed, with
// each HTML comment in it replaced by spaces and its ends trimmed.
func headingText(body string, n *markdown.Node) string {
	var comments []markdown.Span
	markdown.Walk(n, func(p *markdown.Node) bool {
		if p.Type == markdown.RawHTML && strings.HasPrefix(p.Text, "<!--") {
			comments = append(comments, p.Span)
		}
		return true
	})
	lines := make([]string, len(n.Lines))
	for i, l := range n.Lines {
		text := []byte(body[l.Start:l.End])
		for _, c := range comments {
			if c.Start < l.End && c.End > l.Start {
				blank(text, max(c.Start, l.Start)-l.Start, min(c.End, l.End)-l.Start)
			}
		}
		lines[i] = strings.TrimRight(string(text), "\r\n")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// render writes the text GitHub renders n's pieces to: a text as resolved, a
// code span's content, a link's text, an autolink's address as written, a
// footnote reference's number, and nothing for an image or raw HTML.
func render(b *strings.Builder, n *markdown.Node, numbers map[string]int) {
	for _, p := range n.Children {
		switch {
		case p.Type == markdown.Text, p.Type == markdown.CodeSpan, p.AutoLink:
			b.WriteString(p.Text)
		case p.Type == markdown.LineBreak:
			b.WriteByte('\n')
		case p.Type == markdown.FootnoteReference:
			b.WriteString(strconv.Itoa(numbers[p.Label]))
		case p.Type == markdown.Image, p.Type == markdown.RawHTML:
		default:
			render(b, p, numbers)
		}
	}
}

// HeadingSurvives reports whether text written as a heading reads back as
// itself. A trailing hash is a closing sequence and trailing whitespace is
// discarded, so a title carrying either produces an H1 that no longer matches
// the front matter, and a glossary term that can never be found by the name
// that was typed.
func HeadingSurvives(text string) bool {
	top, _ := headingsOf("# "+text, 1)
	return len(top) == 1 && top[0].Text == text
}

// Anchor is the fragment GitHub would generate for a heading standing on its
// own. Headings inside a document take their anchors from a walk that numbers
// repeats, so this is only for naming the anchor a heading would have had when
// the heading is not there to ask.
func Anchor(text string) string {
	if _, anchors := headingsOf("# "+text, 1); len(anchors) == 1 {
		return anchors[0]
	}
	return anchor(text, map[string]int{})
}

// anchor slugs a heading's rendered text the way GitHub does: lowercased,
// keeping letters, marks, decimal digits, letter numbers, connector
// punctuation and hyphens, each space a hyphen, and a numeric suffix when the
// same anchor has already been used in this document. taken is updated in
// place.
//
// Keeping only IsLetter and IsDigit dropped the marks of Devanagari and the
// letter numbers and connector punctuation GitHub keeps. L17 then rejected
// links that work and passed ones that 404.
func anchor(rendered string, taken map[string]int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(rendered) {
		switch {
		case unicode.IsLetter(r), unicode.IsMark(r), unicode.IsDigit(r), unicode.Is(unicode.Nl, r), unicode.Is(unicode.Pc, r), r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	base := b.String()

	// GitHub re-checks the suffixed result against the anchors already used, so
	// "Foo", "Foo-1", "Foo" yields foo, foo-1, foo-2 and not a second foo-1.
	// taken records both the next suffix to try for a base and the anchors that
	// are spoken for; they are the same question.
	for n := taken[base]; ; n++ {
		candidate := base
		if n > 0 {
			candidate = fmt.Sprintf("%s-%d", base, n)
		}
		if _, clash := taken[candidate]; clash {
			continue
		}
		taken[base] = n + 1
		taken[candidate] = 1
		return candidate
	}
}
