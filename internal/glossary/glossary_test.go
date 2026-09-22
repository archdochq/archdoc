package glossary_test

import (
	"strings"
	"testing"

	"archdoc.dev/internal/glossary"
	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/repotest"
)

func nothingFrozen(*repo.Document) bool { return false }

func everythingFrozen(*repo.Document) bool { return true }

// TestAddWritesATermFile pins the shape every later command reads back: the
// front matter keys L01 requires, and the H1 L10 holds to the title.
func TestAddWritesATermFile(t *testing.T) {
	r := repotest.New(t, map[string]string{})

	path, source, err := glossary.Add(r, "Effectively obsolete", "An accepted document obsoletes it.", "ADR-0003")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if want := "term/effectively-obsolete.md"; path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	for _, want := range []string{
		"title: Effectively obsolete", "formerly: []", "named_by: ADR-0003",
		"# Effectively obsolete", "An accepted document obsoletes it.",
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("the term file does not carry %q:\n%s", want, source)
		}
	}
}

// TestAddRefusesASlugAlreadyTaken is the guard the case question turned on.
// Anchor lowercases, so two terms whose slugs match compete for one anchor in
// the generated page and which one keeps it depends on document order. On a
// case-insensitive filesystem they are also one file.
func TestAddRefusesASlugAlreadyTaken(t *testing.T) {
	r := repotest.New(t, map[string]string{
		repotest.TermPath("Dave"): repotest.Term("Dave", "A term."),
	})

	for _, name := range []string{"dave", "DAVE", "Dave"} {
		if _, _, err := glossary.Add(r, name, "Another.", ""); err == nil {
			t.Errorf("Add(%q) was accepted beside an existing %q", name, "Dave")
		}
	}
}

// TestRenameRecordsThePreviousName is what makes a rename safe. Without the
// record, generation writes no anchor for the old name and every link made
// before the rename breaks, including one inside a frozen document that could
// never be edited to follow it.
func TestRenameRecordsThePreviousName(t *testing.T) {
	r := repotest.New(t, map[string]string{
		repotest.TermPath("editable"): repotest.Term("editable", "Not frozen."),
	})

	was, now, source, err := glossary.Rename(r, "editable", "open")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if was != "term/editable.md" || now != "term/open.md" {
		t.Errorf("paths = %q -> %q, want term/editable.md -> term/open.md", was, now)
	}
	for _, want := range []string{"title: open", "formerly: [editable]", "# open"} {
		if !strings.Contains(string(source), want) {
			t.Errorf("the renamed file does not carry %q:\n%s", want, source)
		}
	}
}

// TestRemoveRefusesATermAFrozenDocumentLinksTo is the same rule renumber
// enforces: the link could not be repaired afterwards, so the removal that
// would break it is refused instead.
func TestRemoveRefusesATermAFrozenDocumentLinksTo(t *testing.T) {
	r := repotest.New(t, map[string]string{
		repotest.TermPath("Widget"): repotest.Term("Widget", "A small thing."),
		"spec/uses.md": "---\ntitle: Uses\nincludes: []\n---\n\n# Uses\n\n" +
			"It holds a [Widget](../GLOSSARY.md#widget).\n",
	})

	if _, err := glossary.Remove(r, "Widget", everythingFrozen); err == nil {
		t.Fatal("removed a term a frozen document links to")
	} else if !strings.Contains(err.Error(), "spec/uses.md") {
		t.Errorf("the error does not name the document that blocks it: %v", err)
	}

	// The same repository with nothing frozen: the link can still be corrected,
	// so the removal is allowed.
	path, err := glossary.Remove(r, "Widget", nothingFrozen)
	if err != nil {
		t.Fatalf("Remove with nothing frozen: %v", err)
	}
	if want := "term/widget.md"; path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

// TestGenerateWritesAnAnchorForEveryFormerName is the guarantee the whole
// design rests on. L17 checks that a link with an anchor resolves to one, and
// AnchorsIn reads explicit anchors so that these count.
func TestGenerateWritesAnAnchorForEveryFormerName(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"term/open.md": "---\ntitle: open\nformerly: [editable, unfrozen]\nnamed_by:\n---\n\n" +
			"# open\n\nNot frozen.\n",
		repotest.TermPath("Backfilled"): repotest.Term("Backfilled", "Written after the fact."),
	})

	got := string(glossary.Generate(r))
	for _, want := range []string{`<a id="editable"></a>`, `<a id="unfrozen"></a>`, "## open", "## Backfilled"} {
		if !strings.Contains(got, want) {
			t.Errorf("the generated glossary lacks %q:\n%s", want, got)
		}
	}
	// Ascending case-insensitively, which is why Backfilled precedes open.
	if strings.Index(got, "## Backfilled") > strings.Index(got, "## open") {
		t.Errorf("terms are out of order:\n%s", got)
	}
	// The anchors have to be findable as anchors, not merely present as text.
	anchors := repo.AnchorsIn([]byte(got))
	for _, want := range []string{"open", "editable", "unfrozen", "backfilled"} {
		if !contains(anchors, want) {
			t.Errorf("AnchorsIn does not report %q: %v", want, anchors)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
