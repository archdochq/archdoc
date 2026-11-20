package repo

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"

	"archdoc.dev/internal/markdown"
)

// Heading is one markdown heading in a document body.
type Heading struct {
	Level int
	Text  string
	// Line is 1-based within the file.
	Line int
	// Anchor is the fragment GitHub would generate for this heading.
	Anchor string
	// index is the heading's first line in the body's lines, used to find
	// where the section before it ends, and last its last line, a setext
	// heading's underline, after which its own body starts. A heading after a
	// lone carriage return shares a line with the block before it.
	index, last int
	// base is the anchor before any suffix, which a question's is compared
	// with.
	base string
	// span is the heading's bytes in the body, and content its content's,
	// from the first byte of its first line to the last of its last.
	span, content markdown.Span
}

// Section is a heading and everything under it, up to the next heading of equal
// or higher level.
type Section struct {
	Heading Heading
	// Text is the section's body as written, which is what a rewrite needs.
	Text string
	// masked is the same lines with each comment in an HTML block blanked,
	// the outline's comment-masked text, which Empty reads.
	masked string
}

// Sources names what a document was checked or reconstructed against. Required
// on every ref, and on any backfilled document.
const Sources = "Sources"

// RejectionRationale is the section a rejected document gains, and which no
// other document may carry.
const RejectionRationale = "Rejection rationale"

// wikiLinkPattern matches a [[...]] link, its name the first group.
var wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// WikiLinkPattern matches a [[...]] link, its name the first group.
var WikiLinkPattern = wikiLinkPattern

// Empty reports whether a section holds nothing but whitespace and HTML
// comments. This is what makes a comment count as nothing throughout: the same
// definition drives L14 and the pre-freeze gate.
func (s Section) Empty() bool { return strings.TrimSpace(s.masked) == "" }

// RequiredSections lists the H2 sections a type must carry, in the order they
// must appear. Other sections are permitted between or after them.
func RequiredSections(t Type) []string {
	switch t {
	case TypeRFC:
		return []string{
			"Abstract", "Motivation", "Proposal", "Alternatives considered",
			"Backwards compatibility",
		}
	case TypeADR:
		return []string{"Context", "Decision", "Alternatives", "Consequences"}
	case TypeRef:
		return []string{Sources}
	default:
		return nil
	}
}

// RequiredSections is RequiredSections for the type, plus Sources when the
// document is backfilled. Both L08, which checks a section is present, and L14,
// which checks it is not empty, read this one list, so they cannot disagree
// about what a document must carry.
func (d *Document) RequiredSections() []string {
	required := RequiredSections(d.Type)
	if !d.FrontMatter.Backfilled.IsZero() && !slices.Contains(required, Sources) {
		required = append(required, Sources)
	}
	// A rejected document's rationale is required, so it is held to the same
	// standard as every other required section: present, and not empty. Without
	// it a document freezes with the template's "Why?" prompt as its permanent
	// answer, which is exactly what the prompt exists to prevent.
	if d.FrontMatter.Status == StatusRejected {
		required = append(required, RejectionRationale)
	}
	return required
}

// ClosingSection is the section that must be a document's last H2, and whether
// it has one at all. Most documents do not: PROCESS.md states a closing section
// only for a ref and for a rejected document, and extra sections may otherwise
// sit after the required ones.
//
// It is the last entry of RequiredSections when that entry is a closing kind,
// which is what keeps this rule and the relative-order rule from contradicting
// each other. They did: Sources was required last and the rationale required
// after Sources, so a backfilled rejected document could satisfy neither, and
// `archdoc new --backfill --status=rejected` wrote one that could never lint
// clean and that L11 then forbade anyone repairing. PROCESS.md's Document
// structure section calls the rationale final and its Backfilling section asks
// only that a backfilled document carry Sources, so the rationale closes the
// document when both are required.
func (d *Document) ClosingSection() (string, bool) {
	required := d.RequiredSections()
	if len(required) == 0 {
		return "", false
	}
	switch last := required[len(required)-1]; last {
	case Sources, RejectionRationale:
		return last, true
	}
	return "", false
}

