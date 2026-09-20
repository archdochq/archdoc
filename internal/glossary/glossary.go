// Package glossary edits spec/glossary.md. Parsing lives in repo; this package
// is the writing half. Every operation returns the rewritten page and leaves
// the file on disk alone.
package glossary

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ollieread/archdoc/internal/repo"
)

// truncated refuses to edit a page whose parse stops short of the file. Every
// writer here splices on a line range derived from that parse, so when the page
// ends inside an unterminated comment or an unclosed fence the range runs past
// the entries the parser could not see and the splice deletes them.
func truncated(d *repo.Document) error {
	if !d.Unclosed() {
		return nil
	}
	return fmt.Errorf(
		"%s ends inside an unterminated <!-- comment or an unclosed code fence, "+
			"so archdoc reads less of it than you wrote and cannot edit it safely; close the marker first", d.Path)
}

// page finds the glossary and its entries together, so that the lines being
// spliced and the positions they are spliced at can never come from different
// files. Taking a *repo.Document instead would let a caller hand in any page,
// which would splice one file using another's line numbers.
func page(r *repo.Repo) (*repo.Document, []repo.GlossaryEntry, error) {
	d := r.ByPage(repo.GlossaryPage)
	if d == nil {
		return nil, nil, fmt.Errorf("this repository has no spec/%s.md", repo.GlossaryPage)
	}
	entries, _ := r.Glossary()
	return d, entries, nil
}

// readBack re-parses a rewritten page and refuses it unless the terms on it are
// exactly those the edit intended, and unless the term named by `one`, if any,
// ends up with exactly one paragraph.
//
// Every writer here computes a line range from a parse and splices on it. When
// the page holds something the parser reads differently from the author, such as
// an unterminated <!-- comment, that range is wrong and the splice destroys
// content: removing one term deleted every entry below it, in silence, on a page
// that lint passed. SetField has had this guard since it was written and
// docs/DECISIONS.md calls it the last line of defence; the glossary writers never had
// it. Comparing the sets rather than the order means an already-misordered page,
// which L15 reports, is still editable.
func readBack(out []byte, want []string, one string) error {
	// The page about to be written must itself parse to its end. truncated()
	// asks this of the page that was read; nothing asked it of the page being
	// produced, so a definition carrying an unclosed fence or comment was
	// written and every later glossary command then refused the page, including
	// removing the entry that caused it.
	if repo.UnclosedIn(out) {
		return errors.New(
			"this edit would leave the page ending inside an unterminated <!-- comment or an unclosed code fence, " +
				"after which archdoc could not edit it at all, so nothing was written")
	}

	entries, ok := repo.GlossaryIn(out)
	if !ok {
		return errors.New("the rewritten page does not read back as a glossary, so nothing was written")
	}

	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.Term
	}
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
		return fmt.Errorf(
			"this edit would have left the page defining %s rather than %s, so nothing was written; "+
				"check the page for an unterminated <!-- comment or an unclosed code fence",
			terms(got), terms(want))
	}
	if one != "" {
		for _, e := range entries {
			if e.Term == one && len(e.Paragraphs) != 1 {
				return fmt.Errorf("%q would read back as %d paragraphs rather than one, so nothing was written",
					one, len(e.Paragraphs))
			}
		}
	}
	return nil
}

// terms renders a term list for a message.
func terms(list []string) string {
	if len(list) == 0 {
		return "nothing"
	}
	return strings.Join(list, ", ")
}

// termsOf is the terms an entry list names.
func termsOf(entries []repo.GlossaryEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Term
	}
	return out
}

