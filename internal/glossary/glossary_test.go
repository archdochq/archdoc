package glossary_test

import (
	"slices"
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
	r := repotest.New(t, map[string]string{
		"adr/0003-obsolete.md": "---\nid: ADR-0003\ntitle: Obsolete\nstatus: proposed\ncreated: 2026-01-20\n" +
			"depends: []\nupdates: []\nobsoletes: []\n---\n\n# ADR-0003: Obsolete\n",
	})

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

// awkwardNames are names YAML reads as something other than a plain string
// when written bare: a mapping, a comment, a tag, an alias, an anchor, a
// sequence, null, or two items of a flow sequence.
var awkwardNames = []string{"Foo: bar", "x # y", "#tag", "!bang", "*star", "& amp", "[x]", "null", "A, B"}

func readTerm(t *testing.T, docPath string, source []byte) *repo.Document {
	t.Helper()
	d, ok := repo.Parse(docPath, source)
	if !ok {
		t.Fatalf("%s is not a document path", docPath)
	}
	return d
}

func TestAddWritesAnyNameSoItReadsBack(t *testing.T) {
	for _, name := range awkwardNames {
		r := repotest.New(t, map[string]string{})
		docPath, source, err := glossary.Add(r, name, "A definition.", "")
		if err != nil {
			t.Errorf("Add(%q): %v", name, err)
			continue
		}
		if got := readTerm(t, docPath, source).FrontMatter.Title; got != name {
			t.Errorf("Add(%q) reads back as %q:\n%s", name, got, source)
		}
	}
}

func TestRenameWritesAnyNameSoItReadsBack(t *testing.T) {
	for _, name := range awkwardNames {
		r := repotest.New(t, map[string]string{
			repotest.TermPath("Plain"): repotest.Term("Plain", "A definition."),
		})
		_, docPath, source, err := glossary.Rename(r, "Plain", name)
		if err != nil {
			t.Errorf("Rename(Plain, %q): %v", name, err)
			continue
		}
		fm := readTerm(t, docPath, source).FrontMatter
		if fm.Title != name || !slices.Equal(fm.Formerly, []string{"Plain"}) {
			t.Errorf("Rename(Plain, %q) reads back as %q, formerly %q:\n%s", name, fm.Title, fm.Formerly, source)
		}
	}
}

// TestRenameKeepsAFormerNameWhole is the guarantee rename exists to give: a
// former name holding a comma was split in two, so GLOSSARY.md served neither
// its anchor nor any link written to it.
func TestRenameKeepsAFormerNameWhole(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"term/a-b.md": "---\ntitle: \"A, B\"\nformerly: []\nnamed_by:\n---\n\n# A, B\n\nA definition.\n",
	})
	_, docPath, source, err := glossary.Rename(r, "A, B", "Cee")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if got := readTerm(t, docPath, source).FrontMatter.Formerly; !slices.Equal(got, []string{"A, B"}) {
		t.Errorf("formerly = %q, want [\"A, B\"]:\n%s", got, source)
	}
}

// TestRenameKeepsAnyFormerNameAsWritten: "null" written bare in formerly read
// back as an empty name.
func TestRenameKeepsAnyFormerNameAsWritten(t *testing.T) {
	for _, name := range awkwardNames {
		r := repotest.New(t, map[string]string{
			repotest.TermPath("Plain"): repotest.Term("Plain", "A definition."),
		})
		_, docPath, source, err := glossary.Rename(r, "Plain", name)
		if err != nil {
			t.Errorf("Rename(Plain, %q): %v", name, err)
			continue
		}
		r = repotest.New(t, map[string]string{docPath: string(source)})
		_, docPath, source, err = glossary.Rename(r, name, "Cee")
		if err != nil {
			t.Errorf("Rename(%q, Cee): %v", name, err)
			continue
		}
		if got := readTerm(t, docPath, source).FrontMatter.Formerly; !slices.Equal(got, []string{"Plain", name}) {
			t.Errorf("formerly = %q, want [Plain %q]:\n%s", got, name, source)
		}
	}
}

func TestRenameRefusesTheCurrentName(t *testing.T) {
	r := repotest.New(t, map[string]string{
		repotest.TermPath("Widget"): repotest.Term("Widget", "A definition."),
	})
	if _, _, _, err := glossary.Rename(r, "Widget", "Widget"); err == nil {
		t.Error("renamed Widget to itself")
	}
}

