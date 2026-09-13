package repo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/repotest"
)

func draft(id, title string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: draft\n" +
		"created: 2026-01-01\ndecided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n## Abstract\n\nWords.\n"
}

func accepted(id, title string, depends string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: accepted\n" +
		"created: 2026-01-01\ndecided: 2026-01-02\ndepends: [" + depends + "]\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n## Abstract\n\nWords.\n"
}

// nothingFrozen is the predicate for a repository with no history, where every
// document is still editable.
func accepted2(id, title, depends string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: draft\n" +
		"created: 2026-01-01\ndecided:\ndepends: [" + depends + "]\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n## Abstract\n\nWords.\n"
}

func nothingFrozen(*repo.Document) bool { return false }

// frozenIDs builds a predicate freezing exactly the named documents, standing in
// for what lint learns from the branch.
func frozenIDs(ids ...string) func(*repo.Document) bool {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return func(d *repo.Document) bool { return set[d.ID] }
}

// TestRenumberRefusesWhenAFrozenDocumentReferencesIt is the guard that matters
// most. A frozen document cannot be edited, so its reference to the old
// identifier could never be corrected, and renaming anyway would leave it
// pointing at a file that no longer exists with no permitted repair.
func TestRenumberRefusesWhenAFrozenDocumentReferencesIt(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md":  draft("RFC-0001", "Base"),
		"rfc/0002-later.md": accepted("RFC-0002", "Later", "RFC-0001"),
	})
	before, err := os.ReadFile(r.File("rfc/0002-later.md"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = repo.Renumber(r, "RFC-0001", "RFC-0009", frozenIDs("RFC-0002"))
	if err == nil {
		t.Fatal("renumbered a document that a frozen document references")
	}
	if !strings.Contains(err.Error(), "RFC-0002") {
		t.Errorf("the error does not name the frozen document that blocks it: %v", err)
	}

	// Nothing may have been written: a refusal is all or nothing.
	if _, err := os.Stat(r.File("rfc/0001-base.md")); err != nil {
		t.Errorf("the original file was removed despite the refusal: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0009-base.md")); err == nil {
		t.Error("a renamed file was left behind despite the refusal")
	}
	after, err := os.ReadFile(r.File("rfc/0002-later.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the frozen document was modified despite the refusal")
	}
}

// TestRenumberRefusesAFrozenDocument covers the other half: the document being
// renumbered is itself frozen, so the rename is the edit L11 forbids.
func TestRenumberRefusesAFrozenDocument(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md": accepted("RFC-0001", "Base", ""),
	})
	if _, err := repo.Renumber(r, "RFC-0001", "RFC-0009", frozenIDs("RFC-0001")); err == nil {
		t.Fatal("renumbered a frozen document")
	}
	if _, err := os.Stat(r.File("rfc/0001-base.md")); err != nil {
		t.Errorf("the original file was removed: %v", err)
	}
}

// TestRenumberRewritesEveryPlaceTheIdentityAppears pins the three places a
// number lives in its own document. Doing two of the three trades an L03 error
// for an L10 one, which is the mistake the command exists to prevent.
func TestRenumberRewritesEveryPlaceTheIdentityAppears(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0002-retry-policy.md": draft("RFC-0002", "Retry policy"),
	})
	changed, err := repo.Renumber(r, "RFC-0002", "RFC-0009", nothingFrozen)
	if err != nil {
		t.Fatalf("Renumber: %v", err)
	}

	if _, err := os.Stat(r.File("rfc/0002-retry-policy.md")); err == nil {
		t.Error("the old file is still there")
	}
	moved := r.File("rfc/0009-retry-policy.md")
	body, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("the renamed file is missing: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "id: RFC-0009") {
		t.Errorf("the front matter id was not rewritten:\n%s", text)
	}
	if !strings.Contains(text, "# RFC-0009: Retry policy") {
		t.Errorf("the heading was not rewritten:\n%s", text)
	}
	if strings.Contains(text, "RFC-0002") {
		t.Errorf("the old identifier survives somewhere:\n%s", text)
	}
	if len(changed) == 0 {
		t.Error("Renumber reported changing nothing")
	}
}

