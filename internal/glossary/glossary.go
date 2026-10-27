// Package glossary writes the term files under term/ and generates GLOSSARY.md
// from them. Nothing here touches disk: every function returns bytes for a
// caller in cmd to write, so the rule that nothing under internal/ performs IO
// on the user's behalf holds here as it does everywhere else.
package glossary

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strings"

	"archdoc.dev/internal/repo"
	"gopkg.in/yaml.v3"
)

// File is the generated glossary, written under root beside INDEX.md.
const File = "GLOSSARY.md"

// PathOf is the file a term lives in, relative to root and slash-separated.
// The slug is the identity, which is why two terms that slug alike cannot both
// exist: they would be one file on a case-insensitive filesystem and one
// anchor in the generated page either way.
func PathOf(term string) string {
	return path.Join(string(repo.TypeTerm), repo.Slug(term)+".md")
}

// Add builds the file for a new term. It refuses a term whose slug is already
// taken, which covers a name differing only in case: Anchor lowercases, so
// both would compete for one anchor in GLOSSARY.md and which kept it would
// depend on document order.
func Add(r *repo.Repo, term, definition, namedBy string) (docPath string, source []byte, err error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return "", nil, fmt.Errorf("a term needs a name")
	}
	slug := repo.Slug(term)
	if slug == "" {
		return "", nil, fmt.Errorf("%q has no ASCII letters or digits to make a filename from", term)
	}
	entries, _ := r.Glossary()
	for _, e := range entries {
		if repo.Slug(e.Term) == slug {
			return "", nil, fmt.Errorf("%q is already defined in %s; two terms cannot share a slug", e.Term, e.Path)
		}
	}
	if namedBy != "" && r.ByID(namedBy) == nil {
		return "", nil, fmt.Errorf("named_by %q is not a document in this repository", namedBy)
	}
	docPath, source = PathOf(term), render(term, definition, nil, namedBy)
	entry, err := readBack(docPath, source, term)
	if err != nil {
		return "", nil, err
	}
	if len(entry.Paragraphs) != 1 {
		return "", nil, fmt.Errorf("the definition reads as %d paragraphs, and a term has exactly one", len(entry.Paragraphs))
	}
	return docPath, source, nil
}

// Rename retitles a term and records the previous name, which generation turns
// into an anchor so links written before the rename keep resolving. That is
// what makes a rename safe for a frozen document, which could never be edited
// to follow one.
func Rename(r *repo.Repo, from, to string) (oldPath, newPath string, source []byte, err error) {
	to = strings.TrimSpace(to)
	if to == "" {
		return "", "", nil, fmt.Errorf("a term needs a name")
	}
	entry, ok := r.Term(from)
	if !ok {
		return "", "", nil, fmt.Errorf("%q is not a term", from)
	}
	if to == entry.Term {
		return "", "", nil, fmt.Errorf("%q is already the term's name", to)
	}
	if repo.Slug(to) == "" {
		return "", "", nil, fmt.Errorf("%q has no ASCII letters or digits to make a filename from", to)
	}
	if repo.Slug(to) != repo.Slug(entry.Term) {
		entries, _ := r.Glossary()
		for _, e := range entries {
			if repo.Slug(e.Term) == repo.Slug(to) {
				return "", "", nil, fmt.Errorf("%q is already defined in %s", e.Term, e.Path)
			}
		}
	}

	d := r.ByPath(entry.Path)
	if d == nil {
		return "", "", nil, fmt.Errorf("%s is missing", entry.Path)
	}
	out, err := repo.SetField(d.Source, "title", d.FrontMatter.LineOf("title"), scalar(to))
	if err != nil {
		return "", "", nil, err
	}
	// The previous name is appended rather than prepended: formerly is oldest
	// first, and the name being replaced is the newest of the old ones. A name
	// with the new heading's anchor is left out, since the heading answers it:
	// a change of case, or a return to a former name, gave GLOSSARY.md one
	// anchor twice.
	formerly := slices.DeleteFunc(append(slices.Clone(d.FrontMatter.Formerly), entry.Term), func(name string) bool {
		return repo.Anchor(name) == repo.Anchor(to)
	})
	out, err = repo.SetField(out, "formerly", d.FrontMatter.LineOf("formerly"), list(formerly))
	if err != nil {
		return "", "", nil, err
	}
	// The H1 carries the term too, and L10 holds it to the title.
	out = retitleHeading(out, entry.Term, to)
	newPath = PathOf(to)
	if _, err := readBack(newPath, out, to); err != nil {
		return "", "", nil, err
	}
	return entry.Path, newPath, out, nil
}