// TestRenameRecordsNoNameTheHeadingAlreadyAnswers: a former name with the
// heading's anchor gave GLOSSARY.md that anchor twice.
func TestRenameRecordsNoNameTheHeadingAlreadyAnswers(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		formerly       []string
		want           []string
	}{
		{"a change of case", "widget", "Widget", nil, nil},
		{"back to a former name", "New", "Old", []string{"Old"}, []string{"New"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			formerly := "[" + strings.Join(tc.formerly, ", ") + "]"
			r := repotest.New(t, map[string]string{
				repotest.TermPath(tc.from): "---\ntitle: " + tc.from + "\nformerly: " + formerly +
					"\nnamed_by:\n---\n\n# " + tc.from + "\n\nA definition.\n",
			})
			_, docPath, source, err := glossary.Rename(r, tc.from, tc.to)
			if err != nil {
				t.Fatalf("Rename: %v", err)
			}
			if got := readTerm(t, docPath, source).FrontMatter.Formerly; !slices.Equal(got, tc.want) {
				t.Errorf("formerly = %q, want %q:\n%s", got, tc.want, source)
			}
		})
	}
}

func TestRenameKeepsACRLFHeadingCRLF(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"term/gamma.md": "---\r\ntitle: Gamma\r\nformerly: []\r\nnamed_by:\r\n---\r\n\r\n# Gamma\r\n\r\nA definition.\r\n",
	})
	_, _, source, err := glossary.Rename(r, "Gamma", "Delta")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if !strings.Contains(string(source), "# Delta\r\n") {
		t.Errorf("the heading lost its carriage return:\n%q", source)
	}
}

func TestAddRefusesADefinitionOfMoreThanOneParagraph(t *testing.T) {
	r := repotest.New(t, map[string]string{})
	if _, _, err := glossary.Add(r, "Widget", "One.\n\nTwo.", ""); err == nil {
		t.Error("accepted a definition of two paragraphs")
	}
}

func TestAddAcceptsOnlyAnExistingDocumentAsNamedBy(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"adr/0001-chosen.md": "---\nid: ADR-0001\ntitle: Chosen\nstatus: proposed\ncreated: 2026-01-20\n" +
			"depends: []\nupdates: []\nobsoletes: []\n---\n\n# ADR-0001: Chosen\n",
	})
	docPath, source, err := glossary.Add(r, "Widget", "A definition.", "ADR-0001")
	if err != nil {
		t.Fatalf("Add with ADR-0001: %v", err)
	}
	if got := readTerm(t, docPath, source).FrontMatter.NamedBy; got != "ADR-0001" {
		t.Errorf("named_by = %q, want ADR-0001", got)
	}
	for _, namedBy := range []string{"ADR-0002", "Foo: bar"} {
		if _, _, err := glossary.Add(r, "Gadget", "A definition.", namedBy); err == nil {
			t.Errorf("Add accepted named_by %q", namedBy)
		}
	}
}

// TestRemoveRefusesEveryLinkAFrozenDocumentHolds covers the two links a
// frozen document can hold besides the current anchor: one made before a
// rename, and one to the term's own file.
func TestRemoveRefusesEveryLinkAFrozenDocumentHolds(t *testing.T) {
	for name, link := range map[string]string{
		"a former name": "../GLOSSARY.md#old-name",
		"the term file": "../term/widget.md",
	} {
		t.Run(name, func(t *testing.T) {
			r := repotest.New(t, map[string]string{
				"term/widget.md": "---\ntitle: Widget\nformerly: [Old name]\nnamed_by:\n---\n\n# Widget\n\nA definition.\n",
				"spec/uses.md":   "---\ntitle: Uses\nincludes: []\n---\n\n# Uses\n\nIt holds a [widget](" + link + ").\n",
			})
			if _, err := glossary.Remove(r, "Widget", everythingFrozen); err == nil {
				t.Errorf("removed a term a frozen document links to by %s", name)
			}
		})
	}
}

// TestAddRefusesANameTheHeadingReadsDifferently: a trailing " #" is an ATX
// closing sequence, so the H1 would read as the name without it.
func TestAddAndRenameRefuseANameTheHeadingReadsDifferently(t *testing.T) {
	r := repotest.New(t, map[string]string{
		repotest.TermPath("Plain"): repotest.Term("Plain", "A definition."),
	})
	if _, _, err := glossary.Add(r, "Sharp #", "A definition.", ""); err == nil {
		t.Error("Add accepted a name whose heading reads as another")
	}
	if _, _, _, err := glossary.Rename(r, "Plain", "Sharp #"); err == nil {
		t.Error("Rename accepted a name whose heading reads as another")
	}
}
