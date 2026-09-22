// Package repotest builds spec repositories for tests. Four test packages need
// a throwaway repository with a handful of documents in it, and cmd/archdoc
// will need a fifth, so the construction lives here rather than being copied.
//
// It imports testing, as httptest and iotest do, and lives under internal/ so
// it never reaches a released binary.
package repotest

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/repo"
)

// DefaultConfig is the archdoc.json used when a test does not care about the
// settings.
const DefaultConfig = `{"name":"T"}`

// New writes files into a temporary repository and opens it. Keys are paths
// relative to root, slash-separated; directories are created as needed.
func New(t *testing.T, files map[string]string) *repo.Repo {
	t.Helper()
	return NewWith(t, DefaultConfig, files)
}

// NewWith is New with a particular archdoc.json, for the tests where the
// setting under test lives in the configuration.
func NewWith(t *testing.T, configuration string, files map[string]string) *repo.Repo {
	t.Helper()
	dir := t.TempDir()
	Write(t, dir, "archdoc.json", configuration)
	for path, contents := range files {
		Write(t, dir, path, contents)
	}
	return open(t, filepath.Join(dir, "archdoc.json"))
}

// Fixture opens one of the checked-in repositories under testdata. The path is
// relative to the calling package's directory, which for every package under
// internal/ and cmd/ is two levels below the module root.
func Fixture(t *testing.T, name string) *repo.Repo {
	t.Helper()
	return open(t, filepath.Join("..", "..", "testdata", name, "archdoc.json"))
}

// Write puts one file into a repository, creating its directory.
func Write(t *testing.T, dir, path, contents string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Document finds a document by its path, failing the test if it is absent.
// Returning the document rather than a bool keeps the caller's assertions about
// the document itself.
func Document(t *testing.T, r *repo.Repo, path string) *repo.Document {
	t.Helper()
	for _, d := range r.Documents() {
		if d.Path == path {
			return d
		}
	}
	t.Fatalf("no document at %q", path)
	return nil
}

func open(t *testing.T, configPath string) *repo.Repo {
	t.Helper()
	c, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading %s: %v", configPath, err)
	}
	r, err := repo.Open(c)
	if err != nil {
		t.Fatalf("opening the repository at %s: %v", configPath, err)
	}
	return r
}

// Section is the first section of a document with the given title.
//
// repo.Document deliberately has no such method any more. A rule deciding
// whether a section holds content must read every section with that title, and
// an exported "just the first one" invited exactly that mistake four times
// before it was removed. Tests that want one section of one document are a
// different case, so the convenience lives here instead.
func Section(d *repo.Document, title string) (repo.Section, bool) {
	found := d.Sections(title)
	if len(found) == 0 {
		return repo.Section{}, false
	}
	return found[0], true
}

// Term renders a term file, the shape `archdoc term add` writes. Definitions
// are given as separate paragraphs so a fixture can exercise L15's one
// paragraph rule.
func Term(title string, paragraphs ...string) string {
	return "---\ntitle: " + title + "\nformerly: []\nnamed_by:\n---\n\n# " + title + "\n\n" +
		strings.Join(paragraphs, "\n\n") + "\n"
}

// TermPath is where Term's output belongs, relative to root.
func TermPath(title string) string {
	return path.Join(string(repo.TypeTerm), repo.Slug(title)+".md")
}