// SectionTitles lists the document's H2 headings in order.
func (d *Document) SectionTitles() []string {
	var titles []string
	for _, h := range d.headings {
		if h.Level == 2 {
			titles = append(titles, h.Text)
		}
	}
	return titles
}

// LinkDestination renders a path as a markdown link destination, percent-
// encoding every character that would otherwise end the destination, the link,
// or the table cell around it.
//
// One function, because the index and the link rewriter each rendered
// destinations their own way and only one of them escaped anything. Encoding
// rather than escaping, because the two interact: the angle-bracket form this
// replaces turned a path's ">" into "\>", and a backslash already in the path
// then paired with that backslash and let the ">" close the destination early.
// An ordinary path contains none of these and comes through unchanged, and
// Document.Links decodes on the way back in, so the two are inverses.
func LinkDestination(path string) string {
	const encode = "%<>()#|\\ "
	var b strings.Builder
	for _, r := range path {
		switch {
		case r < 0x20 || r == 0x7f || strings.ContainsRune(encode, r):
			fmt.Fprintf(&b, "%%%02X", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// EscapeMarkdown makes arbitrary text safe to place inside generated markdown:
// a table cell, or a link label. A title is free text, and a document called
// "Pwned](https://evil.example) x" would otherwise become a live hyperlink in
// another document's body and in the index, with lint reporting nothing.
//
// The backslash goes first, or escaping the others would double it.
func EscapeMarkdown(text string) string {
	replaced := strings.NewReplacer(
		`\`, `\\`,
		"[", `\[`,
		"]", `\]`,
		"|", `\|`,
		// Raw HTML passes straight through rendered markdown, so a title
		// carrying a tag would put live markup into a generated file.
		"<", "&lt;",
		">", "&gt;",
	).Replace(text)
	// A line break would end a table row wherever this lands.
	return strings.Join(strings.Fields(replaced), " ")
}

// Headings returns the body's top-level headings, in order.
func (d *Document) Headings() []Heading { return d.headings }

// H1 returns the first heading when it is an H1.
func (d *Document) H1() (Heading, bool) {
	if len(d.headings) == 0 || d.headings[0].Level != 1 {
		return Heading{}, false
	}
	return d.headings[0], true
}

// Description is a group's body after its H1, as written, or the whole body
// where the first heading is not an H1. It is what was parsed, whatever L10
// reports, so a group whose heading is wrong still publishes its prose.
func (d *Document) Description() string {
	h1, ok := d.H1()
	if !ok {
		return d.text()
	}
	return strings.Join(d.bodyLines()[h1.last+1:], "\n")
}

// sectionAt returns the heading at index i and everything beneath it, up to the
// next heading of equal or higher level. Callers that already know which
// heading they mean use this rather than Section, which searches by title and
// would hand every duplicate the first one's body.
func (d *Document) sectionAt(i int) Section {
	lines, masked := d.bodyLines(), d.commentMaskedLines()
	h := d.headings[i]
	end := d.sectionEnd(i)
	return Section{
		Heading: h,
		Text:    strings.Join(lines[h.last+1:end], "\n"),
		masked:  strings.Join(masked[h.last+1:end], "\n"),
	}
}

// nextSection is the index of the heading that ends heading i's section, the
// next of equal or higher level, or -1 where the section runs to the end.
func (d *Document) nextSection(i int) int {
	for j := i + 1; j < len(d.headings); j++ {
		if d.headings[j].Level <= d.headings[i].Level {
			return j
		}
	}
	return -1
}

// sectionEnd is the body line index at which heading i's section ends.
func (d *Document) sectionEnd(i int) int {
	if next := d.nextSection(i); next >= 0 {
		// A heading after a lone carriage return starts on the line the one
		// before it ends on, which leaves that one's section empty.
		return max(d.headings[next].index, d.headings[i].last+1)
	}
	return len(d.bodyLines())
}

// Entry is one heading and the lines beneath it, up to the next heading of any
// level, whatever that heading's level is.
//
// A Section is a different unit and stays one: it is an H2 and everything under
// it, subsections included, which is what the required-section rules mean by the
// word. An entry's body holds no other entry's prose, so walking the entries
// reads every line of a body exactly once.
type Entry struct {
	Heading Heading
	// Text is the body as written, from the line after the heading's last, a
	// setext heading's underline, to the line before the next one.
	Text string
}

// Contents is the document as a flat list of its headings, in document order.
func (d *Document) Contents() []Entry {
	lines := d.bodyLines()
	out := make([]Entry, 0, len(d.headings))
	for i, h := range d.headings {
		end := len(lines)
		if i+1 < len(d.headings) {
			end = max(d.headings[i+1].index, h.last+1)
		}
		out = append(out, Entry{Heading: h, Text: strings.Join(lines[h.last+1:end], "\n")})
	}
	return out
}

// Sections returns every H2 with exactly this text.
func (d *Document) Sections(title string) []Section {
	var found []Section
	for i, h := range d.headings {
		if h.Level == 2 && h.Text == title {
			found = append(found, d.sectionAt(i))
		}
	}
	return found
}

func (d *Document) bodyLines() []string {
	if d.lines == nil {
		d.lines = strings.Split(d.text(), "\n")
	}
	return d.lines
}

// commentMaskedLines is the body's comment-masked text: each line as written
// with every byte of an HTML comment the outline holds a space, which a
// section's emptiness is read from.
func (d *Document) commentMaskedLines() []string {
	if d.commentMasked == nil {
		d.commentMasked = strings.Split(string(commentMasked(d.text(), d.outline)), "\n")
	}
	return d.commentMasked
}

// ProseLine is one line of a body that is neither inside a fenced code block
// nor inside an HTML comment.
//
// The two strings are a pair, and which is which matters: a caller decides from
// Masked and applies the edit that follows to Raw, at the same offsets. They are
// named fields rather than adjacent parameters of the same type because
// confusing the two is the single root cause of three defects this package has
// shipped.
type ProseLine struct {
	// Raw is the line as written. Edits are applied to this one.
	Raw string
	// Masked is Raw with every byte that is not prose a space, keeping its
	// length so that a position found here is the same column in Raw.
	// Matches are looked for in this one.
	Masked string
	// Number is the line's 1-based number in the file.
	Number int
}

// Unclosed reports whether the body ends inside a top-level fenced code block
// or HTML block only its end marker closes, a comment block among them. One
// inside a container is not: the blank line and unindented heading a writer
// appends end the container, and the block with it.
//
// A writer splicing at the end of such a body writes inside the block: the
// rejection rationale appended to it would be code, or hidden, and the
// document would still lack the section the command reported writing.
func (d *Document) Unclosed() bool {
	if d.outline == nil {
		return false
	}
	end := len(strings.TrimRight(d.Body, " \t\r\n"))
	for _, n := range d.outline.Children {
		open := n.Type == markdown.CodeBlock && n.Fenced || n.Type == markdown.HTMLBlock && closedByMarker(n.Text)
		if open && !n.Closed && n.End >= end {
			return true
		}
	}
	return false
}

// closedByMarker reports whether an HTML block is of a type only its end
// marker closes: one opened by script, pre, style or textarea, a comment, a
// processing instruction, a declaration or CDATA.
func closedByMarker(html string) bool {
	return markerClosed.MatchString(strings.TrimLeft(html, " \t"))
}

var markerClosed = regexp.MustCompile(`(?i)^(?:<(?:script|pre|style|textarea)(?:[ \t\r\n>]|$)|<!--|<\?|<![A-Za-z]|<!\[CDATA\[)`)

// advance is the column after b, a tab running to the next multiple of four.
func advance(col int, b byte) int {
	if b == '\t' {
		return col + 4 - col%4
	}
	return col + 1
}

// blank writes spaces over out[from:to].
func blank(out []byte, from, to int) {
	for i := from; i < to; i++ {
		out[i] = ' '
	}
}

// LinkType is the form a link was written in. An image resolves like any other
// link and renders as something else entirely, so a consumer rewriting
// destinations has to tell them apart.
type LinkType string

const (
	LinkInline     LinkType = "inline"
	LinkImage      LinkType = "image"
	LinkDefinition LinkType = "definition"
	// LinkReference is a use of a definition, which only References returns.
	LinkReference LinkType = "reference"
)

// Link is a relative markdown link found in a body.
type Link struct {
	Text string
	// Target is the destination with its escapes, entities and
	// percent-encoding decoded, and Written the bytes the file holds, which a
	// consumer rewriting the destination replaces. Path and Fragment are the
	// parts before and after its first #, each decoded on its own, so an
	// encoded # in a filename stays in the path.
	Target   string
	Written  string
	Path     string
	Fragment string
	Type     LinkType
	// Line is 1-based within the file.
	Line int
	// Start and End are the columns of the destination as written, inside any
	// <...>, so a writer can replace it in the line without reparsing it.
	Start, End int
}

// WikiLink is an unexpanded [[...]] reference.
type WikiLink struct {
	Name string
	Line int
	// Start and End are the columns of the whole [[...]] in its line.
	Start, End int
	// Heading is whether the link is in a heading, where link leaves it: the
	// heading's text is what L08, L09, L10 and anchors read.
	Heading bool
	// Embed is whether an unescaped ! comes straight before it, as in
	// Obsidian's ![[X]], where link leaves it: expanded, it would be an image.
	Embed bool
}

// AnchorsIn returns the heading anchors of arbitrary markdown, for checking a
// link into a file that is not itself a document, or nil where its body is
// longer than MaxBody and is not read.
func AnchorsIn(source []byte) []string {
	_, body, _, _ := splitFrontMatter(source)
	// A byte order mark is stripped from a front matter delimiter, and a file
	// with none kept it before its first heading.
	d := &Document{Body: strings.TrimPrefix(string(body), "\uFEFF"), BodyLine: 1}
	d.readHeadings()
	return d.Anchors()
}

// Anchors is every fragment a link into the document can name: each
// heading's anchor, then each explicit one. It is nil where the body is not
// read, as no anchor in it is known, and never nil otherwise.
func (d *Document) Anchors() []string {
	if d.unread {
		return nil
	}
	anchors := make([]string, 0, len(d.allHeadings))
	for _, h := range d.allHeadings {
		anchors = append(anchors, h.Anchor)
	}
	return append(anchors, explicitAnchors(d.outline)...)
}

// anchorTag matches an <a> start tag, its attributes in CommonMark's grammar,
// which is how a Markdown document names a target that is not a heading, and
// anchorAttribute each attribute in it with its value, quoted or not. Read by
// attribute, an id or a name is one only as an attribute's whole name: found
// by pattern anywhere in the tag, data-id="x" and title="id='z'" were taken
// for anchors and id=y, unquoted, was not.
var (
	anchorTag       = regexp.MustCompile(`(?i)<a((?:\s+[a-z_:][a-z0-9_.:-]*(?:\s*=\s*(?:[^"'=<>\x60\s]+|'[^']*'|"[^"]*"))?)*)\s*/?>`)
	anchorAttribute = regexp.MustCompile(`(?i)\s+([a-z_:][a-z0-9_.:-]*)(?:\s*=\s*(?:([^"'=<>\x60\s]+)|'([^']*)'|"([^"]*)"))?`)
)

// tagAnchors is the id and name values of each <a> start tag in text.
func tagAnchors(text string) []string {
	var anchors []string
	for _, tag := range anchorTag.FindAllStringSubmatch(text, -1) {
		for _, a := range anchorAttribute.FindAllStringSubmatch(tag[1], -1) {
			if name := strings.ToLower(a[1]); name != "id" && name != "name" {
				continue
			}
			if value := a[2] + a[3] + a[4]; value != "" {
				anchors = append(anchors, html.UnescapeString(value))
			}
		}
	}
	return anchors
}
