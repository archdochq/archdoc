package repo

import (
	"fmt"
	"html"
	"iter"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Heading is one markdown heading in a document body.
type Heading struct {
	Level int
	Text  string
	// Line is 1-based within the file.
	Line int
	// Anchor is the fragment GitHub would generate for this heading.
	Anchor string
	// index is the heading's position in the body's lines, used to find where
	// its section ends.
	index int
	// written is the heading's content as written, comments and all, which
	// its anchor is rendered from: a comment renders as nothing, and the
	// space beside it as a hyphen.
	written string
}

// Section is a heading and everything under it, up to the next heading of equal
// or higher level.
type Section struct {
	Heading Heading
	// Text is the section's body as written, which is what a rewrite needs.
	Text string
	// masked is the same lines with comments blanked, taken from the
	// document's own walk. Empty reads this one, so that what counts as
	// nothing here is what counts as nothing everywhere else.
	masked string
}

// Sources names what a document was checked or reconstructed against. Required
// on every ref, and on any backfilled document.
const Sources = "Sources"

// RejectionRationale is the section a rejected document gains, and which no
// other document may carry.
const RejectionRationale = "Rejection rationale"

// The patterns anchored at the end of a line accept a carriage return before
// it. A body is split on "\n", so a line of a CRLF file keeps its "\r", which is
// neither a space nor a tab: a link definition went unread and a heading kept
// its closing hashes in a file Git for Windows checked out. The "\r" is the
// line's last byte, so accepting it moves no offset.
var (
	headingPattern  = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*\r?$`)
	fencePattern    = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	linkPattern     = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	closingHashes   = regexp.MustCompile(`[ \t]+#+$`)
	wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)
	// A line, read from behind any quote markers, that ends the paragraph
	// before it and so any link text left open there: a heading, a table row,
	// a thematic break or a setext underline. A list item is the block walk's
	// to say, as whether one interrupts depends on the containers around it.
	blockStartPattern = regexp.MustCompile(
		`^[ \t]*(?:#{1,6}(?:[ \t]|\r?$)|\|)|^ {0,3}(?:=+|-+|(?:[*_-][ \t]*){3,})[ \t]*\r?$`)
	// A reference after a link's closing bracket: [label], or [] to use the
	// text as the label.
	referencePattern = regexp.MustCompile(`^\[([^\[\]]*)\]`)
	// A link reference definition, whose destination is a link target like any
	// other and was checked by nothing.
	//
	// Anchored at both ends, with a non-empty label and an optional title,
	// because a line is only a definition if that is all it is. Without the end
	// anchor a footnote, a numbered citation or an ordinary sentence beginning
	// with a bracketed word was read as a link and reported as a broken one,
	// and the only remedy available to an author was to reword the line. The
	// masking pattern in internal/link may over-match, where the cost is a
	// missed suggestion; deciding that a line IS a link is a stricter question.
	definitionPattern = regexp.MustCompile(
		`^[ \t]{0,3}\[([^\]^][^\]]*)\]:[ \t]*(?:<([^<>\n]*)>|(\S+))(?:[ \t]+(?:"[^"]*"|'[^']*'|\([^)]*\)))?[ \t]*\r?$`)
)

// Patterns for spans a suggestion must never be offered inside: an inline
// link, a reference link, or an unexpanded wiki link. Wrapping text inside any
// of them produces markup that means something else entirely, and in the wiki
// case it hides the very error that was being reported.
var (
	// The same two rules as linkPattern and wikiLinkPattern above, which carry
	// a capture group the maskers do not need. A group costs about 1% on these
	// inputs, so one declaration each is the better trade: they were three
	// separate spellings of two rules and could drift.
	MarkdownLinkPattern  = linkPattern
	ReferenceLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\[[^\]]*\]`)
	WikiLinkPattern      = wikiLinkPattern
	// WikiLinkSubmatchPattern is the same compiled rule under a second name, for
	// a caller that reads the name out rather than only blanking the span. One
	// declaration, as above: the two names mark the two uses.
	WikiLinkSubmatchPattern = wikiLinkPattern
)

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

// IsHeading reports whether a line is an ATX heading. A heading's raw text is a
// section's identity throughout this package and in lint, so a rewriter must
// leave one alone.
func IsHeading(line string) bool { return headingPattern.MatchString(line) }

// HeadingSurvives reports whether text written as a heading reads back as
// itself. A trailing hash is a closing sequence and trailing whitespace is
// discarded, so a title carrying either produces an H1 that no longer matches
// the front matter, and a glossary term that can never be found by the name
// that was typed.
func HeadingSurvives(text string) bool {
	parsed := parseHeadings("# "+text, 1)
	return len(parsed) == 1 && parsed[0].Text == text
}

// Headings returns every heading in the body, in order.
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
		return d.Body
	}
	return strings.Join(d.bodyLines()[h1.Line-d.BodyLine+1:], "\n")
}

// sectionAt returns the heading at index i and everything beneath it, up to the
// next heading of equal or higher level. Callers that already know which
// heading they mean use this rather than Section, which searches by title and
// would hand every duplicate the first one's body.
func (d *Document) sectionAt(i int) Section {
	lines, masked := d.bodyLines(), d.maskedLines()
	h := d.headings[i]
	end := d.sectionEnd(i)
	return Section{
		Heading: h,
		Text:    strings.Join(lines[h.index+1:end], "\n"),
		masked:  strings.Join(masked[h.index+1:end], "\n"),
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
		return d.headings[next].index
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
	// Text is the body as written, from the line after the heading to the line
	// before the next one, so Heading.Line plus one is its first line.
	Text string
}

// Contents is the document as a flat list of its headings, in document order.
func (d *Document) Contents() []Entry {
	lines := d.bodyLines()
	out := make([]Entry, 0, len(d.headings))
	for i, h := range d.headings {
		end := len(lines)
		if i+1 < len(d.headings) {
			end = d.headings[i+1].index
		}
		out = append(out, Entry{Heading: h, Text: strings.Join(lines[h.index+1:end], "\n")})
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
		d.lines = strings.Split(d.Body, "\n")
	}
	return d.lines
}

// maskedLines is the body with every HTML comment blanked, walked once for the
// whole document.
//
// Running scanBody again on a fragment is not the same thing: it starts from a
// clean state, so a comment opened before the fragment and closed inside it
// left a stray "-->" reading as content, and the section holding nothing but
// that comment passed the freeze gate. scanBody was the single definition of
// what a comment is without being the single walk over the document.
func (d *Document) maskedLines() []string {
	if d.masked == nil {
		d.masked, d.literal, d.quoted, _, d.unclosed = scanBody(d.Body)
	}
	return d.masked
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
	// Masked is Raw with the contents of inline code spans and HTML comments
	// blanked, keeping their length so that a position found here is the same
	// column in Raw. Matches are looked for in this one.
	Masked string
	// Number is the line's 1-based number in the file.
	Number int
	// Item is whether the line opens a list item, which ends the paragraph
	// before it.
	Item bool
}

// Unclosed reports whether the body ends inside an HTML comment, a fenced
// code block or an HTML block that only a marker closes.
//
// Everything from such a marker to the end of the file is masked, which is
// correct, and it means the document's headings and sections stop there: the
// parse is a prefix of what the author wrote. A writer that computes a line
// range from that parse and splices on it cuts across the gap, which is how
// `term remove` deleted every entry below an entry holding a stray `<!--`,
// silently, on a page that lint passed. Writers ask this before they splice.
func (d *Document) Unclosed() bool {
	d.maskedLines()
	return d.unclosed
}

// UnclosedIn is the same question about a whole file that is not a Document
// yet, which is what a writer has in hand just before it returns.
func UnclosedIn(source []byte) bool {
	_, body, _, ok := splitFrontMatter(source)
	if !ok {
		body = source
	}
	_, _, _, _, unclosed := scanBody(string(body))
	return unclosed
}

// Prose iterates the prose lines of a body: those not literal. Leaving the
// loop stops the walk.
func Prose(body string, bodyLine int) iter.Seq[ProseLine] {
	return func(yield func(ProseLine) bool) {
		raw := strings.Split(body, "\n")
		masked, literal, _, item, _ := scanBody(body)
		masked = maskParagraphCode(masked, literal, item)
		for i := range masked {
			if literal[i] != blockProse {
				continue
			}
			if !yield(ProseLine{Raw: raw[i], Masked: masked[i], Number: bodyLine + i, Item: item[i]}) {
				return
			}
		}
	}
}

// maskParagraphCode blanks the code spans in each run of prose lines that
// forms one paragraph, since a span may cross a line break within one: read a
// line at a time, a span wrapped onto the next line left its links readable.
// A heading, a list item and the other block starts begin a run of their own.
func maskParagraphCode(masked []string, literal []block, item []bool) []string {
	out := slices.Clone(masked)
	joins := func(i int) bool {
		inner, _ := stripQuotes(masked[i])
		return literal[i] == blockProse && strings.TrimSpace(inner) != "" && !IsHeading(inner)
	}
	for start := 0; start < len(masked); {
		end := start + 1
		if joins(start) {
			// Given the line before, so a quoted line continues a quoted
			// paragraph rather than starting a block of its own.
			for end < len(masked) && joins(end) && !startsBlock(ProseLine{Masked: masked[end], Item: item[end]}, []ProseLine{{Masked: masked[end-1]}}) {
				end++
			}
		}
		copy(out[start:], strings.Split(maskCode(strings.Join(masked[start:end], "\n")), "\n"))
		start = end
	}
	return out
}

// Prose is the same over one document's body.
func (d *Document) Prose() iter.Seq[ProseLine] { return Prose(d.Body, d.BodyLine) }

// maskCode blanks code spans, keeping the length so that any position derived
// from the text still refers to the right column. A line break inside a span
// is kept, so text joined from several lines splits back into them.
func maskCode(text string) string {
	out := []byte(text)
	for i := 0; i < len(text); {
		start, end, ok := codeSpan(text[i:])
		if !ok {
			break
		}
		for j := i + start; j < i+end; j++ {
			if out[j] != '\n' && out[j] != '\r' {
				out[j] = ' '
			}
		}
		i += end
	}
	return string(out)
}

// codeSpan finds the first inline code span in s and returns its byte range.
//
// A span opens on a run of backticks and closes on a run of exactly the same
// length. That "exactly the same" is what a regular expression cannot say here:
// Go's regexp is RE2, which has no backreferences, so the pattern this replaces
// accepted a closing run of any length and cut a double-backtick span short at
// its first inner backtick. Everything after the cut then parsed as prose, which
// is how “a `<!--` “ in a document opened a comment and buried the headings
// below it.
//
// A run with no matching closer is literal text, and scanning resumes just
// after it, so a shorter span later on the line is still found.
func codeSpan(s string) (start, end int, ok bool) {
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		opening := i
		for i < len(s) && s[i] == '`' {
			i++
		}
		if closed := matchingRun(s, i, i-opening); closed > 0 {
			return opening, closed, true
		}
	}
	return 0, 0, false
}