// Add inserts a term in ascending case-insensitive order.
func Add(r *repo.Repo, term, definition string) ([]byte, error) {
	d, entries, err := page(r)
	if err != nil {
		return nil, err
	}
	if err := truncated(d); err != nil {
		return nil, err
	}
	if _, found := find(entries, term); found {
		return nil, fmt.Errorf("the glossary already defines %q", term)
	}
	if !repo.HeadingSurvives(term) {
		return nil, fmt.Errorf("%q is not usable as a term: markdown would read it back as something else", term)
	}
	if blankLinePattern.MatchString(definition) {
		return nil, fmt.Errorf("a definition is one paragraph; this one has a blank line in it")
	}
	out := insert(d.Source, entries, term, body(definition))
	if err := readBack(out, append(termsOf(entries), term), term); err != nil {
		return nil, err
	}
	return out, nil
}

// Remove deletes a term and its definition.
func Remove(r *repo.Repo, term string) ([]byte, error) {
	d, entries, err := page(r)
	if err != nil {
		return nil, err
	}
	if err := truncated(d); err != nil {
		return nil, err
	}
	i, found := find(entries, term)
	if !found {
		return nil, fmt.Errorf("the glossary does not define %q", term)
	}
	lines := split(d.Source)
	start, end := span(entries, i, len(lines))
	out := join(slices.Delete(lines, start, end), repo.LineEnding(d.Source))

	want := slices.Delete(termsOf(entries), i, i+1)
	if err := readBack(out, want, ""); err != nil {
		return nil, err
	}
	return out, nil
}

// Rename retitles a term, records the previous name, and re-sorts. It does not
// rewrite links elsewhere: the old anchor is gone, so L17 reports them, and
// they must then be corrected by hand. `archdoc link` cannot repair them. It
// expands [[...]] and deliberately never edits an existing markdown link, and
// [[<old name>]] does not resolve either, because Repo.Term keys on current
// headings and a recorded former name is not a term.
func Rename(r *repo.Repo, from, to string) ([]byte, error) {
	d, entries, err := page(r)
	if err != nil {
		return nil, err
	}
	if err := truncated(d); err != nil {
		return nil, err
	}
	i, found := find(entries, from)
	if !found {
		return nil, fmt.Errorf("the glossary does not define %q", from)
	}
	if j, clash := find(entries, to); clash && j != i {
		return nil, fmt.Errorf("the glossary already defines %q", to)
	}
	if !repo.HeadingSurvives(to) {
		return nil, fmt.Errorf("%q is not usable as a term: markdown would read it back as something else", to)
	}
	entry := entries[i]

	// The entry's own lines are carried over verbatim. Re-rendering it from the
	// parsed paragraphs would drop anything that is not one, such as an HTML
	// comment, and would reflow a fenced block's blank lines and indentation.
	lines := split(d.Source)
	start, end := span(entries, i, len(lines))
	carried := recordFormerly(slices.Clone(lines[start+1:end]), entry.Term)

	// Removing and re-inserting puts the entry back in the right place, which
	// a rename to a different letter always changes.
	without := join(slices.Delete(lines, start, end), repo.LineEnding(d.Source))

	// Read the rewritten page back, so the insert works from real entry
	// positions rather than positions adjusted by hand after the delete.
	remaining, ok := repo.GlossaryIn(without)
	if !ok {
		return nil, fmt.Errorf("rewriting %s produced something that is not a glossary", d.Path)
	}
	out := insert(without, remaining, to, carried)

	// No paragraph check: a rename carries the entry's own lines across
	// verbatim, and an entry that already held more than one paragraph is
	// L15's business, not a reason to refuse to rename it.
	want := termsOf(entries)
	want[i] = to
	if err := readBack(out, want, ""); err != nil {
		return nil, err
	}
	return out, nil
}

// recordFormerly adds a previous name to an entry's lines, grouped with any
// already there so that repeated renames accumulate rather than scatter.
func recordFormerly(lines []string, previous string) []string {
	note := fmt.Sprintf("Formerly *%s*.", previous)

	last := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			last = i
		}
	}
	if last < 0 {
		return append(lines, note)
	}
	if formerlyLine.MatchString(strings.TrimSpace(lines[last])) {
		return slices.Insert(lines, last+1, note)
	}
	return slices.Insert(lines, last+1, "", note)
}

