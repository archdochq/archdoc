// Package link resolves wiki links and suggests new ones. Both operate on
// lines and never touch fenced code, inline code, front matter or any heading.
package link

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/archdochq/archdoc/internal/lint"
	"github.com/archdochq/archdoc/internal/repo"
)

// Changes maps a document path to its rewritten source. A document that needed
// no change, or that could not be rewritten, is absent.
type Changes map[string][]byte

// Suggestion is a proposed link over text that is already written.
type Suggestion struct {
	Path string
	Line int
	// Text is the matched text, exactly as the document has it.
	Text string
	// Replacement wraps Text without altering it: prose that is already
	// written is not the tool's to edit.
	Replacement string
	// Col is the byte offset of Text within the line, so that Apply rewrites
	// the occurrence that was matched rather than the first one on the line.
	Col int
	// LineText is the whole line as it was when the suggestion was made, shown
	// to the user and checked by Apply before anything is rewritten.
	LineText string
}

var (
	// repo owns the wiki-link rule; this was a third spelling of it.
	wikiLink = repo.WikiLinkSubmatchPattern
	// Identifiers are matched only in their canonical form. Lower case in
	// prose is more likely a filename fragment than a reference.
	identifier = regexp.MustCompile(`\b(?:RFC|ADR|REF)-\d{4}\b`)
)

// target is something a name can resolve to.
type target struct {
	// key identifies what was resolved, independent of how it was spelled, so
	// that one target is suggested once per document.
	key string
	// expansion is what [[X]] becomes, which supplies the title because the
	// author wrote a placeholder expecting it.
	expansion string
	// wrap is what a bare mention becomes, which does not.
	wrap string
}

// Resolve expands every [[X]] in the editable documents: non-terminal RFCs and
// ADRs, every spec page, every ref. A document holding a link that resolves to
// nothing is left alone and reported, so one bad link never half-rewrites a
// file.
func Resolve(ctx lint.Context) (Changes, []lint.Finding) {
	changes := Changes{}
	var findings []lint.Finding

	for _, d := range ctx.Repo().Documents() {
		links := d.WikiLinks()
		if len(links) == 0 {
			continue
		}
		if state, severity, ok := rewritable(ctx, d); !ok {
			findings = append(findings, lint.Finding{
				Path: d.Path, Line: links[0].Line, Severity: severity, Rule: "link",
				Message: fmt.Sprintf("%s document contains [[%s]] and is not rewritten", state, links[0].Name),
			})
			continue
		}

		var unresolved []lint.Finding
		rewritten := rewrite(d.Body, d.BodyLine, func(line repo.ProseLine) string {
			// A heading is left exactly as written, as Suggest leaves it: L10
			// compares the H1's raw text against the front matter, and L08, L09
			// and the freeze gate compare an H2's against the required list, so
			// expanding a link inside one changes what all four read. L16 still
			// reports the unresolved link, which is the right outcome: it has to
			// be rewritten by hand or not written there.
			if repo.IsHeading(line.Masked) {
				return line.Raw
			}

			// Matches are found in the masked line, so a [[X]] inside a code
			// span is never seen, and spliced into the raw line by offset, so
			// the code span is never disturbed.
			var b strings.Builder
			last := 0
			for _, m := range wikiLink.FindAllStringSubmatchIndex(line.Masked, -1) {
				name := line.Raw[m[2]:m[3]]
				found, ok := resolve(ctx.Repo(), d, name)
				if !ok {
					unresolved = append(unresolved, lint.Finding{
						Path: d.Path, Line: line.Number, Severity: lint.Error, Rule: "link",
						Message: fmt.Sprintf("[[%s]] matches no identifier and no glossary term", name),
					})
					continue
				}
				b.WriteString(line.Raw[last:m[0]])
				b.WriteString(found.expansion)
				last = m[1]
			}
			b.WriteString(line.Raw[last:])
			return b.String()
		})
		if len(unresolved) > 0 {
			findings = append(findings, unresolved...)
			continue
		}
		changes[d.Path] = replaceBody(d, rewritten)
	}
	return changes, findings
}