// matchingRun is the index just past the first run of exactly n backticks at or
// after from, or 0 when there is none.
func matchingRun(s string, from, n int) int {
	for i := from; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		run := i
		for i < len(s) && s[i] == '`' {
			i++
		}
		if i-run == n {
			return i
		}
	}
	return 0
}

// proseLines iterates the body lines that are not literal, with their
// zero-based index and with everything inside an HTML comment blanked.
func proseLines(body string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		masked, literal, _, _, _ := scanBody(body)
		for i, line := range masked {
			if literal[i] != blockProse {
				continue
			}
			if !yield(i, line) {
				return
			}
		}
	}
}

// scanBody splits text into lines, blanks the HTML comments in each, and
// reports which lines are literal: inside a fenced or indented code block or
// an HTML block, where nothing is prose but everything is content. quoted
// marks the lines a blockquote holds, literal or not, and item those that open
// a list item.
//
// It is the single definition of how blocks, code spans and comments
// interact, and every consumer reads it rather than restating the rule. There
// used to be a second definition, a regular expression requiring a closing
// marker, behind Section.Empty and the glossary's paragraph split. It
// disagreed with this one about an unterminated comment, and that
// disagreement was enough to carry an unfinished document through the freeze
// gate in silence.
//
// A literal block holds its lines to its end, so nothing in it opens a
// comment. A block can only begin a line, so a line already inside a comment
// cannot open one: whichever opens first holds the lines until it closes.
// Blocks are read inside blockquotes and list items, as CommonMark reads
// them: a fence indented under a list item hid the rest of the document
// behind a `<!--` it held, because it was read as prose.
func scanBody(text string) (masked []string, literal []block, quoted, item []bool, unclosed bool) {
	lines := strings.Split(text, "\n")
	masked = make([]string, len(lines))
	literal = make([]block, len(lines))
	quoted = make([]bool, len(lines))
	item = make([]bool, len(lines))

	var w blockWalk
	open := false
	// block is whether the open comment began a block, and depth how many
	// containers held it. A block ends with its container; a comment inside a
	// paragraph leaves the paragraph open around it.
	block, depth := false, 0
	for i, line := range lines {
		if open && block {
			if n, _ := w.continued(strings.TrimSuffix(line, "\r")); n < depth {
				open = false
			}
		}
		if open {
			// Past the marker that closes it, a comment opens only as one
			// inside a paragraph does.
			masked[i], open = maskCommentsIn(line, true, func(marker int) bool {
				// The rest of the line closing a comment block belongs to
				// the block, so nothing in it opens another.
				if block {
					return false
				}
				return closesInParagraph(lines, i, marker+len(commentOpen), &w)
			})
			w.paragraph = !block
			continue
		}
		at, kind := w.line(strings.TrimSuffix(line, "\r"))
		quoted[i], item[i] = len(w.stack) > 0 && w.stack[0].quote, w.item
		if kind != blockProse {
			masked[i], literal[i] = line, kind
			continue
		}
		opened := false // a comment block opened on this line holds the rest of it
		masked[i], open = maskCommentsIn(line, false, func(marker int) bool {
			if opened {
				return false
			}
			// A comment starting a block runs until it closes, however far
			// that is. One opened inside a paragraph is raw HTML only where
			// it closes before the paragraph ends, and text otherwise.
			if marker-at <= 3 && strings.TrimSpace(line[at:marker]) == "" {
				block, depth, opened = true, len(w.stack), true
				return true
			}
			block = false
			return closesInParagraph(lines, i, marker+len(commentOpen), &w) && !inCodeAcrossLines(lines, i, marker, &w)
		})
		w.prose(masked[i][min(at, len(masked[i])):])
	}
	// open means a comment never closed, and unclosed a block that runs to the
	// end of the text. Either way the walk masked everything from there on,
	// so the parse that follows is a prefix of what the author wrote.
	return masked, literal, quoted, item, open || w.unclosed()
}

// closesInParagraph reports whether a comment opened at column from of line i
// closes before its paragraph ends, w having read the lines up to i.
func closesInParagraph(lines []string, i, from int, w *blockWalk) bool {
	if strings.Contains(lines[i][from:], commentClose) {
		return true
	}
	for _, line := range lines[i+1 : paragraphEnd(lines, i, w)] {
		if strings.Contains(line, commentClose) {
			return true
		}
	}
	return false
}

// paragraphEnd is the index of the line that ends the paragraph holding line
// i, each line after it read in the containers w, the walk up to i, holds
// open: the next line blank once its containers' markers are passed, or one
// starting a block, which a line leaving the paragraph's container does more
// readily than one within it. A line continuing no container but starting no
// block continues the paragraph lazily.
func paragraphEnd(lines []string, i int, w *blockWalk) int {
	// A stack of its own, as truncating and pushing would overwrite w's.
	ahead := blockWalk{stack: slices.Clone(w.stack), paragraph: true}
	for k := i + 1; k < len(lines); k++ {
		line := strings.TrimSuffix(lines[k], "\r")
		at, _ := ahead.line(line)
		rest := line[at:]
		if !ahead.paragraph || IsHeading(rest) || thematicPattern.MatchString(rest) ||
			indentColumns(rest, column(line, at)) < 4 && strings.HasPrefix(strings.TrimLeft(rest, " \t"), commentOpen) {
			return k
		}
	}
	return len(lines)
}

