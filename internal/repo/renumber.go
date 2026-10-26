package repo

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
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

// referenceKeys are the front matter keys that name other documents. A
// renumbered document has to be renamed in every one of them.
var referenceKeys = []string{"depends", "updates", "obsoletes", "includes", "documents", "named_by"}

// Renumber changes a numbered document's identifier, rewriting the filename,
// the front matter id, the heading, and every reference to the old identifier
// or the old file, in the document itself and in every other.
//
// The number lives in three places inside its own document and in an unbounded
// number of places outside it, so doing this by hand reliably trades one lint
// error for another. Two concurrent pull requests taking the same number is the
// case that makes it routine.
//
// When another document still claims the old identifier, as when two branches
// each took the same next number, an identifier in a front matter list or a
// wiki link cannot say which document it means. Those are left alone and each
// line holding one is returned as unchanged, as path:line: text and marked
// where the document is frozen, for the author to repoint any that meant the
// moved document. A link to the file says which it means, so it is still
// rewritten.
//
// frozen reports whether a document may no longer be written to. Renumbering
// refuses when the document itself is frozen, and when any document that would
// have to change is, because a reference inside a frozen document could never
// be corrected and the rename would leave it pointing at a file that is gone.
// The refusal is complete: nothing is written unless everything can be.
func Renumber(r *Repo, from, to string, frozen func(*Document) bool) (changed, unchanged []string, err error) {
	d, err := selectDocument(r, from)
	if err != nil {
		return nil, nil, err
	}
	if !d.Type.Numbered() {
		return nil, nil, fmt.Errorf("%s is not numbered, so it has no number to change", d.Path)
	}
	if frozen(d) {
		return nil, nil, fmt.Errorf("%s is frozen; write a new document that updates it rather than renaming it", d.ID)
	}

	number, err := targetNumber(r, d, to)
	if err != nil {
		return nil, nil, err
	}
	to = Identifier(d.Type, number)
	if to == d.ID {
		return nil, nil, fmt.Errorf("%s already has that identifier", d.ID)
	}
	if taken := r.ByID(to); taken != nil {
		return nil, nil, fmt.Errorf("%s is already used by %s", to, taken.Path)
	}

	newPath, err := renamedPath(d, number)
	if err != nil {
		return nil, nil, err
	}

	// Everything that has to change is found before anything is written, so a
	// frozen reference stops the whole operation rather than half of it.
	from = d.ID
	rn := rename{
		from:     from,
		to:       to,
		mentions: regexp.MustCompile(`\b` + regexp.QuoteMeta(from) + `\b`),
		oldPath:  d.Path,
		newPath:  newPath,
	}
	for _, other := range r.Documents() {
		if other != d && other.ID == from {
			rn.shared = true
		}
	}
	var blocked []string
	edits := map[string][]byte{}
	own := d.Source
	for _, other := range r.Documents() {
		rewritten, touched, left, err := rewriteReferences(other, rn)
		if err != nil {
			return nil, nil, err
		}
		if other == d {
			// Its references to itself move with it. It is not frozen, or
			// the rename would already have been refused.
			if touched {
				own = rewritten
			}
			unchanged = append(unchanged, left...)
			continue
		}
		if frozen(other) {
			for i := range left {
				left[i] += " (frozen)"
			}
		}
		unchanged = append(unchanged, left...)
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
		// By path when the identifier is shared, since it names both.
		named := from
		if rn.shared {
			named = d.Path
		}
		return nil, nil, fmt.Errorf("%s is referenced by frozen %s, which cannot be edited to follow the rename",
			named, strings.Join(blocked, ", "))
	}

	source, err := renameInSelf(d, own, from, to)
	if err != nil {
		return nil, nil, err
	}

	if err := write(r, d, newPath, source, edits); err != nil {
		return nil, nil, err
	}
	changed = make([]string, 0, len(edits)+1)
	for docPath := range edits {
		changed = append(changed, docPath)
	}
	changed = append(changed, d.Path+" -> "+newPath)
	sort.Strings(changed)
	return changed, unchanged, nil
}