// TestRenumberTakesTheNextFreeNumber covers the form the contribution flow
// uses, where the contributor does not care which number they end up with.
func TestRenumberTakesTheNextFreeNumber(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-first.md": draft("RFC-0001", "First"),
		"rfc/0002-mine.md":  draft("RFC-0002", "Mine"),
		"rfc/0007-other.md": draft("RFC-0007", "Other"),
	})
	if _, err := repo.Renumber(r, "RFC-0002", "", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0008-mine.md")); err != nil {
		t.Errorf("expected the next free number, 0008: %v", err)
	}
}

// TestRenumberUpdatesReferencesInEditableDocuments stops the rename from
// breaking L04 everywhere the old identifier was named.
func TestRenumberUpdatesReferencesInEditableDocuments(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-base.md":  draft("RFC-0001", "Base"),
		"rfc/0002-later.md": accepted2("RFC-0002", "Later", "RFC-0001"),
		"spec/thing.md":     "---\ntitle: Thing\nincludes: [RFC-0001]\n---\n\n# Thing\n\nWords.\n",
	})
	if _, err := repo.Renumber(r, "RFC-0001", "RFC-0009", nothingFrozen); err != nil {
		t.Fatalf("Renumber: %v", err)
	}
	for _, check := range []struct{ path, want string }{
		{"rfc/0002-later.md", "depends: [RFC-0009]"},
		{"spec/thing.md", "includes: [RFC-0009]"},
	} {
		body, err := os.ReadFile(filepath.Join(r.Config().RootDir(), filepath.FromSlash(check.path)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), check.want) {
			t.Errorf("%s was not updated, wanted %q:\n%s", check.path, check.want, body)
		}
	}
}

// TestRenumberRefusesANumberAlreadyTaken keeps the command from creating the
// very collision it exists to resolve.
func TestRenumberRefusesANumberAlreadyTaken(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-first.md": draft("RFC-0001", "First"),
		"rfc/0002-mine.md":  draft("RFC-0002", "Mine"),
	})
	if _, err := repo.Renumber(r, "RFC-0002", "RFC-0001", nothingFrozen); err == nil {
		t.Fatal("renumbered onto a number that is already taken")
	}
}

// TestRenumberRefusesToChangeType keeps an RFC from becoming an ADR, which
// would move it to a directory with different required sections.
func TestRenumberRefusesToChangeType(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-first.md": draft("RFC-0001", "First"),
	})
	if _, err := repo.Renumber(r, "RFC-0001", "ADR-0001", nothingFrozen); err == nil {
		t.Fatal("renumbered an RFC into an ADR")
	}
}

// TestRenumberRefusesAnAmbiguousIdentifier covers the case the command exists
// for. When two pull requests both claimed a number, two documents carry the
// identifier, and ByID answers with whichever path sorts first. Acting on that
// would rename the other contributor's document.
func TestRenumberRefusesAnAmbiguousIdentifier(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0002-alice.md": draft("RFC-0002", "Alice"),
		"rfc/0002-bob.md":   draft("RFC-0002", "Bob"),
	})
	_, err := repo.Renumber(r, "RFC-0002", "", nothingFrozen)
	if err == nil {
		t.Fatal("renumbered an ambiguous identifier instead of refusing")
	}
	// Both have to be named, or the user cannot tell which one to pass.
	for _, want := range []string{"rfc/0002-alice.md", "rfc/0002-bob.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
	for _, path := range []string{"rfc/0002-alice.md", "rfc/0002-bob.md"} {
		if _, err := os.Stat(r.File(path)); err != nil {
			t.Errorf("%s was touched despite the refusal: %v", path, err)
		}
	}
}

// TestRenumberAcceptsAPathToDisambiguate is the way out of the above: name the
// file, which is unambiguous even when the identifier is not.
func TestRenumberAcceptsAPathToDisambiguate(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0002-alice.md": draft("RFC-0002", "Alice"),
		"rfc/0002-bob.md":   draft("RFC-0002", "Bob"),
	})
	if _, err := repo.Renumber(r, "rfc/0002-bob.md", "", nothingFrozen); err != nil {
		t.Fatalf("Renumber by path: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0003-bob.md")); err != nil {
		t.Errorf("the named document was not renumbered: %v", err)
	}
	if _, err := os.Stat(r.File("rfc/0002-alice.md")); err != nil {
		t.Errorf("the other contributor's document was disturbed: %v", err)
	}
}