// inCodeAcrossLines reports whether the marker at column marker of line i sits
// in a code span opening before it on that line and closing on a later line
// of the paragraph. A span closing on the same line is maskCommentsIn's.
func inCodeAcrossLines(lines []string, i, marker int, w *blockWalk) bool {
	s := lines[i]
	for j := 0; j < marker; {
		if s[j] != '`' {
			j++
			continue
		}
		opening := j
		for j < len(s) && s[j] == '`' {
			j++
		}
		if closed := matchingRun(s, j, j-opening); closed > 0 {
			j = closed
			continue
		}
		for _, next := range lines[i+1 : paragraphEnd(lines, i, w)] {
			if matchingRun(next, 0, j-opening) > 0 {
				return true
			}
		}
	}
	return false
}

// blockWalk follows the blocks of a body a line at a time: the blockquotes
// and list items each line continues, and the leaf block it is in.
//
// It reads as much of CommonMark as deciding what is prose needs: containers
// by their markers and indentation, lazy continuation of a paragraph, and
// which leaves cannot interrupt one. A tab runs to the next multiple of four
// columns.
type blockWalk struct {
	stack []container
	leaf  leafKind
	// depth is len(stack) when the leaf opened. A leaf ends with the
	// container holding it.
	depth     int
	fenceChar byte
	fenceLen  int
	// htmlEnd holds the markers that close an HTML block, any one of them;
	// empty, a blank line closes it.
	htmlEnd   []string
	paragraph bool
	// item is whether the last line read opened a list item.
	item bool
}

// container is a blockquote, or a list item whose content starts indent
// columns in from where its parent's content starts.
type container struct {
	quote  bool
	indent int
}

type leafKind int

// block is what holds a line: prose, a code block or an HTML block. Only
// prose is read for structure; an HTML block's anchors are still targets.
type block uint8

const (
	blockProse block = iota
	blockCode
	blockHTML
)

const (
	leafNone leafKind = iota
	leafFence
	leafIndented
	leafHTML
)

var (
	// The space after a quote marker is optional and may be a tab, which
	// is taken whole: CommonMark reads part of it as the space and the rest as
	// indentation, and counting the rest as four columns made code of a
	// definition written after a tab.
	quoteMarkerPattern = regexp.MustCompile(`^ {0,3}>[ \t]?`)
	listMarkerPattern  = regexp.MustCompile(`^( {0,3})([-+*]|(\d{1,9})[.)])([ \t]+|$)`)
	thematicPattern    = regexp.MustCompile(`^ {0,3}(?:(?:\*[ \t]*){3,}|(?:-[ \t]*){3,}|(?:_[ \t]*){3,})$`)
	// The HTML blocks CommonMark recognises, other than a comment, which the
	// comment masker reads: those closing on a marker, then those closing on a
	// blank line. The last, a lone tag, cannot interrupt a paragraph.
	htmlRawPattern    = regexp.MustCompile(`(?i)^<(script|pre|style|textarea)(?:[ \t>]|$)`)
	htmlRawTagPattern = regexp.MustCompile(`(?i)^</?(?:script|pre|style|textarea)\b`)
	htmlOtherPattern  = regexp.MustCompile(`^<(?:\?|!\[CDATA\[|![A-Za-z])`)
	htmlBlockPattern  = regexp.MustCompile(`(?i)^</?(?:address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(?:[ \t]|/?>|$)`)
	htmlTagPattern    = regexp.MustCompile(`^(?:` + openTag + `|</[A-Za-z][A-Za-z0-9-]*[ \t]*>)[ \t]*$`)
)

// openTag is an HTML opening tag with its attributes.
const openTag = `<[A-Za-z][A-Za-z0-9-]*(?:[ \t]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t]*=[ \t]*(?:[^ \t"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?)*[ \t]*/?>`

// line reads one line, without its carriage return, and returns the column
// its content starts at once its containers' markers are passed, and
// the block holding it.
func (w *blockWalk) line(s string) (at int, kind block) {
	w.item = false
	matched, at := w.continued(s)
	if matched < len(w.stack) {
		// Read in the last container the line continues, which is not the
		// paragraph, so an item of any number or none interrupts it.
		var outside blockWalk
		if w.leaf == leafNone && w.paragraph && strings.TrimSpace(s[at:]) != "" && !outside.interrupts(s[at:], column(s, at)) {
			return at, blockProse // a lazy continuation of the paragraph
		}
		w.stack, w.paragraph = w.stack[:matched], false
		if w.depth > matched {
			w.leaf = leafNone
		}
	}

	rest := s[at:]
	switch w.leaf {
	case leafFence:
		if closesFence(rest, w.fenceChar, w.fenceLen) {
			w.leaf = leafNone
		}
		return at, blockCode
	case leafHTML:
		if len(w.htmlEnd) == 0 && strings.TrimSpace(rest) == "" {
			w.leaf, w.paragraph = leafNone, false
			return at, blockProse
		}
		if closesHTML(rest, w.htmlEnd) {
			w.leaf = leafNone
		}
		return at, blockHTML
	case leafIndented:
		if strings.TrimSpace(rest) == "" {
			return at, blockProse
		}
		if indentColumns(rest, column(s, at)) >= 4 {
			return at, blockCode
		}
		w.leaf, w.paragraph = leafNone, false
	}

	for {
		rest = s[at:]
		if m := quoteMarkerPattern.FindStringIndex(rest); m != nil {
			w.stack = append(w.stack, container{quote: true})
			at, w.paragraph = at+m[1], false
			continue
		}
		if n, indent, ok := w.listItem(rest, column(s, at)); ok {
			w.stack = append(w.stack, container{indent: indent})
			at, w.paragraph, w.item = at+n, false, true
			continue
		}
		break
	}

	rest = s[at:]
	switch {
	case strings.TrimSpace(rest) == "":
		w.paragraph = false
		return at, blockProse
	case fenceOpens(rest):
		m := fencePattern.FindStringSubmatch(rest)
		w.open(leafFence)
		w.fenceChar, w.fenceLen = m[1][0], len(m[1])
		return at, blockCode
	case !w.paragraph && indentColumns(rest, column(s, at)) >= 4:
		w.open(leafIndented)
		return at, blockCode
	}
	if end, ok := htmlStarts(rest, column(s, at), w.paragraph); ok {
		w.open(leafHTML)
		w.htmlEnd = end
		// Searched from past the "<" that opened it, as the opening and the
		// closing marker may share a character: <?> opens and closes at once.
		if closesHTML(rest[strings.IndexByte(rest, '<')+1:], end) {
			w.leaf = leafNone
		}
		return at, blockHTML
	}
	return at, blockProse
}

// continued is how many of the open containers line s continues, and the byte
// its content starts at once their markers are passed.
func (w *blockWalk) continued(s string) (matched, at int) {
	for ; matched < len(w.stack); matched++ {
		rest := s[at:]
		if w.stack[matched].quote {
			m := quoteMarkerPattern.FindStringIndex(rest)
			if m == nil {
				break
			}
			at += m[1]
			continue
		}
		if strings.TrimSpace(rest) == "" {
			continue
		}
		n, ok := consumeIndent(rest, w.stack[matched].indent, column(s, at))
		if !ok {
			break
		}
		at += n
	}
	return matched, at
}

func (w *blockWalk) open(leaf leafKind) {
	w.leaf, w.depth, w.paragraph = leaf, len(w.stack), false
}

// prose records what a prose line, with its comments blanked, leaves open: a
// paragraph, unless it was blank, a heading or a thematic break.
func (w *blockWalk) prose(content string) {
	content = strings.TrimSuffix(content, "\r")
	w.paragraph = strings.TrimSpace(content) != "" && !IsHeading(content) && !thematicPattern.MatchString(content)
}

