package repo

import (
	"fmt"
	"iter"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
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
	// What follows a link's closing bracket: the destination in either of
	// its two forms, <...>, which may hold spaces, and a bare run, then any
	// title.
	linkTailPattern = regexp.MustCompile(`^\(\s*(?:<([^<>\n]*)>|([^)\s]+))[^)]*\)`)
	// A line, read from behind any quote markers, that ends the paragraph
	// before it and so any link text left open there: a heading, a list item
	// at any depth, a table row, a thematic break or a setext underline.
	blockStartPattern = regexp.MustCompile(
		`^[ \t]*(?:(?:#{1,6}|[-*+]|\d{1,9}[.)])(?:[ \t]|\r?$)|\|)|^ {0,3}(?:=+|-+|(?:[*_-][ \t]*){3,})[ \t]*\r?$`)
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
		d.masked, d.literal, d.quoted, d.unclosed = scanBody(d.Body)
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
}

// Unclosed reports whether the body ends inside an HTML comment or a fenced
// code block.
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
	_, _, _, unclosed := scanBody(string(body))
	return unclosed
}

// Prose iterates the prose lines of a body: those not literal. Leaving the
// loop stops the walk.
func Prose(body string, bodyLine int) iter.Seq[ProseLine] {
	return func(yield func(ProseLine) bool) {
		raw := strings.Split(body, "\n")
		masked, literal, _, _ := scanBody(body)
		masked = maskParagraphCode(masked, literal)
		for i := range masked {
			if literal[i] {
				continue
			}
			if !yield(ProseLine{Raw: raw[i], Masked: masked[i], Number: bodyLine + i}) {
				return
			}
		}
	}
}