// write makes the rename: the new file, every other document's edits, then
// removing the old file. A failure part way restores what was already written,
// since a referring document rewritten to a number no file carries is the
// state the command exists to prevent. The new file goes first because an
// existing path in its place, a directory say, is the likeliest failure, and
// its write then changes nothing.
func write(r *Repo, d *Document, newPath string, source []byte, edits map[string][]byte) (err error) {
	if _, err := os.Lstat(r.File(newPath)); err == nil {
		return fmt.Errorf("%s already exists", newPath)
	}
	var written []string
	defer func() {
		if err == nil {
			return
		}
		for _, docPath := range written {
			_ = r.WriteFile(docPath, r.ByPath(docPath).Source)
		}
		_ = os.Remove(r.File(newPath))
	}()

	if err := r.WriteFile(newPath, source); err != nil {
		return err
	}
	paths := slices.Sorted(maps.Keys(edits))
	for _, docPath := range paths {
		if err := r.WriteFile(docPath, edits[docPath]); err != nil {
			return err
		}
		written = append(written, docPath)
	}
	return os.Remove(r.File(d.Path))
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

// renameInSelf rewrites the front matter id and the heading of the document's
// source, given with its references to itself already followed. Both, or the
// document trades an L03 error for an L10 one.
func renameInSelf(d *Document, source []byte, from, to string) ([]byte, error) {
	source, err := SetField(source, "id", d.FrontMatter.LineOf("id"), to)
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

// lineEdit replaces columns start to end of one line, counted from 0.
type lineEdit struct {
	line, start, end int
	text             string
}

// rename is one renumbering, as each other document's references see it.
type rename struct {
	from, to         string
	mentions         *regexp.Regexp
	oldPath, newPath string
	// shared is set when another document still claims from.
	shared bool
}

// rewriteReferences follows the rename inside one document, the moved one
// included: in the front matter lists that name documents, in wiki links naming
// the identifier, and in links that resolve to the old path. Under a shared
// identifier, the lines holding it in a front matter list or a wiki link are
// returned as left rather than rewritten, in line order and as they read once
// the rest is rewritten. A link it cannot rewrite as written is an error.
func rewriteReferences(d *Document, rn rename) (rewritten []byte, touched bool, left []string, err error) {
	lines := strings.Split(string(d.Source), "\n")
	var leftAt []int

	for _, key := range referenceKeys {
		start := d.FrontMatter.LineOf(key)
		if start == 0 {
			continue
		}
		for i := start - 1; i < len(lines) && i < d.BodyLine-1; i++ {
			if i > start-1 && isNewKey(lines[i]) {
				break
			}
			if !rn.mentions.MatchString(lines[i]) {
				continue
			}
			if rn.shared {
				leftAt = append(leftAt, i)
				continue
			}
			lines[i] = rn.mentions.ReplaceAllString(lines[i], rn.to)
			touched = true
		}
	}

	// The body: wiki links naming the identifier, and links that resolve to
	// the file being renamed, both read from the masked view so that code and
	// comments are left alone. A bare mention in prose is left alone too,
	// because rewriting a sentence is not this command's business.
	var edits []lineEdit
	wiki := "[[" + rn.from + "]]"
	for line := range d.Prose() {
		for at := 0; ; {
			i := strings.Index(line.Masked[at:], wiki)
			if i < 0 {
				break
			}
			at += i
			if rn.shared {
				leftAt = append(leftAt, line.Number-1)
			} else {
				edits = append(edits, lineEdit{line.Number - 1, at, at + len(wiki), "[[" + rn.to + "]]"})
			}
			at += len(wiki)
		}
	}
	for _, l := range d.Links() {
		target, _, _ := strings.Cut(l.Target, "#")
		// Skipped as L17 skips them: a URL is not a path, though its own path
		// can climb back to the file.
		if strings.Contains(target, "://") || strings.HasPrefix(l.Target, "mailto:") ||
			path.Join(path.Dir(d.Path), target) != rn.oldPath {
			continue
		}
		// The way the link was written is kept, and only the file name in it
		// changes: the renamed file stays in the same directory. That needs
		// the file name where the text as written puts it, which an encoded
		// / or # moves; replacing it anyway would write another destination.
		written, fragment, hasFragment := strings.Cut(lines[l.Line-1][l.Start:l.End], "#")
		name := written[strings.LastIndex(written, "/")+1:]
		// A name that does not decode cannot have resolved to the file.
		if decoded, _ := url.PathUnescape(name); decoded != path.Base(rn.oldPath) {
			return nil, false, nil, fmt.Errorf("%s:%d: the link to %s is encoded so that renumber cannot rewrite it; write it as a plain relative path first",
				d.Path, l.Line, rn.oldPath)
		}
		replacement := written[:len(written)-len(name)] + path.Base(rn.newPath)
		if hasFragment {
			replacement += "#" + fragment
		}
		edits = append(edits, lineEdit{l.Line - 1, l.Start, l.End, replacement})
	}
	// From the end of each line backwards, so an edit leaves the columns of
	// those before it where they were. Edits to different lines are
	// independent, so their order among each other does not matter.
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		lines[e.line] = lines[e.line][:e.start] + e.text + lines[e.line][e.end:]
		touched = true
	}

	// Formatted last, so each line reads as written, and sorted, because the
	// front matter keys are visited in a fixed order rather than the file's.
	// The moved document is reported where the author will find it.
	shown := d.Path
	if shown == rn.oldPath {
		shown = rn.newPath
	}
	sort.Ints(leftAt)
	for _, i := range slices.Compact(leftAt) {
		left = append(left, fmt.Sprintf("%s:%d: %s", shown, i+1, strings.TrimSpace(lines[i])))
	}
	if !touched {
		return nil, false, left, nil
	}
	return []byte(strings.Join(lines, "\n")), true, left, nil
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