// unclosed reports a block still open at the end that only a marker closes.
func (w *blockWalk) unclosed() bool {
	return w.leaf == leafFence || w.leaf == leafHTML && len(w.htmlEnd) > 0
}

// interrupts reports whether a line starts a block, so that it cannot be a
// lazy continuation of the paragraph before it.
func (w *blockWalk) interrupts(s string, from int) bool {
	_, _, item := w.listItem(s, from)
	_, html := htmlStarts(s, from, true)
	return item || html || quoteMarkerPattern.MatchString(s) || fenceOpens(s) || IsHeading(s) ||
		thematicPattern.MatchString(s) || strings.HasPrefix(strings.TrimLeft(s, " "), commentOpen)
}

// listItem reads a list marker: how many bytes it and the spaces after it
// take, and the column the item's content starts at. A thematic break is not
// an item, and inside a paragraph neither is an empty item or a numbered one
// not starting at 1.
func (w *blockWalk) listItem(s string, from int) (n, indent int, ok bool) {
	m := listMarkerPattern.FindStringSubmatchIndex(s)
	if m == nil || thematicPattern.MatchString(s) {
		return 0, 0, false
	}
	empty := m[1] == len(s)
	if w.paragraph && (empty || m[6] >= 0 && strings.TrimLeft(s[m[6]:m[7]], "0") != "1") {
		return 0, 0, false
	}
	marker := m[5]
	spaces := indentColumns(s[marker:], from+marker)
	switch {
	case empty:
		return len(s), marker + 1, true
	case spaces > 4:
		// The content is an indented code block, one column in.
		return marker + 1, marker + 1, true
	}
	return m[1], marker + spaces, true
}

// fenceOpens reports a fence opening a line. A backtick fence's info string
// cannot hold a backtick, so ```go``` is a code span in a paragraph.
func fenceOpens(s string) bool {
	m := fencePattern.FindStringSubmatchIndex(s)
	return m != nil && (s[m[2]] != '`' || !strings.Contains(s[m[3]:], "`"))
}

// htmlStarts reports an HTML block opening a line, and the markers that close
// it, any one of them, empty where a blank line does.
func htmlStarts(s string, from int, paragraph bool) (end []string, ok bool) {
	if indentColumns(s, from) > 3 {
		return nil, false
	}
	s = strings.TrimLeft(s, " \t")
	if htmlRawPattern.MatchString(s) {
		return rawEnds, true
	}
	switch m := htmlOtherPattern.FindString(s); m {
	case "":
	case "<?":
		return []string{"?>"}, true
	case "<![CDATA[":
		return []string{"]]>"}, true
	default:
		return []string{">"}, true
	}
	// A lone tag is any but the four the first kind opens with, whose closing
	// tags are not a block of their own either.
	if htmlBlockPattern.MatchString(s) || !paragraph && htmlTagPattern.MatchString(s) && !htmlRawTagPattern.MatchString(s) {
		return nil, true
	}
	return nil, false
}

// rawEnds close a block opened by script, pre, style or textarea, whichever
// opened it.
var rawEnds = []string{"</script>", "</pre>", "</style>", "</textarea>"}

// closesHTML reports whether a line holds one of the markers that close an
// HTML block, in any case.
func closesHTML(line string, end []string) bool {
	lower := strings.ToLower(line)
	return slices.ContainsFunc(end, func(marker string) bool { return strings.Contains(lower, marker) })
}

// column is the column byte at of s begins in. A tab runs to the next multiple
// of four, so its width depends on where it starts. What precedes at is
// markers and whitespace, a byte a column each.
func column(s string, at int) int {
	col := 0
	for i := 0; i < at; i++ {
		col = advance(col, s[i])
	}
	return col
}

// advance is the column after byte b, read at column col.
func advance(col int, b byte) int {
	if b == '\t' {
		return col + 4 - col%4
	}
	return col + 1
}

// indentColumns is the width of s's leading whitespace when s begins at column
// from.
func indentColumns(s string, from int) int {
	col := from
	for i := 0; i < len(s) && (s[i] == ' ' || s[i] == '\t'); i++ {
		col = advance(col, s[i])
	}
	return col - from
}

// consumeIndent passes want columns of leading whitespace, s beginning at
// column from, and returns the bytes that took, or false when the line has
// fewer.
func consumeIndent(s string, want, from int) (int, bool) {
	col := from
	for i := 0; col-from < want; i++ {
		if i == len(s) {
			return 0, false
		}
		if s[i] != ' ' && s[i] != '\t' {
			return 0, false
		}
		col = advance(col, s[i])
		if col-from >= want {
			return i + 1, true
		}
	}
	return 0, true
}

// closesFence reports whether a line closes an open fence: a run of the same
// character, at least as long as the run that opened it, followed by nothing
// but whitespace.
//
// An info string is permitted on the opening fence only, so a line reading
// "``` inside" within a ```text block is content. One pattern was serving both
// ends, and because it is unanchored it accepted the trailing text: the block
// ended early and its real closing fence then opened a second one, which ran to
// the end of the document.
func closesFence(line string, char byte, length int) bool {
	loc := fencePattern.FindStringSubmatchIndex(line)
	if loc == nil {
		return false
	}
	run := line[loc[2]:loc[3]]
	if run[0] != char || len(run) < length {
		return false
	}
	return strings.TrimSpace(line[loc[3]:]) == ""
}