// readBack parses a term file as written and refuses it unless its H1 reads as
// the term. The front matter is quoted through the YAML encoder, which a
// heading has no equivalent of: a name ending in " #" reads as an ATX closing
// sequence, and one holding a line break as a heading and a paragraph.
func readBack(docPath string, source []byte, term string) (repo.GlossaryEntry, error) {
	d, _ := repo.Parse(docPath, source)
	entry := d.TermEntry()
	heading := ""
	for _, h := range d.Headings() {
		if h.Level == 1 {
			heading = h.Text
			break
		}
	}
	if heading != term {
		return entry, fmt.Errorf("%q would not read back as written; choose another name", term)
	}
	return entry, nil
}

// Remove reports the file to delete. frozen answers whether a document may no
// longer be edited; a term a frozen document links to cannot be removed,
// because the link would be left pointing at an anchor that no longer exists
// and the document holding it could never be corrected. renumber refuses on
// the same grounds. A link counts by any anchor the term is served under,
// former names included, or to the term's own file.
func Remove(r *repo.Repo, term string, frozen func(*repo.Document) bool) (docPath string, err error) {
	entry, ok := r.Term(term)
	if !ok {
		return "", fmt.Errorf("%q is not a term", term)
	}
	anchors := []string{entry.Anchor}
	for _, former := range entry.Formerly {
		anchors = append(anchors, repo.Anchor(former))
	}
	var blocking []string
	for _, d := range r.Documents() {
		if d.Type == repo.TypeTerm || !frozen(d) {
			continue
		}
		for _, l := range d.Links() {
			target, anchor, _ := strings.Cut(l.Target, "#")
			if path.Base(target) == File && slices.Contains(anchors, anchor) ||
				path.Join(path.Dir(d.Path), target) == entry.Path {
				blocking = append(blocking, d.Path)
				break
			}
		}
	}
	if len(blocking) > 0 {
		return "", fmt.Errorf("%q is linked from %s, which is frozen and could not be corrected",
			entry.Term, strings.Join(blocking, ", "))
	}
	return entry.Path, nil
}

// Generate renders GLOSSARY.md from the term files. Every former name gets an
// explicit anchor before its heading, so a link written before a rename still
// resolves; L17 checks exactly that, and AnchorsIn reads explicit anchors for
// this reason.
func Generate(r *repo.Repo) []byte {
	var b bytes.Buffer
	b.WriteString("# Glossary\n\n")
	b.WriteString("<!-- Generated by archdoc glossary. Edit the files under term/ instead. -->\n")

	entries, _ := r.Glossary()
	for _, e := range entries {
		b.WriteString("\n")
		for _, former := range e.Formerly {
			fmt.Fprintf(&b, "<a id=%q></a>\n", repo.Anchor(former))
		}
		fmt.Fprintf(&b, "## %s\n\n", e.Term)
		for i, p := range e.Paragraphs {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(strings.TrimRight(p, "\n"))
			b.WriteString("\n")
		}
	}
	return b.Bytes()
}

// render builds a term file.
func render(term, definition string, formerly []string, namedBy string) []byte {
	var b bytes.Buffer
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", scalar(term))
	fmt.Fprintf(&b, "formerly: %s\n", list(formerly))
	fmt.Fprintf(&b, "named_by: %s\n", namedBy)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", term)
	b.WriteString(strings.TrimRight(definition, "\n"))
	b.WriteString("\n")
	return b.Bytes()
}

// scalar is a string as YAML writes it, quoted only where it would otherwise
// read as something else: "Foo: bar" as a mapping, "x # y" as x and a comment,
// "#tag" and "null" as nothing.
func scalar(s string) string {
	return marshal(&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s})
}

// list renders a flow sequence, the spelling every other list in front matter
// uses. The encoder quotes an item holding a comma, which would otherwise be
// read as two.
func list(items []string) string {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, item := range items {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item})
	}
	return marshal(seq)
}

// marshal encodes one node. A string node cannot fail to encode, so an error
// here is a fault in this package.
func marshal(n *yaml.Node) string {
	out, err := yaml.Marshal(n)
	if err != nil {
		panic(err)
	}
	return strings.TrimRight(string(out), "\n")
}

// retitleHeading replaces the H1 that carries the term. Only the first level
// one heading is touched: the definition below it is the author's.
func retitleHeading(source []byte, from, to string) []byte {
	lines := bytes.Split(source, []byte("\n"))
	want := []byte("# " + from)
	for i, line := range lines {
		if text, cr := bytes.CutSuffix(line, []byte("\r")); bytes.Equal(text, want) {
			lines[i] = []byte("# " + to)
			if cr {
				lines[i] = append(lines[i], '\r')
			}
			break
		}
	}
	return bytes.Join(lines, []byte("\n"))
}
