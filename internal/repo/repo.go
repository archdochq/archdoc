// Package repo discovers, parses and relates the documents of a spec
// repository. Nothing here reads stdin, prompts, or exits.
package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"archdoc.dev/internal/config"
)

// Type is one of the four kinds of document, and also the directory each kind
// lives in.
type Type string

const (
	TypeRFC  Type = "rfc"
	TypeADR  Type = "adr"
	TypeRef  Type = "ref"
	TypeSpec Type = "spec"
)

// Types is every document type, in directory order.
var Types = []Type{TypeADR, TypeRef, TypeRFC, TypeSpec}

// ParseType turns user input into a Type. Type is a string type with no
// constructor, so Type("nonsense") is otherwise a perfectly valid value and
// every caller would need its own list of the real ones.
func ParseType(s string) (Type, bool) {
	t := Type(s)
	return t, slices.Contains(Types, t)
}

// Numbered reports whether documents of this type carry an identifier derived
// from a number in the filename. Spec pages are named by path instead.
func (t Type) Numbered() bool { return t != TypeSpec }

// HasLifecycle reports whether documents of this type move through statuses. A
// spec page reflects what has already been decided and a ref records something
// outside the project, so neither has a status at all.
func (t Type) HasLifecycle() bool { return t == TypeRFC || t == TypeADR }

// Normative reports whether a document of this type may be named in a depends
// or includes list. Those name designs and decisions; a ref is informational
// and is cited from the body instead.
//
// It answers the same as HasLifecycle today and is a separate question: one is
// about whether a document can change state, the other about whether anything
// may rest on it. A fifth type would very likely separate them.
func (t Type) Normative() bool { return t == TypeRFC || t == TypeADR }