// hasContent reports whether text holds anything but whitespace and HTML
// comments, reading a comment exactly as the prose walk does. A fenced block is
// content, marker or no marker.
func hasContent(text string) bool {
	masked, _, _, _, _ := scanBody(text)
	for _, line := range masked {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

const (
	commentOpen  = "<!--"
	commentClose = "-->"
)

// maskCommentsIn blanks the parts of one line that lie inside an HTML comment,
// given whether a comment was already open when the line began, and reports
// whether one is still open when it ends. Blanking keeps every byte where it
// was, so an offset in the result is the same offset in the line.
//
// A comment is nothing wherever it sits. Section.Empty has always read one that
// way, and a heading commented out must not satisfy L08. Blanking rather than
// dropping whole lines is what makes a comment on one line behave like a
// comment spanning several, and what keeps the prose that follows a --> on the
// line that closes one.
//
// A marker inside an inline code span is literal, because a span binds tighter
// than raw HTML: `<!--` written as an example used to open a comment that
// swallowed the rest of the document. opens decides whether an unescaped
// marker outside a span opens a comment, which depends on where in its block
// it sits.
func maskCommentsIn(line string, open bool, opens func(marker int) bool) (string, bool) {
	out := []byte(line)
	for i := 0; i < len(line); {
		if open {
			// Searched from the opening marker rather than past it, so that
			// <!--> reads as the empty comment it is.
			end := strings.Index(line[i:], commentClose)
			if end < 0 {
				blank(out, i, len(line))
				return string(out), true
			}
			end += i + len(commentClose)
			blank(out, i, end)
			i, open = end, false
			continue
		}

		rest := line[i:]
		start := strings.Index(rest, commentOpen)
		if start < 0 {
			break
		}
		// A code span that begins before the marker either contains it, making
		// it literal, or ends before it, in which case the marker is found
		// again on the next pass. Resuming after the span is right either way.
		if spanStart, spanEnd, ok := codeSpan(rest); ok && spanStart < start {
			i += spanEnd
			continue
		}
		// A marker after an odd run of backslashes is escaped. Any other opens
		// a comment where opens says so, and is text otherwise.
		if escaped(line, i+start) || !opens(i+start) {
			i += start + len(commentOpen)
			continue
		}
		i, open = i+start, true
	}
	return string(out), open
}

// escaped reports an odd run of backslashes before s[at].
func escaped(s string, at int) bool {
	n := 0
	for at-n > 0 && s[at-n-1] == '\\' {
		n++
	}
	return n%2 == 1
}

// blank overwrites a range with spaces, keeping every byte position so that an
// offset in the result means the same column in the original.
func blank(out []byte, from, to int) {
	for i := from; i < to; i++ {
		out[i] = ' '
	}
}

// parseHeadings walks the body once and assigns each heading its anchor.
// Anchors depend on the headings before them, so they cannot be derived from a
// heading alone.
func parseHeadings(body string, bodyLine int) []Heading {
	var headings []Heading
	raw := strings.Split(body, "\n")
	for i, line := range proseLines(body) {
		at := headingPattern.FindStringSubmatchIndex(line)
		if at == nil {
			continue
		}
		// The masked line keeps every offset, so the content as written is at
		// the same columns of the raw one, from after the marker: a comment
		// masked to spaces is read as part of the space after it.
		content, written := "", ""
		if at[4] >= 0 {
			content, written = line[at[4]:at[5]], raw[i][at[3]:at[5]]
		}
		// A heading of nothing but a closing sequence is empty.
		text := ""
		if strings.Trim(content, "#") != "" {
			text = strings.TrimSpace(closingHashes.ReplaceAllString(content, ""))
		}
		headings = append(headings, Heading{
			Level:   at[3] - at[2],
			Text:    text,
			Line:    bodyLine + i,
			index:   i,
			written: strings.TrimSpace(closingHashes.ReplaceAllString(written, "")),
		})
	}
	return headings
}

// anchorHeadings gives each heading its anchor. A reference link in a
// heading renders as a link only where the document defines its label, so
// the anchors wait for the definitions, which are read once the headings are.
func (d *Document) anchorHeadings() {
	labels := d.labels()
	taken := map[string]int{}
	for i := range d.headings {
		d.headings[i].Anchor = anchor(d.headings[i].written, taken, labels)
	}
}

// labels are the labels of the document's link reference definitions,
// normalised.
func (d *Document) labels() map[string]bool {
	labels := map[string]bool{}
	for _, l := range d.definitions() {
		labels[normalLabel(l.Text)] = true
	}
	return labels
}

// Anchor is the fragment GitHub would generate for a heading standing on its
// own. Headings inside a document take their anchors from a walk that numbers
// repeats, so this is only for naming the anchor a heading would have had when
// the heading is not there to ask.
// A reference link in it has no definition to name, so it is text.
func Anchor(text string) string { return anchor(text, map[string]int{}, nil) }

// anchor renders a heading the way GitHub does: slugged from its rendered
// text, lowercased, keeping letters, marks, decimal digits, letter numbers,
// connector punctuation and hyphens, each space a hyphen, and a numeric suffix
// when the same anchor has already been used in this document. taken is
// updated in place.
//
// Slugging the text as written kept what renders as nothing: emphasis
// underscores, a tag's name, a reference link's label, an image's alt text.
// Keeping only IsLetter and IsDigit dropped the marks of Devanagari and the
// letter numbers and connector punctuation GitHub keeps. L17 then rejected
// links that work and passed ones that 404.
func anchor(text string, taken map[string]int, labels map[string]bool) string {
	var b strings.Builder
	for _, r := range strings.ToLower(renderedText(text, labels)) {
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

// renderedText is a heading's inline content as GitHub renders it to text:
// escapes and entities resolved, a code span's content kept as written, tags
// dropped, a link as its text, an image as nothing, an autolink as its
// address, and an emphasis delimiter kept only where it pairs with nothing.
// labels are the document's link reference definitions,
// normalised: a reference to a label it does not define is text.
func renderedText(s string, labels map[string]bool) string {
	return inline{s: s, labels: labels}.render(0, len(s), false)
}

// inline renders a heading's content. Positions are into the whole of it, as
// a delimiter is judged by the characters either side of it even where those
// are a link's brackets.
type inline struct {
	s      string
	labels map[string]bool
}

// render renders s[from:to] as one scope for emphasis: a link's text is
// paired on its own, as CommonMark pairs it before the link is closed.
// bracketed is set for a link's text, where GitHub links no bare address.
func (in inline) render(from, to int, bracketed bool) string {
	s := in.s[:to]
	var pieces []string
	var runs []delimiterRun
	var b strings.Builder
	piece := func(text string) {
		pieces = append(pieces, b.String(), text)
		b.Reset()
	}
	// brackets are the brackets opened as text and not yet closed, as
	// cmark-gfm stacks them: [ and ! for a link's and an image's, and a space
	// for a link's once a link after it has closed. While a link's or an
	// image's is open, a bare address is not linked.
	var brackets []byte
	for i := from; i < to; {
		if !bracketed && !slices.ContainsFunc(brackets, func(c byte) bool { return c != ' ' }) {
			if end := in.autolink(i); end > 0 {
				b.WriteString(s[i:end])
				i = end
				continue
			}
		}
		switch c := s[i]; {
		case c == '\\' && i+1 < to && isASCIIPunct(s[i+1]):
			b.WriteByte(s[i+1])
			i += 2
		case c == '`':
			run := i
			for i < to && s[i] == '`' {
				i++
			}
			end := matchingRun(s, i, i-run)
			if end == 0 {
				continue // a backtick renders as itself, which a slug drops
			}
			content := s[i : end-(i-run)]
			if content[0] == ' ' && content[len(content)-1] == ' ' && strings.TrimSpace(content) != "" {
				content = content[1 : len(content)-1]
			}
			b.WriteString(content)
			i = end
		case c == '&':
			if m := entityPattern.FindString(s[i:]); m != "" {
				b.WriteString(html.UnescapeString(m))
				i += len(m)
				continue
			}
			b.WriteByte(c)
			i++
		case c == '<':
			if m := inlineTagPattern.FindString(s[i:]); m != "" {
				i += len(m)
			} else if m := autolinkPattern.FindStringSubmatch(s[i:]); m != nil {
				b.WriteString(m[1])
				i += len(m[0])
			} else {
				b.WriteByte(c)
				i++
			}
		case c == '!' && strings.HasPrefix(s[i+1:], "["):
			if _, end, ok := in.link(i+1, to); ok {
				i = end
				continue
			}
			b.WriteString("![")
			brackets = append(brackets, '!')
			i += 2
		case c == '[':
			// A link holding another is text around that one.
			if close, end, ok := in.link(i, to); ok && !in.holdsLink(i+1, close) {
				piece(in.render(i+1, close, true))
				i = end
				for k, open := range brackets {
					if open == '[' {
						brackets[k] = ' '
					}
				}
				continue
			}
			b.WriteByte(c)
			brackets = append(brackets, '[')
			i++
		case c == ']':
			if len(brackets) > 0 {
				brackets = brackets[:len(brackets)-1]
			}
			b.WriteByte(c)
			i++
		case c == '*' || c == '_' || c == '~':
			run := i
			for i < to && s[i] == c {
				i++
			}
			runs = append(runs, in.delimiters(run, i, len(pieces)+1))
			piece(s[run:i])
		default:
			b.WriteByte(c)
			i++
		}
	}
	pair(runs)
	for _, r := range runs {
		pieces[r.piece] = strings.Repeat(string(r.char), r.left)
	}
	return strings.Join(pieces, "") + b.String()
}

// autolink is where the address GitHub links without brackets at s[i] ends,
// or 0, as cmark-gfm's autolink extension reads one: http://, https:// or
// ftp:// in any case, not straight after a letter, or www. at the start or
// after a space, a tab, *, _, ~ or (, and in both a domain with no
// underscore in its last two parts. The address runs to a space or a tab,
// then stops before any <, and sheds trailing punctuation, a trailing entity
// and each ) beyond those it opens. An email address is linked too, but only
// once emphasis is read and as the text it was, so it changes no anchor.
func (in inline) autolink(i int) int {
	s := in.s
	switch {
	case strings.HasPrefix(s[i:], "www."):
		if i > 0 && strings.IndexByte("*_~( \t", s[i-1]) < 0 || underscored(s[i:]) {
			return 0
		}
	case i == 0 || !isASCIIAlpha(s[i-1]):
		if !linkedScheme(s[i:]) {
			return 0
		}
	default:
		return 0
	}
	end := i
	for end < len(s) && s[end] != ' ' && s[end] != '\t' {
		end++
	}
	address := s[i:end]
	if at := strings.IndexByte(address, '<'); at >= 0 {
		address = address[:at]
	}
	for {
		n := len(address)
		switch c := address[n-1]; {
		case strings.IndexByte("?!.,:*_~'\"", c) >= 0:
			address = address[:n-1]
		case c == ';':
			k := n - 2
			for isASCIIAlpha(address[k]) {
				k--
			}
			if k < n-2 && address[k] == '&' {
				address = address[:k]
			} else {
				address = address[:n-1]
			}
		case c == ')' && strings.Count(address, ")") > strings.Count(address, "("):
			address = address[:n-1]
		default:
			return i + n
		}
	}
}

// linkedScheme reports whether s begins with http://, https:// or ftp://, in
// any case, and a domain GitHub links.
func linkedScheme(s string) bool {
	for _, scheme := range []string{"http://", "https://", "ftp://"} {
		if len(s) > len(scheme) && strings.EqualFold(s[:len(scheme)], scheme) {
			return hostChar(s[len(scheme):]) && !underscored(s[len(scheme):])
		}
	}
	return false
}

// underscored reports whether either of the last two parts of the domain at
// the start of s holds an underscore. The domain runs over the characters a
// host may hold, hyphens, underscores and dots.
func underscored(s string) bool {
	before, last := false, false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '_':
			last = true
		case s[i] == '.':
			before, last = last, false
		case !hostChar(s[i:]) && s[i] != '-':
			return before || last
		}
	}
	return before || last
}

// hostChar reports whether s begins with a character a domain may hold:
// neither a space nor punctuation. A byte inside a character is
// not one, so a domain is read only as far as its first character beyond
// ASCII, as cmark-gfm reads it byte by byte.
func hostChar(s string) bool {
	r, n := utf8.DecodeRuneInString(s)
	return (n > 1 || r != utf8.RuneError) && !space(r) && !punctuation(r)
}

func isASCIIAlpha(c byte) bool { return 'a' <= c|0x20 && c|0x20 <= 'z' }

var (
	entityPattern    = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});`)
	inlineTagPattern = regexp.MustCompile(`^(?:` + openTag + `|</[A-Za-z][A-Za-z0-9-]*[ \t]*>|<!--.*?-->)`)
	autolinkPattern  = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9+.-]{1,31}:[^ \t<>]*|[^ \t<>@]+@[^ \t<>]+)>`)
)