// maskParagraphCode blanks the code spans in each run of prose lines that
// forms one paragraph, since a span may cross a line break within one: read a
// line at a time, a span wrapped onto the next line left its links readable.
// A heading, a list item and the other block starts begin a run of their own.
func maskParagraphCode(masked []string, literal []bool) []string {
	out := slices.Clone(masked)
	joins := func(i int) bool {
		inner, _ := stripQuotes(masked[i])
		return !literal[i] && strings.TrimSpace(inner) != "" && !IsHeading(inner)
	}
	for start := 0; start < len(masked); {
		end := start + 1
		if joins(start) {
			for end < len(masked) && joins(end) && !startsBlock(masked[end], nil) {
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
		masked, literal, _, _ := scanBody(body)
		for i, line := range masked {
			if literal[i] {
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
// marks the lines a blockquote holds, literal or not.
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
func scanBody(text string) (masked []string, literal, quoted []bool, unclosed bool) {
	lines := strings.Split(text, "\n")
	masked = make([]string, len(lines))
	literal = make([]bool, len(lines))
	quoted = make([]bool, len(lines))

	var w blockWalk
	open := false
	for i, line := range lines {
		if open {
			// Past the marker that closes it, a comment opens only as one
			// inside a paragraph does.
			masked[i], open = maskCommentsIn(line, true, func(marker int) bool {
				return closesInParagraph(lines, i, marker+len(commentOpen))
			})
			w.paragraph = false
			continue
		}
		at, lit := w.line(strings.TrimSuffix(line, "\r"))
		quoted[i] = len(w.stack) > 0 && w.stack[0].quote
		if lit {
			masked[i], literal[i] = line, true
			continue
		}
		masked[i], open = maskCommentsIn(line, false, func(marker int) bool {
			// A comment starting a block runs until it closes, however far
			// that is. One opened inside a paragraph is raw HTML only where
			// it closes before the paragraph ends, and text otherwise.
			if marker-at <= 3 && strings.TrimSpace(line[at:marker]) == "" {
				return true
			}
			return closesInParagraph(lines, i, marker+len(commentOpen))
		})
		w.prose(masked[i][min(at, len(masked[i])):])
	}
	// open means a comment never closed, and unclosed a block that runs to the
	// end of the text. Either way the walk masked everything from there on,
	// so the parse that follows is a prefix of what the author wrote.
	return masked, literal, quoted, open || w.unclosed()
}

// closesInParagraph reports whether a comment opened at column from of line i
// closes before its paragraph ends at the next blank line.
func closesInParagraph(lines []string, i, from int) bool {
	if strings.Contains(lines[i][from:], commentClose) {
		return true
	}
	for _, line := range lines[i+1:] {
		if strings.TrimSpace(line) == "" {
			return false
		}
		if strings.Contains(line, commentClose) {
			return true
		}
	}
	return false
}

// blockWalk follows the blocks of a body a line at a time: the blockquotes
// and list items each line continues, and the leaf block it is in.
//
// It reads as much of CommonMark as deciding what is prose needs: containers
// by their markers and indentation, lazy continuation of a paragraph, and
// which leaves cannot interrupt one. Tabs count as four columns.
type blockWalk struct {
	stack []container
	leaf  leafKind
	// depth is len(stack) when the leaf opened. A leaf ends with the
	// container holding it.
	depth     int
	fenceChar byte
	fenceLen  int
	// htmlEnd closes an HTML block that ends on a marker; empty, a blank line
	// closes it.
	htmlEnd   string
	paragraph bool
}

// container is a blockquote, or a list item whose content starts indent
// columns in from where its parent's content starts.
type container struct {
	quote  bool
	indent int
}

type leafKind int

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
	htmlRawPattern   = regexp.MustCompile(`(?i)^<(script|pre|style|textarea)(?:[ \t>]|$)`)
	htmlOtherPattern = regexp.MustCompile(`^<(?:\?|!\[CDATA\[|![A-Za-z])`)
	htmlBlockPattern = regexp.MustCompile(`(?i)^</?(?:address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(?:[ \t]|/?>|$)`)
	htmlTagPattern   = regexp.MustCompile(`^(?:<[A-Za-z][A-Za-z0-9-]*(?:[ \t]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t]*=[ \t]*(?:[^ \t"'=<>` + "`" + `]+|'[^']*'|"[^"]*"))?)*[ \t]*/?>|</[A-Za-z][A-Za-z0-9-]*[ \t]*>)[ \t]*$`)
)

// line reads one line, without its carriage return, and returns the column
// its content starts at once its containers' markers are passed, and whether
// it is literal.
func (w *blockWalk) line(s string) (at int, literal bool) {
	matched := 0
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
		n, ok := consumeIndent(rest, w.stack[matched].indent)
		if !ok {
			break
		}
		at += n
	}
	if matched < len(w.stack) {
		if w.leaf == leafNone && w.paragraph && strings.TrimSpace(s[at:]) != "" && !w.interrupts(s[at:]) {
			return at, false // a lazy continuation of the paragraph
		}
		w.stack = w.stack[:matched]
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
		return at, true
	case leafHTML:
		if w.htmlEnd == "" && strings.TrimSpace(rest) == "" {
			w.leaf, w.paragraph = leafNone, false
			return at, false
		}
		if w.htmlEnd != "" && strings.Contains(strings.ToLower(rest), w.htmlEnd) {
			w.leaf = leafNone
		}
		return at, true
	case leafIndented:
		if strings.TrimSpace(rest) == "" {
			return at, false
		}
		if indentColumns(rest) >= 4 {
			return at, true
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
		if n, indent, ok := w.listItem(rest); ok {
			w.stack = append(w.stack, container{indent: indent})
			at, w.paragraph = at+n, false
			continue
		}
		break
	}

	rest = s[at:]
	switch {
	case strings.TrimSpace(rest) == "":
		w.paragraph = false
		return at, false
	case fenceOpens(rest):
		m := fencePattern.FindStringSubmatch(rest)
		w.open(leafFence)
		w.fenceChar, w.fenceLen = m[1][0], len(m[1])
		return at, true
	case !w.paragraph && indentColumns(rest) >= 4:
		w.open(leafIndented)
		return at, true
	}
	if end, ok := htmlStarts(rest, w.paragraph); ok {
		w.open(leafHTML)
		w.htmlEnd = end
		if end != "" && strings.Contains(strings.ToLower(rest)[2:], end) {
			w.leaf = leafNone
		}
		return at, true
	}
	return at, false
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
	return w.leaf == leafFence || w.leaf == leafHTML && w.htmlEnd != ""
}

// interrupts reports whether a line starts a block, so that it cannot be a
// lazy continuation of the paragraph before it.
func (w *blockWalk) interrupts(s string) bool {
	_, _, item := w.listItem(s)
	_, html := htmlStarts(s, true)
	return item || html || quoteMarkerPattern.MatchString(s) || fenceOpens(s) || IsHeading(s) ||
		thematicPattern.MatchString(s) || strings.HasPrefix(strings.TrimLeft(s, " "), commentOpen)
}

// listItem reads a list marker: how many bytes it and the spaces after it
// take, and the column the item's content starts at. A thematic break is not
// an item, and inside a paragraph neither is an empty item or a numbered one
// not starting at 1.
func (w *blockWalk) listItem(s string) (n, indent int, ok bool) {
	m := listMarkerPattern.FindStringSubmatchIndex(s)
	if m == nil || thematicPattern.MatchString(s) {
		return 0, 0, false
	}
	empty := m[1] == len(s)
	if w.paragraph && (empty || m[6] >= 0 && strings.TrimLeft(s[m[6]:m[7]], "0") != "1") {
		return 0, 0, false
	}
	marker := m[5]
	spaces := indentColumns(s[marker:])
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

// htmlStarts reports an HTML block opening a line, and the marker that closes
// it, empty where a blank line does.
func htmlStarts(s string, paragraph bool) (end string, ok bool) {
	if indentColumns(s) > 3 {
		return "", false
	}
	s = strings.TrimLeft(s, " ")
	if m := htmlRawPattern.FindStringSubmatch(s); m != nil {
		return "</" + strings.ToLower(m[1]) + ">", true
	}
	switch m := htmlOtherPattern.FindString(s); m {
	case "":
	case "<?":
		return "?>", true
	case "<![CDATA[":
		return "]]>", true
	default:
		return ">", true
	}
	if htmlBlockPattern.MatchString(s) || !paragraph && htmlTagPattern.MatchString(s) {
		return "", true
	}
	return "", false
}

// indentColumns is the width of a line's leading whitespace.
func indentColumns(s string) int {
	cols := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ':
			cols++
		case '\t':
			cols += 4
		default:
			return cols
		}
	}
	return cols
}

// consumeIndent passes want columns of leading whitespace and returns the bytes
// that took, or false when the line has fewer.
func consumeIndent(s string, want int) (int, bool) {
	cols := 0
	for i := 0; cols < want; i++ {
		if i == len(s) {
			return 0, false
		}
		switch s[i] {
		case ' ':
			cols++
		case '\t':
			cols += 4
		default:
			return 0, false
		}
		if cols >= want {
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
	masked, _, _, _ := scanBody(text)
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
	taken := map[string]int{}

	for i, line := range proseLines(body) {
		m := headingPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// A heading of nothing but a closing sequence is empty.
		text := ""
		if strings.Trim(m[2], "#") != "" {
			text = strings.TrimSpace(closingHashes.ReplaceAllString(m[2], ""))
		}
		h := Heading{
			Level: len(m[1]),
			Text:  text,
			Line:  bodyLine + i,
			index: i,
		}
		h.Anchor = anchor(text, taken)
		headings = append(headings, h)
	}
	return headings
}

// Anchor is the fragment GitHub would generate for a heading standing on its
// own. Headings inside a document take their anchors from a walk that numbers
// repeats, so this is only for naming the anchor a heading would have had when
// the heading is not there to ask.
func Anchor(text string) string { return anchor(text, map[string]int{}) }

// anchor renders a heading the way GitHub does: link syntax reduced to its
// text, everything but letters, numbers, underscores, hyphens and spaces
// removed, lowercased, spaces hyphenated, and a numeric suffix when the same
// anchor has already been used in this document. taken is updated in place.
func anchor(text string, taken map[string]int) string {
	rendered := linkPattern.ReplaceAllString(text, "$1")

	var b strings.Builder
	for _, r := range rendered {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_', r == '-':
			b.WriteRune(unicode.ToLower(r))
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

// LinkType is the form a link was written in. An image resolves like any other
// link and renders as something else entirely, so a consumer rewriting
// destinations has to tell them apart.
type LinkType string

const (
	LinkInline     LinkType = "inline"
	LinkImage      LinkType = "image"
	LinkDefinition LinkType = "definition"
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
}

// Links returns every markdown link in the body, ignoring code, in the order
// their destinations appear. A link reference definition counts: its
// destination is a target like any other, and nothing was checking it. A
// footnote, [^label]:, is GitHub's and holds prose, so it is not one.
func (d *Document) Links() []Link {
	links := d.definitions()
	labels := map[string]bool{}
	for _, l := range links {
		labels[normalLabel(l.Text)] = true
	}
	inline := inlineLinks{labels: labels}
	for line := range d.Prose() {
		links = append(links, inline.read(line)...)
	}
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Line != links[j].Line {
			return links[i].Line < links[j].Line
		}
		return links[i].Start < links[j].Start
	})
	return links
}

// definitions returns the link reference definitions, which are read first
// because a reference may come before the definition it uses.
func (d *Document) definitions() []Link {
	var links []Link
	opensBlock := true
	quoted := d.questionQuotes()
	for line := range d.Prose() {
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
		opensBlock = strings.TrimSpace(content) == ""
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
// is not one. A reference use is not returned; its definition is.
type inlineLinks struct {
	// labels are the document's definitions, normalised.
	labels map[string]bool
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
	if gap || strings.TrimSpace(line.Masked) == "" || startsBlock(line.Masked, in.paragraph) {
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
			} else if skip, ok := in.reference(o, here, i, rest); ok && !inactive {
				if !o.image {
					in.inactiveBelow = len(in.open)
				}
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
func tail(rest string, closes bool) []int {
	if !closes {
		return nil
	}
	return linkTailPattern.FindStringSubmatchIndex(rest)
}

// reference reports whether the bracket closing at column end makes a link by
// reference, [text][label], [text][] or [text], with a label that is defined,
// and how far its [label] runs past the bracket.
func (in *inlineLinks) reference(o opener, last, end int, rest string) (int, bool) {
	if len(in.labels) == 0 {
		return 0, false
	}
	if at := referencePattern.FindStringSubmatchIndex(rest); at != nil {
		if label := rest[at[2]:at[3]]; label != "" {
			return at[1], in.labels[normalLabel(label)]
		}
		text, ok := in.label(o, last, end)
		return at[1], ok && in.labels[normalLabel(text)]
	}
	text, ok := in.label(o, last, end)
	return 0, ok && in.labels[normalLabel(text)]
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

// startsBlock reports whether a line ends the paragraph before it. The first
// line of a quote does; its later lines continue the quote, and are read from
// behind their markers like any other line.
func startsBlock(line string, paragraph []ProseLine) bool {
	inner, quoted := stripQuotes(line)
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
	target := bare
	if bracketed != "" {
		target = strings.NewReplacer(`\<`, "<", `\>`, ">").Replace(bracketed)
	}
	if decoded, err := url.PathUnescape(target); err == nil {
		return decoded
	}
	return target
}

// WikiLinks returns every [[...]] in the body, ignoring code.
func (d *Document) WikiLinks() []WikiLink {
	var links []WikiLink
	for line := range d.Prose() {
		for _, m := range wikiLinkPattern.FindAllStringSubmatch(line.Masked, -1) {
			links = append(links, WikiLink{Name: m[1], Line: line.Number})
		}
	}
	return links
}

// AnchorsIn returns the heading anchors of arbitrary markdown, for checking a
// link into a file that is not itself a document.
func AnchorsIn(source []byte) []string {
	_, body, _, _ := splitFrontMatter(source)
	headings := parseHeadings(string(body), 1)
	anchors := make([]string, 0, len(headings))
	for _, h := range headings {
		anchors = append(anchors, h.Anchor)
	}
	// An explicit anchor is a target too. GLOSSARY.md writes one for every
	// former name of a term, so a link written before a rename still resolves;
	// without this L17 would report every one of them, and a frozen document
	// holding such a link could never be repaired.
	for _, m := range explicitAnchor.FindAllStringSubmatch(string(body), -1) {
		anchors = append(anchors, m[2])
	}
	return anchors
}

// explicitAnchor matches an HTML anchor carrying an id or a name, which is how
// a Markdown document names a target that is not a heading.
var explicitAnchor = regexp.MustCompile(`(?i)<a\s+[^>]*\b(id|name)\s*=\s*"([^"]+)"`)