// Suggest proposes links over bare identifiers and glossary terms that are not
// already inside a link or a code span, one per distinct target per document.
// The document's own identifier, every heading, front matter, code and the
// glossary's own entries are excluded.
func Suggest(r *repo.Repo) []Suggestion {
	patterns := termPatterns(r)
	var suggestions []Suggestion

	for _, d := range r.Documents() {
		if !open(d) || d.Page == repo.GlossaryPage {
			continue
		}
		offered := map[string]bool{}

		for line := range d.Prose() {
			// Headings are excluded, as is anything already inside a link: a
			// suggestion over a link would nest one inside another.
			//
			// Every heading, not just the H1. A section's identity is its raw
			// heading text, compared with == by L08, L09, L14 and the freeze
			// gate, and anchor() is the only place that renders it first. So
			// wrapping an H2 in a link left the anchor resolving and deleted
			// the section from every rule at once: a repository that linted
			// clean reported a missing required section, and a proposed RFC
			// with an open question was accepted because "Open questions" was
			// no longer that.
			if repo.IsHeading(line.Masked) {
				continue
			}
			masked := maskLinks(line.Masked)

			for _, m := range matches(patterns, masked) {
				text := line.Raw[m.start:m.end]
				found, ok := resolve(r, d, text)
				// Keyed on what the text resolves to, not how it was spelled,
				// so Wings, wings and WINGS are one suggestion.
				if !ok || found.key == d.ID || offered[found.key] {
					continue
				}
				offered[found.key] = true
				suggestions = append(suggestions, Suggestion{
					Path: d.Path, Line: line.Number, Text: text, Col: m.start,
					Replacement: found.wrap, LineText: line.Raw,
				})
			}
		}
	}
	return suggestions
}

// maskLinks blanks every span a suggestion must not be offered inside, keeping
// their length so offsets still line up with the raw line. Wiki links are
// masked too: wrapping text inside [[...]] leaves markup that no longer matches
// the wiki-link pattern, so the unresolved link it was reporting disappears.
func maskLinks(line string) string {
	blank := func(m string) string { return strings.Repeat(" ", len(m)) }
	for _, pattern := range []*regexp.Regexp{
		// referenceDefinition first: it needs the destination still there to
		// match at all, and bareURL blanks exactly that. Ordered the other way
		// the definition's label stayed exposed, a term standing in it was
		// offered, and --apply spliced an inline link into the label, which
		// degrades every [text][label] in the repository to plain text with
		// lint blind to it.
		referenceDefinition,
		repo.WikiLinkPattern, repo.MarkdownLinkPattern, repo.ReferenceLinkPattern,
		bracketed, bareURL, autolink,
	} {
		line = pattern.ReplaceAllStringFunc(line, blank)
	}

	// Raw HTML, and a bracket still unmatched after all that. Both mean the line
	// holds something a per-line masker cannot see the extent of: an HTML block,
	// whose contents markdown does not re-parse, or a link whose label wraps
	// onto another line. The whole line is withheld rather than guessed at, on
	// the trade this function already makes: over-masking costs one missed
	// suggestion, and getting it wrong writes markdown source into the middle of
	// an attribute, or a link inside a link, with nothing afterwards able to see
	// either.
	//
	// A blank line ends an HTML block, so prose between two of them carries no
	// tag and is still offered.
	if htmlTag.MatchString(line) || strings.Count(line, "[") != strings.Count(line, "]") {
		return strings.Repeat(" ", len(line))
	}
	return line
}

