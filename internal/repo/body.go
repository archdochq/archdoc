package repo

import (
	"fmt"
	"iter"
	"net/url"
	"regexp"
	"slices"
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

var (
	headingPattern  = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*$`)
	fencePattern    = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	linkPattern     = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	closingHashes   = regexp.MustCompile(`[ \t]+#+$`)
	wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)
	// Two destination forms: <...>, which may hold spaces, and a bare run.
	mdLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(\s*(?:<([^<>\n]*)>|([^)\s]+))[^)]*\)`)
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
		`^[ \t]{0,3}\[([^\]]+)\]:[ \t]*(?:<([^<>\n]*)>|(\S+))(?:[ \t]+(?:"[^"]*"|'[^']*'|\([^)]*\)))?[ \t]*$`)
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
	// WikiLinkSubmatchPattern is the same rule with the name captured, for a
	// caller that needs to read the name out rather than only blank the span.
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
			"Backwards compatibility", "Open questions", "Changelog",
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

// sectionAt returns the heading at index i and everything beneath it, up to the
// next heading of equal or higher level. Callers that already know which
// heading they mean use this rather than Section, which searches by title and
// would hand every duplicate the first one's body.
func (d *Document) sectionAt(i int) Section {
	lines, masked := d.bodyLines(), d.maskedLines()
	h := d.headings[i]
	end := len(lines)
	for _, next := range d.headings[i+1:] {
		if next.Level <= h.Level {
			end = next.index
			break
		}
	}
	return Section{
		Heading: h,
		Text:    strings.Join(lines[h.index+1:end], "\n"),
		masked:  strings.Join(masked[h.index+1:end], "\n"),
	}
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
		d.masked, _, d.unclosed = scanBody(d.Body)
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
	_, _, unclosed := scanBody(string(body))
	return unclosed
}

// Prose iterates the prose lines of a body: those not inside a fenced code
// block or an HTML comment. Leaving the loop stops the walk.
func Prose(body string, bodyLine int) iter.Seq[ProseLine] {
	return func(yield func(ProseLine) bool) {
		// proseLines yields the masked line, so the line as written is taken
		// from the body at the same index.
		raw := strings.Split(body, "\n")
		for i, masked := range proseLines(body) {
			if !yield(ProseLine{Raw: raw[i], Masked: maskCode(masked), Number: bodyLine + i}) {
				return
			}
		}
	}
}

// Prose is the same over one document's body.
func (d *Document) Prose() iter.Seq[ProseLine] { return Prose(d.Body, d.BodyLine) }

// maskCode blanks code spans, keeping the length so that any position derived
// from the line still refers to the right column.
func maskCode(line string) string {
	out := []byte(line)
	for i := 0; i < len(line); {
		start, end, ok := codeSpan(line[i:])
		if !ok {
			break
		}
		blank(out, i+start, i+end)
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

// proseLines iterates the body lines that are not inside a fenced code block,
// with their zero-based index and with everything inside an HTML comment
// blanked. A fence closes only on a run of the same character at least as long
// as the one that opened it, so a shorter example nested inside a longer block
// does not end it.
func proseLines(body string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		masked, fenced, _ := scanBody(body)
		for i, line := range masked {
			if fenced[i] {
				continue
			}
			if !yield(i, line) {
				return
			}
		}
	}
}

// scanBody splits text into lines, blanks the HTML comments in each, and
// reports which lines belong to a fenced code block.
//
// It is the single definition of how fences, code spans and comments interact,
// and every consumer reads it rather than restating the rule. There used to be
// a second definition, a regular expression requiring a closing marker, behind
// Section.Empty and the glossary's paragraph split. It disagreed with this one
// about an unterminated comment, and that disagreement was enough to carry an
// unfinished document through the freeze gate in silence.
//
// A fenced block is literal to its end, so nothing in it is prose and nothing
// in it opens a comment. A fence marker can only begin a line, so a line
// already inside a comment cannot open one: a comment and a fenced block are
// both blocks, and whichever opens first holds the lines until it closes.
func scanBody(text string) (masked []string, fenced []bool, unclosed bool) {
	lines := strings.Split(text, "\n")
	masked = make([]string, len(lines))
	fenced = make([]bool, len(lines))

	var fenceChar byte
	var fenceLen int
	open := false

	for i, line := range lines {
		switch {
		case fenceLen != 0:
			masked[i], fenced[i] = line, true
			if closesFence(line, fenceChar, fenceLen) {
				fenceLen = 0
			}
			continue

		case !open:
			if m := fencePattern.FindStringSubmatch(line); m != nil {
				fenceChar, fenceLen = m[1][0], len(m[1])
				masked[i], fenced[i] = line, true
				continue
			}
		}
		masked[i], open = maskCommentsIn(line, open)
	}
	// open means a comment never closed; fenceLen means a fence never did.
	// Either way the walk masked everything from there to the end of the text,
	// so the parse that follows is a prefix of what the author wrote.
	return masked, fenced, open || fenceLen != 0
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
	masked, _, _ := scanBody(text)
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
// swallowed the rest of the document. An unterminated comment does run to the
// end of the body, which is how a browser reads one.
func maskCommentsIn(line string, open bool) (string, bool) {
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
		i, open = i+start, true
	}
	return string(out), open
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
		text := strings.TrimSpace(closingHashes.ReplaceAllString(m[2], ""))
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

// anchor renders a heading the way GitHub does: link syntax reduced to its
// text, everything but letters, numbers, underscores, hyphens and spaces
// removed, lowercased, spaces hyphenated, and a numeric suffix when the same
// anchor has already been used in this document. taken is updated in place.
// Anchor is the fragment GitHub would generate for a heading standing on its
// own. Headings inside a document take their anchors from a walk that numbers
// repeats, so this is only for naming the anchor a heading would have had when
// the heading is not there to ask.
func Anchor(text string) string { return anchor(text, map[string]int{}) }

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

// Link is a relative markdown link found in a body.
type Link struct {
	Text   string
	Target string
	// Line is 1-based within the file.
	Line int
}

// WikiLink is an unexpanded [[...]] reference.
type WikiLink struct {
	Name string
	Line int
}

// Links returns every markdown link in the body, ignoring code. A link
// reference definition counts: its destination is a target like any other, and
// nothing was checking it.
func (d *Document) Links() []Link {
	var links []Link
	opensBlock := true
	for line := range d.Prose() {
		for _, m := range mdLinkPattern.FindAllStringSubmatch(line.Masked, -1) {
			links = append(links, Link{Text: m[1], Target: destination(m[2], m[3]), Line: line.Number})
		}
		// A definition cannot interrupt a paragraph, so it only counts after a
		// blank line or another definition.
		if opensBlock {
			if m := definitionPattern.FindStringSubmatch(line.Masked); m != nil {
				links = append(links, Link{Text: m[1], Target: destination(m[2], m[3]), Line: line.Number})
				continue
			}
		}
		opensBlock = strings.TrimSpace(line.Masked) == ""
	}
	return links
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
	anchors := make([]string, len(headings))
	for i, h := range headings {
		anchors[i] = h.Anchor
	}
	return anchors
}