// link reads a link whose text opens at s[i] and that ends before to:
// [text](destination), or a reference to a label the document defines,
// [text][label], [text][] or [text]. It returns where its text closes and
// where it ends.
func (in inline) link(i, to int) (close, end int, ok bool) {
	s := in.s[:to]
	at := balanced(s[i:], '[', ']')
	if at < 0 {
		return 0, 0, false
	}
	close = i + at
	rest := s[close+1:]
	if strings.HasPrefix(rest, "(") {
		if at := balanced(rest, '(', ')'); at >= 0 {
			return close, close + at + 2, true
		}
	}
	label, end := s[i+1:close], close+1
	if strings.HasPrefix(rest, "[") {
		if at := balanced(rest, '[', ']'); at >= 0 {
			if at > 1 {
				label = rest[1:at]
			}
			end += at + 1
		}
	}
	return close, end, in.labels[normalLabel(label)]
}

// holdsLink reports whether s[from:to] holds a link. An image's brackets are
// not a link's, though a link in its text is.
func (in inline) holdsLink(from, to int) bool {
	for i := from; i < to; i++ {
		switch in.s[i] {
		case '\\':
			i++
		case '!':
			if in.s[i+1] == '[' { // s[to] is the closing bracket
				i++
			}
		case '[':
			if _, _, ok := in.link(i, to); ok {
				return true
			}
		}
	}
	return false
}

// balanced is the index of the closer matching the opener at s[0], with
// backslash escapes passed over, or -1.
func balanced(s string, opener, closer byte) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case opener:
			depth++
		case closer:
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

// delimiterRun is a run of *, _ or ~, which is text where it pairs with
// nothing and renders as nothing where it does.
type delimiterRun struct {
	char byte
	// n is the run's length, and left how much of it pairs with nothing.
	n, left     int
	open, close bool
	// removed is set once a pair encloses the run, after which it cannot
	// pair, though its characters stay.
	removed bool
	// piece is the run's place among the rendered pieces.
	piece int
}

// delimiters reads the run s[from:to] as cmark-gfm does: whether it can open
// or close follows from what flanks it, a strikethrough's tildes beside it
// passed over, and a run of three or more tildes is text.
func (in inline) delimiters(from, to, piece int) delimiterRun {
	before, after := ' ', ' '
	i, j := from, to
	for i > 0 && in.s[i-1] == '~' {
		i--
	}
	for j < len(in.s) && in.s[j] == '~' {
		j++
	}
	if i > 0 {
		before, _ = utf8.DecodeLastRuneInString(in.s[:i])
	}
	if j < len(in.s) {
		after, _ = utf8.DecodeRuneInString(in.s[j:])
	}
	left := !space(after) && (!punctuation(after) || space(before) || punctuation(before))
	right := !space(before) && (!punctuation(before) || space(after) || punctuation(after))
	r := delimiterRun{char: in.s[from], n: to - from, left: to - from, open: left, close: right, piece: piece}
	switch {
	case r.char == '_':
		r.open = left && (!right || punctuation(before))
		r.close = right && (!left || punctuation(after))
	case r.char == '~' && r.n > 2:
		r.open, r.close = false, false
	}
	return r
}

// space is a space as cmark-gfm reads one beside a delimiter or in a
// domain: a space separator, a tab or a form feed. A line ending cannot be
// within a heading.
func space(r rune) bool { return unicode.Is(unicode.Zs, r) || r == '\t' || r == '\f' }

// punctuation is CommonMark's punctuation as cmark-gfm reads it: ASCII
// punctuation and symbols, and the Unicode punctuation classes, so not a
// symbol beyond ASCII.
func punctuation(r rune) bool {
	return r < utf8.RuneSelf && isASCIIPunct(byte(r)) || unicode.IsPunct(r)
}

// pair matches the delimiter runs of one scope as cmark-gfm does, from each
// closer back to the nearest opener of the same character: two characters
// from each where both have two, otherwise one, and a strikethrough only
// between runs of the same length. A pair removes the runs between it. A run
// that can open and close does not pair with one whose length would make the
// two sum to a multiple of three, unless both are.
func pair(runs []delimiterRun) {
	for c := 0; c < len(runs); {
		// A run is only removed by a pair closing at or before it, so the
		// closer read here never has been.
		closer := &runs[c]
		o := -1
		if closer.close {
			for o = c - 1; o >= 0; o-- {
				opener := &runs[o]
				if !opener.removed && opener.open && opener.char == closer.char &&
					(!(closer.open || opener.close) || closer.n%3 == 0 || (opener.n+closer.n)%3 != 0) {
					break
				}
			}
		}
		if o < 0 {
			c++
			continue
		}
		opener := &runs[o]
		if closer.char == '~' {
			// The tildes are left as text, which a slug drops.
			if opener.n == closer.n {
				for k := o; k <= c; k++ {
					runs[k].removed = true
				}
			}
			c++
			continue
		}
		use := 1
		if opener.left >= 2 && closer.left >= 2 {
			use = 2
		}
		for k := o + 1; k < c; k++ {
			runs[k].removed = true
		}
		opener.left -= use
		closer.left -= use
		opener.removed = opener.left == 0
		if closer.left == 0 {
			closer.removed = true
			c++
		}
	}
}

func isASCIIPunct(c byte) bool {
	return c < 0x80 && (unicode.IsPunct(rune(c)) || unicode.IsSymbol(rune(c)))
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
	Text   string
	Target string
	Type   LinkType
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
	// Heading is whether the link is in a heading, where link leaves it: the
	// heading's text is what L08, L09, L10 and anchors read.
	Heading bool
}

