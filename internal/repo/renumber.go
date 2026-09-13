package repo

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// identifierPattern is the shape Identifier produces, read back.
var identifierPattern = regexp.MustCompile(`^([A-Z]+)-(\d{4})$`)

// ParseIdentifier reads an identifier back into the type and number it names.
func ParseIdentifier(id string) (Type, int, bool) {
	m := identifierPattern.FindStringSubmatch(id)
	if m == nil {
		return "", 0, false
	}
	t, ok := ParseType(strings.ToLower(m[1]))
	if !ok || !t.Numbered() {
		return "", 0, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n < 1 {
		return "", 0, false
	}
	return t, n, true
}

// referenceKeys are the front matter lists that name other documents. A
// renumbered document has to be renamed in every one of them.
var referenceKeys = []string{"depends", "updates", "obsoletes", "includes"}

// Renumber changes a numbered document's identifier, rewriting the filename,
// the front matter id, the heading, and every reference to the old identifier
// in every other document.
//
// The number lives in three places inside its own document and in an unbounded
// number of places outside it, so doing this by hand reliably trades one lint
// error for another. Two concurrent pull requests taking the same number is the
// case that makes it routine.
//
// frozen reports whether a document may no longer be written to. Renumbering
// refuses when the document itself is frozen, and when any document that would
// have to change is, because a reference inside a frozen document could never
// be corrected and the rename would leave it pointing at a file that is gone.
// The refusal is complete: nothing is written unless everything can be.
func Renumber(r *Repo, from, to string, frozen func(*Document) bool) ([]string, error) {
	d, err := selectDocument(r, from)
	if err != nil {
		return nil, err
	}
	if !d.Type.Numbered() {
		return nil, fmt.Errorf("%s is not numbered, so it has no number to change", d.Path)
	}
	if frozen(d) {
		return nil, fmt.Errorf("%s is frozen; write a new document that updates it rather than renaming it", d.ID)
	}

	number, err := targetNumber(r, d, to)
	if err != nil {
		return nil, err
	}
	to = Identifier(d.Type, number)
	if to == d.ID {
		return nil, fmt.Errorf("%s already has that identifier", d.ID)
	}
	if taken := r.ByID(to); taken != nil {
		return nil, fmt.Errorf("%s is already used by %s", to, taken.Path)
	}

	newPath, err := renamedPath(d, number)
	if err != nil {
		return nil, err
	}

	// Everything that has to change is found before anything is written, so a
	// frozen reference stops the whole operation rather than half of it.
	from = d.ID
	mentions := regexp.MustCompile(`\b` + regexp.QuoteMeta(from) + `\b`)
	var blocked []string
	edits := map[string][]byte{}
	for _, other := range r.Documents() {
		if other == d {
			continue
		}
		rewritten, touched := rewriteReferences(other, from, mentions, to, d.Path, newPath)
		if !touched {
			continue
		}
		if frozen(other) {
			blocked = append(blocked, other.ID+" ("+other.Path+")")
			continue
		}
		edits[other.Path] = rewritten
	}
	if len(blocked) > 0 {
		sort.Strings(blocked)
		return nil, fmt.Errorf("%s is referenced by frozen %s, which cannot be edited to follow the rename",
			from, strings.Join(blocked, ", "))
	}

	source, err := renameInSelf(d, from, to)
	if err != nil {
		return nil, err
	}

	changed := make([]string, 0, len(edits)+1)
	for docPath, body := range edits {
		if err := r.WriteFile(docPath, body); err != nil {
			return nil, err
		}
		changed = append(changed, docPath)
	}
	if err := r.WriteFile(newPath, source); err != nil {
		return nil, err
	}
	if err := os.Remove(r.File(d.Path)); err != nil {
		return nil, err
	}
	changed = append(changed, d.Path+" -> "+newPath)
	sort.Strings(changed)
	return changed, nil
}

// selectDocument finds the document to rename, by path or by identifier.
//
// An identifier is not always unique: two pull requests branching from the same
// commit both take the next free number, and after both merge two documents
// carry it. That is the case this command exists for, and ByID answers it with
// whichever path sorts first, so acting on the identifier alone would rename
// the other contributor's document. A path is unambiguous, so it is offered as
// the way through.
func selectDocument(r *Repo, from string) (*Document, error) {
	if strings.Contains(from, "/") || strings.HasSuffix(from, ".md") {
		if d := r.ByPath(path.Clean(from)); d != nil {
			return d, nil
		}
		return nil, fmt.Errorf("no document at %s", from)
	}
	var matches []*Document
	for _, d := range r.Documents() {
		if d.ID == from {
			matches = append(matches, d)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no document has the identifier %s", from)
	case 1:
		return matches[0], nil
	}
	paths := make([]string, 0, len(matches))
	for _, d := range matches {
		paths = append(paths, d.Path)
	}
	sort.Strings(paths)
	return nil, fmt.Errorf("%d documents claim %s (%s); name the one to renumber by path",
		len(matches), from, strings.Join(paths, ", "))
}

// targetNumber resolves the requested identifier, or picks the next free number
// when none was given, which is what a contributor resolving a collision wants.
func targetNumber(r *Repo, d *Document, to string) (int, error) {
	if to == "" {
		highest := 0
		for _, o := range r.Documents() {
			if o.Type == d.Type {
				highest = max(highest, o.Number)
			}
		}
		if highest+1 > maxNumber {
			return 0, fmt.Errorf("%s numbering stops at %04d, and %04d is taken", d.Type, maxNumber, highest)
		}
		return highest + 1, nil
	}
	t, n, ok := ParseIdentifier(to)
	if !ok {
		return 0, fmt.Errorf("%q is not an identifier of the form RFC-0001", to)
	}
	if t != d.Type {
		return 0, fmt.Errorf("%s is a %s and cannot become a %s; the type decides the directory and the required sections",
			d.ID, d.Type, t)
	}
	return n, nil
}

// renamedPath keeps the slug and changes only the number, because the slug is
// for humans and the number is the identity.
func renamedPath(d *Document, number int) (string, error) {
	m := filenamePattern.FindStringSubmatch(path.Base(d.Path))
	if m == nil {
		return "", fmt.Errorf("%s is not named <NNNN>-<slug>.md, so there is no number to change", d.Path)
	}
	return path.Join(path.Dir(d.Path), fmt.Sprintf("%04d-%s.md", number, m[2])), nil
}

// renameInSelf rewrites the front matter id and the heading. Both, or the
// document trades an L03 error for an L10 one.
func renameInSelf(d *Document, from, to string) ([]byte, error) {
	source, err := SetField(d.Source, "id", d.FrontMatter.LineOf("id"), to)
	if err != nil {
		return nil, fmt.Errorf("rewriting the id of %s: %w", d.Path, err)
	}
	h1, ok := d.H1()
	if !ok {
		return source, nil
	}
	lines := strings.Split(string(source), "\n")
	if h1.Line < 1 || h1.Line > len(lines) {
		return source, nil
	}
	i := h1.Line - 1
	// Only where the heading actually opens with the old identifier: a heading
	// that does not match the front matter is L10's to report, not this
	// command's to silently repair.
	if strings.Contains(lines[i], from) {
		lines[i] = strings.Replace(lines[i], from, to, 1)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// rewriteReferences renames the identifier inside one other document: in the
// front matter lists that name documents, and in the body wherever the old
// identifier or the old path appears.
func rewriteReferences(d *Document, from string, mentions *regexp.Regexp, to, oldPath, newPath string) ([]byte, bool) {
	lines := strings.Split(string(d.Source), "\n")
	touched := false

	for _, key := range referenceKeys {
		start := d.FrontMatter.LineOf(key)
		if start == 0 {
			continue
		}
		for i := start - 1; i < len(lines) && i < d.BodyLine-1; i++ {
			if i > start-1 && isNewKey(lines[i]) {
				break
			}
			if replaced := mentions.ReplaceAllString(lines[i], to); replaced != lines[i] {
				lines[i] = replaced
				touched = true
			}
		}
	}

	// The body: wiki links naming the identifier, and links whose destination
	// is the file being renamed. A bare mention in prose is left alone, because
	// rewriting a sentence is not this command's business.
	for i := d.BodyLine - 1; i < len(lines); i++ {
		if i < 0 {
			continue
		}
		before := lines[i]
		lines[i] = strings.ReplaceAll(lines[i], "[["+from+"]]", "[["+to+"]]")
		lines[i] = replacePath(lines[i], oldPath, newPath)
		if lines[i] != before {
			touched = true
		}
	}
	if !touched {
		return nil, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// isNewKey reports whether a front matter line starts a different key, which is
// where the value above it ends.
func isNewKey(line string) bool {
	trimmed := strings.TrimRight(line, "\r")
	if trimmed == "" || strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") ||
		strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "#") {
		return false
	}
	return strings.Contains(trimmed, ":")
}

// replacePath rewrites a link destination pointing at the renamed file. Paths
// are relative, so only the final segments are compared.
func replacePath(line, oldPath, newPath string) string {
	oldBase, newBase := path.Base(oldPath), path.Base(newPath)
	if !strings.Contains(line, oldBase) {
		return line
	}
	return strings.ReplaceAll(line, oldBase, newBase)
}