// blankLinePattern separates paragraphs, matching CRLF as well as LF so that a
// definition saved on Windows does not read as one unbroken paragraph.
var blankLinePattern = regexp.MustCompile(`\r?\n[ \t]*\r?\n`)

// formerlyLine matches the note a rename leaves behind.
var formerlyLine = regexp.MustCompile(`^Formerly \*.+\*\.$`)

// Include appends identifiers to the page's includes list, in the order given,
// skipping any already present. It takes source separately so that it can be
// applied to a page another operation has already rewritten.
func Include(r *repo.Repo, source []byte, ids []string) ([]byte, error) {
	d, _, err := page(r)
	if err != nil {
		return nil, err
	}
	current := slices.Clone(d.FrontMatter.Includes)
	for _, id := range ids {
		// Refuse anything L04 or L06 would reject, rather than writing a page
		// the tool's own linter fails.
		target := r.ByID(id)
		switch {
		case target == nil:
			return nil, fmt.Errorf("%s does not exist", id)
		case !target.Type.Normative():
			return nil, fmt.Errorf("%s is a %s: only RFCs and ADRs may be included", id, target.Type)
		case target.FrontMatter.Status != repo.StatusAccepted:
			return nil, fmt.Errorf("%s is %s: a spec page reflects accepted documents only", id, target.FrontMatter.Status)
		}
		if !slices.Contains(current, id) {
			current = append(current, id)
		}
	}
	return repo.SetField(source, "includes", d.FrontMatter.LineOf("includes"),
		"["+strings.Join(current, ", ")+"]")
}

// find locates a term, case-insensitively as uniqueness is defined.
func find(entries []repo.GlossaryEntry, term string) (int, bool) {
	for i, e := range entries {
		if strings.EqualFold(e.Term, term) {
			return i, true
		}
	}
	return 0, false
}

// span is the line range an entry occupies, zero-based and half-open. It runs
// to the next entry's heading, so the blank line separating them travels with
// the entry above and insertion and deletion both stay balanced.
func span(entries []repo.GlossaryEntry, i, total int) (start, end int) {
	start = entries[i].Line - 1
	// End comes from the walk that found the entry, so this and that walk
	// cannot disagree about where the entry stops. They did: this ended an
	// entry at the next entry, which runs straight through an ordinary H1
	// section sitting between or after them, and removing a term then deleted
	// that section with it.
	if end = entries[i].End - 1; entries[i].End == 0 || end > total {
		return start, total
	}
	return start, end
}

// body renders the lines beneath a heading for a new entry.
//
// It used to take the previous names too, but Rename now carries an entry's own
// lines across verbatim rather than re-rendering them, so its only caller passed
// nil and half the function was unreachable for every possible input.
func body(definition string) []string {
	// Split, so that a soft-wrapped definition becomes separate elements and
	// each picks up the file's own line ending from join. Emitting it as one
	// element writes a bare newline into a CRLF file.
	lines := append([]string{""}, strings.Split(definition, "\n")...)
	return append(lines, "")
}

// insert splices an entry in before the first entry that sorts after it, or at
// the end when there is none. Anything before the first entry is preamble and
// is never touched.
func insert(source []byte, entries []repo.GlossaryEntry, term string, body []string) []byte {
	lines := split(source)
	lower := strings.ToLower(term)

	at := len(lines)
	for _, e := range entries {
		if strings.ToLower(e.Term) > lower {
			at = e.Line - 1
			break
		}
	}
	return join(slices.Insert(lines, at, append([]string{"## " + term}, body...)...), repo.LineEnding(source))
}

// split divides a page into lines with their endings stripped, so that an edit
// works on content and the ending is reapplied once by join.
func split(source []byte) []string {
	lines := strings.Split(string(source), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// join rebuilds a page with the file's own line ending, finishing it with
// exactly one however the edit left it.
func join(lines []string, ending string) []byte {
	joined := strings.TrimRight(strings.Join(lines, ending), "\r\n")
	return []byte(joined + ending)
}