// Links returns every markdown link in the body, ignoring code, in the order
// their destinations appear. A link reference definition counts: its
// destination is a target like any other, and nothing was checking it. A
// footnote, [^label]:, is GitHub's and holds prose, so it is not one.
func (d *Document) Links() []Link {
	links, _ := d.links()
	return links
}

// References returns each use of a link reference definition, [text][label],
// [label][] or [label], with the destination of the first definition of its
// label. Its position is its closing bracket's, which follows its text.
func (d *Document) References() []Link {
	_, uses := d.links()
	return uses
}

// links reads the links and the reference uses together, since a use is only
// one where a definition of its label exists.
func (d *Document) links() (links, uses []Link) {
	links = d.definitions()
	labels := map[string]bool{}
	targets := map[string]string{}
	defined := map[int]bool{}
	for _, l := range links {
		defined[l.Line] = true
		label := normalLabel(l.Text)
		if !labels[label] {
			labels[label] = true
			targets[label] = l.Target
		}
	}
	inline := inlineLinks{labels: labels}
	for line := range d.Prose() {
		links = append(links, inline.read(line)...)
	}
	// A definition's own label reads as a use of it, which it is not.
	for _, use := range inline.uses {
		if !defined[use.Line] {
			use.Target = targets[use.Text]
			uses = append(uses, use)
		}
	}
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Line != links[j].Line {
			return links[i].Line < links[j].Line
		}
		return links[i].Start < links[j].Start
	})
	return links, uses
}

// definitions returns the link reference definitions, which are read first
// because a reference may come before the definition it uses.
func (d *Document) definitions() []Link {
	var links []Link
	opensBlock := true
	quoted := d.questionQuotes()
	previous := d.BodyLine - 1
	for line := range d.Prose() {
		// A literal block between two prose lines ends the paragraph before
		// it, as a heading does.
		if line.Number != previous+1 {
			opensBlock = true
		}
		previous = line.Number
		// A definition an answer uses is written inside its blockquote, so in
		// a question section it is read from behind the marker. A blockquote
		// interrupts a paragraph, so its first line can hold one.
		content := line.Masked
		if index := line.Number - d.BodyLine; quoted[index] {
			content = unquote(content)
			if !quoted[index-1] {
				opensBlock = true
			}
		}
		// A definition cannot interrupt a paragraph, so it only counts after a
		// blank line or another definition.
		if opensBlock {
			if at := definitionPattern.FindStringSubmatchIndex(content); at != nil {
				m := submatches(content, at)
				// Unquoting changes only what precedes the text, so a column
				// in content is the same distance from the end of the line.
				shift := len(line.Masked) - len(content)
				start, end := destinationAt(at, 2)
				links = append(links, Link{
					Text: m[1], Target: destination(m[2], m[3]),
					Type: LinkDefinition, Line: line.Number,
					Start: start + shift, End: end + shift,
				})
				continue
			}
		}
		opensBlock = strings.TrimSpace(content) == "" || IsHeading(content)
	}
	return links
}

// normalLabel compares labels as CommonMark does: case folded, with each run
// of whitespace one space.
func normalLabel(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(label), " "))
}

// maxLabel is the longest label CommonMark allows, in bytes.
const maxLabel = 999

// inlineLinks reads [text](destination) and ![text](destination) across the
// lines of a paragraph, as CommonMark does: the text may run onto later lines
// and hold brackets in balanced pairs, an escaped bracket is literal, an image
// may be the text of a link, and a link, inline or by reference, may not.
// Lines are read from the masked view, so a bracket inside code or a comment
// is not one. A reference use is not returned; its definition is, and the use
// is kept in uses.
type inlineLinks struct {
	// labels are the document's definitions, normalised.
	labels map[string]bool
	// uses are the reference uses read so far, each under its normalised
	// label, at its closing bracket.
	uses []Link
	// paragraph is the lines read since the text could last have opened, and
	// open the brackets in them not yet closed, innermost last.
	paragraph []ProseLine
	open      []opener
	// inactiveBelow is how many of open, from the outermost, a link has
	// closed inside, which makes those that are not images inactive, since
	// links do not nest. One bound rather than a flag on each opener, so a
	// link closing does not walk every bracket still open.
	inactiveBelow int
}

// opener is a [ or ![ waiting for its ].
type opener struct {
	line, column int // within paragraph, the column after the bracket
	image        bool
}

// read takes the next prose line and returns the links that close on it.
func (in *inlineLinks) read(line ProseLine) []Link {
	gap := len(in.paragraph) > 0 && in.paragraph[len(in.paragraph)-1].Number != line.Number-1
	if gap || strings.TrimSpace(line.Masked) == "" || startsBlock(line, in.paragraph) {
		in.paragraph, in.open, in.inactiveBelow = nil, nil, 0
	}
	in.paragraph = append(in.paragraph, line)
	here := len(in.paragraph) - 1

	var links []Link
	s := line.Masked
	// A destination ends with a ), so after the last one on the line there is
	// none to try for. Without this each ] ran the pattern to the end of the
	// line, which made a long line quadratic.
	lastClose := strings.LastIndexByte(s, ')')
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == '[':
			in.open = append(in.open, opener{line: here, column: i + 1})
		case s[i] == '!' && i+1 < len(s) && s[i+1] == '[':
			in.open = append(in.open, opener{line: here, column: i + 2, image: true})
			i++
		case s[i] == ']' && len(in.open) > 0:
			o := in.open[len(in.open)-1]
			in.open = in.open[:len(in.open)-1]
			inactive := !o.image && len(in.open) < in.inactiveBelow
			in.inactiveBelow = min(in.inactiveBelow, len(in.open))
			rest := s[i+1:]
			if at := tail(rest, i < lastClose); at != nil {
				if inactive {
					continue
				}
				m := submatches(rest, at)
				start, end := destinationAt(at, 1)
				link := Link{
					Text: in.text(o, here, i), Target: destination(m[1], m[2]), Type: LinkInline,
					Line: line.Number, Start: i + 1 + start, End: i + 1 + end,
				}
				if o.image {
					link.Type = LinkImage
				} else {
					in.inactiveBelow = len(in.open)
				}
				links = append(links, link)
				i += at[1]
			} else if skip, label, ok := in.reference(o, here, i, rest); ok && !inactive {
				if !o.image {
					in.inactiveBelow = len(in.open)
				}
				in.uses = append(in.uses, Link{Text: label, Type: LinkReference, Line: line.Number, Start: i, End: i})
				i += skip
			}
		}
	}
	if headingPattern.MatchString(s) {
		// A heading is one line, so nothing it leaves open continues.
		in.paragraph, in.open, in.inactiveBelow = nil, nil, 0
	}
	return links
}

// tail matches a destination after a closing bracket, where one can follow.
//
// What follows a link's closing bracket is the destination in either of its
// two forms, <...>, which may hold spaces, or a bare run, then any title, all
// in parentheses. A bare run holds parentheses only in balanced pairs, and a
// backslash escapes one; ending it at the first ) read foo(bar).md as foo(bar.
// The result is laid out as a regexp submatch index: the whole tail, then the
// <...> form, then the bare one.
func tail(rest string, closes bool) []int {
	if !closes || !strings.HasPrefix(rest, "(") {
		return nil
	}
	i := blanks(rest, 1)
	at := []int{0, 0, -1, -1, -1, -1}
	if strings.HasPrefix(rest[i:], "<") {
		end := strings.IndexAny(rest[i+1:], "<>\n")
		if end < 0 || rest[i+1+end] != '>' {
			return nil
		}
		at[2], at[3] = i+1, i+1+end
		i += end + 2
	} else {
		start, depth := i, 0
	bare:
		for ; i < len(rest); i++ {
			switch rest[i] {
			case '\\':
				if i+1 < len(rest) && isASCIIPunct(rest[i+1]) {
					i++
				}
			case ' ', '\t', '\n':
				break bare
			case '(':
				depth++
			case ')':
				if depth == 0 {
					break bare
				}
				depth--
			}
		}
		if depth != 0 {
			return nil
		}
		at[4], at[5] = start, i
	}
	// A title is set apart from the destination, and in "", '' or ().
	j := blanks(rest, i)
	if closer, ok := titleClosers[byteAt(rest, j)]; ok && j > i {
		k := j + 1
		for ; k < len(rest) && rest[k] != closer; k++ {
			if rest[k] == '\\' {
				k++
			} else if rest[k] == '(' && closer == ')' {
				return nil
			}
		}
		j = blanks(rest, k+1)
	}
	if byteAt(rest, j) != ')' {
		return nil
	}
	at[1] = j + 1
	return at
}