// htmlTag is an opening, closing or self-closing raw HTML tag.
var htmlTag = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9-]*(?:\s[^>]*)?/?>`)

// A URL is not prose, and neither is the target of a link reference
// definition. Splicing a suggestion into the middle of one destroys it
// silently: lint parses only markdown links, so a mangled bare URL is not a
// link to any rule, and the relative link spliced into it is itself valid.
var (
	bareURL  = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*://\S+`)
	autolink = regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9+.\-]*:[^>\s]*>`)
	// The whole line, title included: a parenthesised title after the
	// destination was left exposed and a term standing in it was rewritten,
	// which destroys the definition.
	referenceDefinition = regexp.MustCompile(`^[ \t]*\[[^\]]*\]:.*$`)
	// Any bracketed span, which covers a shortcut reference link ([Label]
	// resolved against a definition elsewhere) as well as the label half of
	// every other form. Nothing inside brackets is prose worth suggesting over.
	bracketed = regexp.MustCompile(`!?\[[^\]\n]*\]`)
)

// Apply rewrites one document's source with the suggestions accepted for it.
// A suggestion for another path, or one whose line has changed since it was
// made, is skipped rather than applied to whatever is there now.
func Apply(path string, source []byte, accepted []Suggestion) []byte {
	lines := strings.Split(string(source), "\n")

	// Grouped by line, because the staleness check compares against the line as
	// it was when the suggestion was made. Checking it per suggestion would
	// pass for the first and fail for every sibling, since the first splice has
	// already changed the line: that silently dropped all but one of them.
	byLine := map[int][]Suggestion{}
	for _, s := range accepted {
		if s.Path == path {
			byLine[s.Line] = append(byLine[s.Line], s)
		}
	}

	for _, line := range slices.Sorted(maps.Keys(byLine)) {
		i := line - 1
		if i < 0 || i >= len(lines) {
			continue
		}
		onLine := byLine[line]
		// The line must be the one the suggestions were computed against; if
		// something else has rewritten it, none of them apply.
		if lines[i] != onLine[0].LineText {
			continue
		}
		// Right to left, so an earlier splice does not shift a later offset.
		slices.SortFunc(onLine, func(a, b Suggestion) int { return cmp.Compare(b.Col, a.Col) })
		for _, s := range onLine {
			if s.Col < 0 || s.Col+len(s.Text) > len(lines[i]) || lines[i][s.Col:s.Col+len(s.Text)] != s.Text {
				continue
			}
			lines[i] = lines[i][:s.Col] + s.Replacement + lines[i][s.Col+len(s.Text):]
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// span is where a candidate was found in a line.
type span struct{ start, end int }

// termPatterns compiles one matcher per glossary term, longest first so that
// "Spec page" wins over "Spec", with equal lengths ordered alphabetically so
// the result is stable. QuoteMeta guarantees each expression compiles, so a
// failure here would be a bug rather than a condition to handle.
func termPatterns(r *repo.Repo) []termPattern {
	entries, ok := r.Glossary()
	if !ok {
		return nil
	}
	terms := make([]string, 0, len(entries))
	for _, e := range entries {
		// An empty heading on the glossary page is a term with nothing in it,
		// and QuoteMeta("") compiles to a pattern matching the empty string at
		// every offset, so every prose line matched at zero and --apply wrote
		// "[](glossary.md#)" into it. L15 reports the entry separately.
		if strings.TrimSpace(e.Term) == "" {
			continue
		}
		terms = append(terms, e.Term)
	}
	slices.SortFunc(terms, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), strings.Compare(a, b))
	})
	patterns := make([]termPattern, len(terms))
	for i, term := range terms {
		// No \b: see standsAlone, which the caller applies instead.
		patterns[i] = termPattern{
			re:   regexp.MustCompile(`(?i)` + regexp.QuoteMeta(term)),
			lead: firstWord(term),
		}
	}
	return patterns
}

// termPattern is one term's matcher together with the first word of the term,
// which is what lets a line skip the terms it cannot contain.
type termPattern struct {
	re *regexp.Regexp
	// lead is the term's first run of letters or digits, lowercased, or empty
	// when the term has none. A term can only occur in a line that contains its
	// first word as a whole word, and running every pattern over every line
	// instead made suggesting linear in the size of the glossary: on 2000 terms
	// it was most of a minute, nearly all of it inside the regexp engine.
	lead string
}

// firstWord is the first run of letters or digits in s, lowercased.
func firstWord(s string) string {
	start := -1
	for i, r := range s {
		switch {
		case isWordRune(r) && start < 0:
			start = i
		case !isWordRune(r) && start >= 0:
			return strings.ToLower(s[start:i])
		}
	}
	if start < 0 {
		return ""
	}
	return strings.ToLower(s[start:])
}

// wordsIn is the set of whole words in a line, lowercased, built the same way
// firstWord reads a term so that the two agree about what a word is.
func wordsIn(line string) map[string]bool {
	words := map[string]bool{}
	start := -1
	for i, r := range line {
		switch {
		case isWordRune(r) && start < 0:
			start = i
		case !isWordRune(r) && start >= 0:
			words[strings.ToLower(line[start:i])] = true
			start = -1
		}
	}
	if start >= 0 {
		words[strings.ToLower(line[start:])] = true
	}
	return words
}

// matches finds every candidate in a line, left to right. A longer term claims
// its span before a shorter one inside it is considered.
func matches(patterns []termPattern, line string) []span {
	var found []span
	remaining := line
	claim := func(m []int) {
		found = append(found, span{m[0], m[1]})
		remaining = remaining[:m[0]] + strings.Repeat(" ", m[1]-m[0]) + remaining[m[1]:]
	}
	for _, m := range identifier.FindAllStringIndex(line, -1) {
		claim(m)
	}
	// The line's words, once, so that each term can be dismissed without
	// running its pattern. A term whose first word is absent cannot occur here,
	// because standsAlone would reject any match that was not a whole word.
	words := wordsIn(line)
	for _, pattern := range patterns {
		if pattern.lead != "" && !words[pattern.lead] {
			continue
		}
		// The first occurrence that stands alone, rather than the first
		// occurrence: a term glued to a longer word is not a use of the term.
		for _, m := range pattern.re.FindAllStringIndex(remaining, -1) {
			if standsAlone(remaining, m[0], m[1]) {
				claim(m)
				break
			}
		}
	}
	slices.SortFunc(found, func(a, b span) int { return cmp.Compare(a.start, b.start) })
	return found
}

// standsAlone reports whether a span is glued to no letter or digit on either
// side, which is what "whole" means for a term.
//
// This used to be \b on both ends of the pattern. Go's \b is an ASCII word
// boundary, a transition between [0-9A-Za-z_] and anything else, and that is
// the wrong question for a term whose own edges are not word characters. For
// C++ it inverted: it failed where the term stood alone between spaces, since
// neither side is a word character and so there is no transition, and matched
// inside C++11, where there is one. Reading the adjacent runes asks the
// question directly, and unicode.IsLetter covers Café and Cafés, which an
// ASCII class cannot.
func standsAlone(line string, start, end int) bool {
	if r, size := utf8.DecodeLastRuneInString(line[:start]); size > 0 && isWordRune(r) {
		return false
	}
	if r, size := utf8.DecodeRuneInString(line[end:]); size > 0 && isWordRune(r) {
		return false
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// resolve finds what a name refers to. A glossary term wins over an identifier,
// as the spec requires, because a term is what a reader is more likely to mean.
func resolve(r *repo.Repo, from *repo.Document, name string) (target, bool) {
	if e, ok := r.Term(name); ok {
		{
			href := repo.LinkDestination(relative(from.Path, r.ByPage(repo.GlossaryPage).Path)) + "#" + e.Anchor
			// The case the author wrote is kept; only the destination is ours.
			// A glossary term has no title to supply, so the two forms coincide.
			// Escaped, as the identifier branch escapes its title. A term
			// carrying a bracket otherwise closes the label early and the rest
			// renders as a live link of the term's own choosing.
			rendered := fmt.Sprintf("[%s](%s)", repo.EscapeMarkdown(name), href)
			return target{key: "term:" + strings.ToLower(e.Term), expansion: rendered, wrap: rendered}, true
		}
	}
	if d := r.ByID(name); d != nil {
		href := repo.LinkDestination(relative(from.Path, d.Path))
		return target{
			key:       d.ID,
			expansion: fmt.Sprintf("[%s: %s](%s)", name, repo.EscapeMarkdown(d.FrontMatter.Title), href),
			wrap:      fmt.Sprintf("[%s](%s)", name, href),
		}, true
	}
	return target{}, false
}

// relative renders the path from one document to another, as a link in the
// first must spell it: up to the nearest common directory, then down. Both
// paths are relative to root and always slash-separated, so filepath is not
// wanted here; on Windows it would produce backslashes in a markdown link.
func relative(from, to string) string {
	fromDir, toDir := path.Dir(from), path.Dir(to)
	up := segments(fromDir)
	down := segments(toDir)

	common := 0
	for common < len(up) && common < len(down) && up[common] == down[common] {
		common++
	}
	var out []string
	for range len(up) - common {
		out = append(out, "..")
	}
	out = append(out, down[common:]...)
	return path.Join(append(out, path.Base(to))...)
}

// segments splits a directory into its parts, treating "." as none.
func segments(dir string) []string {
	if dir == "." || dir == "" {
		return nil
	}
	return strings.Split(dir, "/")
}

// rewritable reports whether Resolve may write to a document, and where it may
// not, how to describe why and at what severity.
//
// Frozen, not merely terminal. PROCESS.md puts the freeze at the push, so a
// document that is terminal in the working tree and not yet on the branch is
// still editable. Refusing it left `archdoc new --backfill`, which creates an
// accepted document without passing the accept gate, in a state where L16
// reported an error reading "run archdoc link" and this command then refused
// the document: the tool named a command that could not act, on the ordinary
// backfilling path.
//
// The severity follows L16's exactly, so the two cannot disagree about the
// same fact: a warning once no permitted edit could clear the finding.
//
// Outside a git repository the question cannot be settled, so a terminal
// document is left alone rather than assumed editable, which is what this did
// before it could tell the two apart. The branch is consulted only for a
// terminal document, so a repository whose wiki links all sit in open
// documents still never builds the snapshot.
func rewritable(ctx lint.Context, d *repo.Document) (state string, severity lint.Severity, ok bool) {
	switch {
	case open(d):
		return "", "", true
	case ctx.Git() == nil:
		return "terminal", lint.Error, false
	case ctx.Frozen(d):
		return "frozen on " + ctx.Repo().Config().Branch, lint.Warning, false
	}
	return "", "", true
}

// open is a document still being worked on: a spec page, a ref, or an RFC or
// ADR that has not reached a terminal status in the working tree.
//
// Suggest keys on this rather than on frozen. Resolving a [[...]] repairs
// something lint reports as an error, so it is done to anything not frozen.
// Offering a new link over prose already written is an addition, and is not
// pressed on a document whose author has called it decided.
func open(d *repo.Document) bool {
	if d.Type.HasLifecycle() {
		return !d.FrontMatter.Status.Terminal()
	}
	return true
}

// rewrite maps every prose line of a body through f, leaving fenced blocks
// exactly as they are.
func rewrite(body string, bodyLine int, f func(repo.ProseLine) string) string {
	// Split once and rewrite in place: nothing else aliases this slice.
	rewritten := strings.Split(body, "\n")
	for line := range repo.Prose(body, bodyLine) {
		rewritten[line.Number-bodyLine] = f(line)
	}
	return strings.Join(rewritten, "\n")
}

// replaceBody puts a rewritten body back with its front matter.
func replaceBody(d *repo.Document, body string) []byte {
	if d.BodyLine <= 1 {
		// No front matter, so the body is the whole document. Rejoining a head
		// that does not exist would shift the file down a line.
		return []byte(body)
	}
	lines := strings.Split(string(d.Source), "\n")
	head := strings.Join(lines[:d.BodyLine-1], "\n")
	return []byte(head + "\n" + body)
}