// filenamePattern is the required shape of a numbered type's filename: a
// four-digit number, a hyphen, and a slug.
var filenamePattern = regexp.MustCompile(`^(\d{4})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)

// maxNumber is the largest identifier filenamePattern's four digits can hold.
const maxNumber = 9999

// Identifier renders the identifier for a type and number, as RFC-0001.
func Identifier(t Type, number int) string {
	return fmt.Sprintf("%s-%04d", strings.ToUpper(string(t)), number)
}

// Repo is a spec repository: its configuration and every document under root.
type Repo struct {
	config    *config.Config
	documents []*Document
	byID      map[string]*Document
	byPage    map[string]*Document
	byPath    map[string]*Document
	terms     map[string]GlossaryEntry
}

// Document is one file in one of the four document directories.
type Document struct {
	Type Type
	// Path is relative to root, always slash-separated.
	Path string
	// ID is the identifier derived from the type and the filename number, such
	// as RFC-0001. Empty for spec pages, which are not numbered. This is the
	// authoritative identity; lint compares the front matter id against it.
	ID string
	// Number is the filename number for numbered types, 0 otherwise.
	Number int
	// Page is a spec page's path below spec/ without the extension, such as
	// http/routing. Empty for numbered types.
	Page string

	// FrontMatter is the parsed YAML block.
	FrontMatter FrontMatter
	// Body is everything after the closing delimiter.
	Body string
	// BodyLine is the 1-based line the body starts on, so that positions
	// found in the body can be reported against the file.
	BodyLine int
	// Source is the document exactly as it is on disk, never normalised on the
	// way in, because every writer splices into it by offset.
	Source []byte
	// Problems are the faults found while reading the document. Lint decides
	// what they mean; repo only records them.
	Problems []Problem

	// UpdatedBy, ObsoletedBy and DependedOnBy hold the identifiers of the
	// documents naming this one in the matching forward list. IncludedIn holds
	// the names of the spec pages listing this document in includes. All are
	// sorted and derived, never written to the document itself.
	UpdatedBy    []string
	ObsoletedBy  []string
	DependedOnBy []string
	IncludedIn   []string

	// EffectivelyObsolete is true when an accepted document obsoletes this one.
	// A claim by a draft, proposed, rejected or withdrawn document has no
	// effect.
	EffectivelyObsolete bool
	// Implemented is true for an accepted RFC or ADR that some spec page
	// includes.
	Implemented bool
	// Stale is true for a spec page including an effectively obsolete document.
	Stale bool

	headings []Heading
	// masked is bodyLines with comments blanked, walked once per document.
	masked []string
	// unclosed records that the body ends inside a comment or a fence.
	unclosed bool
	repo     *Repo

	// lines and glossary are derived from an immutable document, so they are
	// computed once on first use. bodyLines was re-splitting the whole body on
	// every section lookup, which made reading a glossary quadratic in its
	// entries.
	lines    []string
	glossary []GlossaryEntry
}

// Repo is the repository this document was discovered in.
func (d *Document) Repo() *Repo { return d.repo }

// Open discovers and parses every document under the configured root.
func Open(c *config.Config) (*Repo, error) {
	r := &Repo{config: c}
	for _, t := range Types {
		found, err := discover(c.RootDir(), t)
		if err != nil {
			return nil, err
		}
		r.documents = append(r.documents, found...)
	}
	for _, d := range r.documents {
		d.repo = r
		if err := d.read(c.RootDir()); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(r.documents, func(a, b *Document) int {
		return strings.Compare(a.Path, b.Path)
	})
	r.derive()
	return r, nil
}

// Config is the configuration this repository was opened with.
func (r *Repo) Config() *config.Config { return r.config }

// File is the absolute location of a document path, which is stored
// slash-separated and relative to root.
func (r *Repo) File(docPath string) string {
	return filepath.Join(r.config.RootDir(), filepath.FromSlash(docPath))
}

// WriteFile puts a rewritten document back where it came from. Writing through
// the repository keeps the path translation and the permission bits in one
// place rather than at every call site that rewrites a document.
func (r *Repo) WriteFile(docPath string, source []byte) error {
	full := r.File(docPath)
	if err := Contains(r.Config().RootDir(), full); err != nil {
		return err
	}
	return WriteFile(full, source, docPath)
}

// Contains reports whether full sits inside root once every directory component
// has been resolved.
//
// config.ValidateRoot already checks the root itself, and that is not enough: a
// root can be perfectly ordinary while a document directory inside it points
// elsewhere, and every write into that directory then lands outside the
// repository while the tool reports a path inside it. A parent that does not
// exist yet is fine, because it will be created inside root.
func Contains(root, full string) error {
	dir, err := filepath.EvalSymlinks(filepath.Dir(full))
	if err != nil {
		return nil
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s resolves outside the repository, so nothing was written", filepath.Dir(full))
	}
	return nil
}

// WriteFile writes a file, refusing to write through a symbolic link. name is
// what the refusal calls the path, which for a document is its repository path.
//
// Every writer in the program must come through here. os.WriteFile follows a
// symlink and truncates whatever is on the other end, so a symlinked INDEX.md
// committed to a repository, which git stores and clone restores, redirected
// `archdoc index` onto any file the user could write, reported only "INDEX.md",
// and exited 0.
func WriteFile(full string, source []byte, name string) error {
	// os.WriteFile follows a symlink and truncates whatever is on the other
	// end, so a symlinked document wrote outside the repository while the tool
	// reported the path inside it. Refused rather than resolved: a document
	// that is a link to somewhere else is not a shape this tool supports, and
	// silently writing through it is the worst of the readings.
	if info, err := os.Lstat(full); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symbolic link, so it is not written through", name)
	}
	return os.WriteFile(full, source, 0o644)
}

// Documents returns every document, ordered by path.
func (r *Repo) Documents() []*Document { return r.documents }

// TypeOf classifies a path relative to root. ok is false when the path is not a
// document: the wrong extension, outside the four directories, or nested in a
// type that does not permit subdirectories.
func TypeOf(p string) (Type, bool) {
	p = filepath.ToSlash(p)
	if path.Ext(p) != ".md" {
		return "", false
	}
	dir, rest, found := strings.Cut(p, "/")
	if !found || rest == "" {
		return "", false
	}
	t := Type(dir)
	if !slices.Contains(Types, t) {
		return "", false
	}
	if t != TypeSpec && strings.Contains(rest, "/") {
		return "", false
	}
	return t, true
}

// read loads a document from disk and parses it. A malformed document is not
// an error: the faults are recorded as problems so that lint can report every
// one of them rather than stopping at the first.
func (d *Document) read(root string) error {
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d.Path)))
	if err != nil {
		return err
	}
	d.Source = source
	block, body, bodyLine, ok := splitFrontMatter(source)
	d.Body, d.BodyLine = string(body), bodyLine
	d.headings = parseHeadings(d.Body, d.BodyLine)
	if !ok {
		d.Problems = append(d.Problems, Problem{Line: 1, Message: "no front matter block"})
		return nil
	}
	fm, problems := parseFrontMatter(d.Type, block)
	d.FrontMatter, d.Problems = fm, append(d.Problems, problems...)
	return nil
}

// discover lists the documents of one type. Subdirectories are ignored except
// under spec/, where pages may be nested. A directory that does not exist holds
// no documents and is not an error.
func discover(root string, t Type) ([]*Document, error) {
	dir := filepath.Join(root, string(t))
	var found []*Document

	// os.DirFS yields relative, slash-separated paths, which is exactly what
	// Document.Path is defined to be, so no conversion is needed here. The OS
	// separator is only wanted when a file is actually opened, in read.
	err := fs.WalkDir(os.DirFS(dir), ".", func(rel string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			// Only spec/ may nest; every other type ignores subdirectories.
			if rel != "." && t != TypeSpec {
				return fs.SkipDir
			}
			return nil
		}
		docPath := path.Join(string(t), rel)
		if _, isDoc := TypeOf(docPath); !isDoc {
			return nil
		}
		d := &Document{Type: t, Path: docPath}
		if t.Numbered() {
			// A filename that does not match leaves ID empty; L02 reports it.
			if m := filenamePattern.FindStringSubmatch(e.Name()); m != nil {
				d.Number, _ = strconv.Atoi(m[1])
				d.ID = Identifier(t, d.Number)
			}
		} else {
			d.Page = strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		}
		found = append(found, d)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return found, nil
}

// slugSeparators are the characters a slug collapses to a single hyphen:
// anything that is not an unaccented ASCII letter or digit. Restricting to
// ASCII is what keeps a generated filename inside the shape L02 enforces.
var slugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// Slug derives the human-readable half of a filename from a title: lowercased,
// runs of anything else collapsed to one hyphen, and the ends trimmed. The
// number is the identity; this is only for reading.
func Slug(title string) string {
	return strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(title), "-"), "-")
}

// Next returns the identifier and path a new document of this type would take.
// The number is one past the highest in the directory, so a gap left by a
// document that was never written is not reused.
func (r *Repo) Next(t Type, title string) (id, path string, err error) {
	if !t.Numbered() {
		return "", "", fmt.Errorf("a %s is named by its path, not numbered", t)
	}
	slug := Slug(title)
	if slug == "" {
		return "", "", fmt.Errorf("%q contains nothing that can be used in a filename", title)
	}

	highest := 0
	for _, d := range r.documents {
		if d.Type == t {
			highest = max(highest, d.Number)
		}
	}
	number := highest + 1
	if number > maxNumber {
		// filenamePattern requires exactly four digits, so a fifth would make
		// every later command unable to read the document this is about to
		// write.
		return "", "", fmt.Errorf("%s numbering stops at %04d, and %04d is taken", t, maxNumber, highest)
	}
	return Identifier(t, number), fmt.Sprintf("%s/%04d-%s.md", t, number, slug), nil
}