// titleClosers closes a link title by what opened it.
var titleClosers = map[byte]byte{'"': '"', '\'': '\'', '(': ')'}

// byteAt is s[i], or 0 past its end.
func byteAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

// blanks is the index of the first byte of s from i that is not a space or a
// tab.
func blanks(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// reference reports whether the bracket closing at column end makes a link by
// reference, [text][label], [text][] or [text], with a label that is defined,
// and how far its [label] runs past the bracket.
func (in *inlineLinks) reference(o opener, last, end int, rest string) (int, string, bool) {
	if len(in.labels) == 0 {
		return 0, "", false
	}
	if at := referencePattern.FindStringSubmatchIndex(rest); at != nil {
		if label := rest[at[2]:at[3]]; label != "" {
			label = normalLabel(label)
			return at[1], label, in.labels[label]
		}
		text, ok := in.label(o, last, end)
		text = normalLabel(text)
		return at[1], text, ok && in.labels[text]
	}
	text, ok := in.label(o, last, end)
	text = normalLabel(text)
	return 0, text, ok && in.labels[text]
}

// label is a link's text where it is short enough to be a label, measured
// before it is joined, so a bracket open across a long paragraph costs no
// more than a label can be long.
func (in *inlineLinks) label(o opener, last, end int) (string, bool) {
	size := end - o.column
	if o.line != last {
		size = len(in.paragraph[o.line].Masked) - o.column + end
		for i := o.line + 1; i < last && size <= maxLabel; i++ {
			size += len(in.paragraph[i].Masked)
		}
	}
	if size > maxLabel {
		return "", false
	}
	return in.text(o, last, end), true
}

// startsBlock reports whether a line ends the paragraph before it. A list item
// does, and so does the first line of a quote; its later lines continue the
// quote, and are read from behind their markers like any other line.
func startsBlock(line ProseLine, paragraph []ProseLine) bool {
	if line.Item {
		return true
	}
	inner, quoted := stripQuotes(line.Masked)
	if quoted && (len(paragraph) == 0 || !isQuoted(paragraph[len(paragraph)-1].Masked)) {
		return true
	}
	return blockStartPattern.MatchString(inner)
}

// stripQuotes removes a line's blockquote markers, each with one space after.
func stripQuotes(line string) (string, bool) {
	quoted := false
	for {
		trimmed := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trimmed, ">") {
			return line, quoted
		}
		line, quoted = strings.TrimPrefix(trimmed[1:], " "), true
	}
}

func isQuoted(line string) bool {
	_, quoted := stripQuotes(line)
	return quoted
}

// text is a link's text from its opener to the closing bracket at column end
// of paragraph line last, its lines joined by "\n" as written, quote markers
// and indentation included.
func (in *inlineLinks) text(o opener, last, end int) string {
	if o.line == last {
		return in.paragraph[last].Masked[o.column:end]
	}
	parts := []string{strings.TrimSuffix(in.paragraph[o.line].Masked[o.column:], "\r")}
	for _, l := range in.paragraph[o.line+1 : last] {
		parts = append(parts, strings.TrimSuffix(l.Masked, "\r"))
	}
	return strings.Join(append(parts, in.paragraph[last].Masked[:end]), "\n")
}

// submatches renders the groups a FindStringSubmatchIndex result locates, an
// unmatched group as "".
func submatches(s string, at []int) []string {
	m := make([]string, len(at)/2)
	for i := range m {
		if at[2*i] >= 0 {
			m[i] = s[at[2*i]:at[2*i+1]]
		}
	}
	return m
}

// destinationAt locates the destination among a match's groups: the bracketed
// form is group n and the bare form group n+1, and exactly one of them matched.
func destinationAt(at []int, n int) (start, end int) {
	if at[2*n] >= 0 {
		return at[2*n], at[2*n+1]
	}
	return at[2*n+2], at[2*n+3]
}

// destination is a link's target, from whichever of the two forms matched, with
// percent-encoding decoded so that a path is compared as it is on disk.
//
// bracketed is the <...> form, which is a valid destination for any path and was
// being read as "<my" for every one of them; bare is the ordinary run. A path
// that does not decode is used as written, because that is what the author
// typed and reporting it that way is more use than reporting a decoding error.
func destination(bracketed, bare string) string {
	target := unescape(bare)
	if bracketed != "" {
		target = strings.NewReplacer(`\<`, "<", `\>`, ">").Replace(bracketed)
	}
	if decoded, err := url.PathUnescape(target); err == nil {
		return decoded
	}
	return target
}

// unescape resolves the backslash escapes in a bare destination, so
// foo\)baz.md names foo)baz.md.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// WikiLinks returns every [[...]] in the body, ignoring code.
func (d *Document) WikiLinks() []WikiLink {
	var links []WikiLink
	for line := range d.Prose() {
		for _, m := range wikiLinkPattern.FindAllStringSubmatch(line.Masked, -1) {
			links = append(links, WikiLink{Name: m[1], Line: line.Number, Heading: IsHeading(line.Masked)})
		}
	}
	return links
}

// AnchorsIn returns the heading anchors of arbitrary markdown, for checking a
// link into a file that is not itself a document.
func AnchorsIn(source []byte) []string {
	_, body, _, _ := splitFrontMatter(source)
	// A byte order mark is stripped from a front matter delimiter, and a file
	// with none kept it before its first heading.
	d := &Document{Body: strings.TrimPrefix(string(body), "\uFEFF"), BodyLine: 1}
	d.headings = parseHeadings(d.Body, d.BodyLine)
	d.anchorHeadings()
	return d.Anchors()
}

// Anchors is every fragment a link into the document can name: each
// heading's anchor, then each explicit one.
func (d *Document) Anchors() []string {
	anchors := make([]string, 0, len(d.headings))
	for _, h := range d.headings {
		anchors = append(anchors, h.Anchor)
	}
	return append(anchors, explicitAnchors(d.Body)...)
}

// explicitAnchors are the ids and names of <a> elements outside comments and
// code, their entities resolved. An explicit anchor is a target too:
// GLOSSARY.md writes one for every former name of a term, so a link written
// before a rename still resolves; without them L17 would report every one of
// those, and a frozen document holding such a link could never be repaired.
// An HTML block is not masked, so a comment within one is removed here, read
// across the block's lines as a browser reads it.
func explicitAnchors(body string) []string {
	masked, literal, _, item, _ := scanBody(body)
	masked = maskParagraphCode(masked, literal, item)
	var anchors []string
	for i := 0; i < len(masked); i++ {
		text := masked[i]
		switch literal[i] {
		case blockCode:
			continue
		case blockHTML:
			end := i + 1
			for end < len(masked) && literal[end] == blockHTML {
				end++
			}
			text = htmlComment.ReplaceAllString(strings.Join(masked[i:end], "\n"), "")
			i = end - 1
		}
		for _, m := range explicitAnchor.FindAllStringSubmatch(text, -1) {
			anchors = append(anchors, html.UnescapeString(m[1]+m[2]))
		}
	}
	return anchors
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// explicitAnchor matches an HTML anchor carrying an id or a name, which is how
// a Markdown document names a target that is not a heading.
var explicitAnchor = regexp.MustCompile(`(?i)<a\s+[^>]*\b(?:id|name)\s*=\s*(?:"([^"]+)"|'([^']+)')`)
