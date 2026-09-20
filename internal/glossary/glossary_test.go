package glossary_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ollieread/archdoc/internal/glossary"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/repotest"
)

const page = `---
title: Glossary
includes: [RFC-0001]   # the naming decision
---

# Glossary

<!-- One term per entry, alphabetical. -->

## Cache

A store of expensive results.

## Wings

The node daemon that runs containers.
`

// glossaryRepo writes a glossary into a throwaway repository, alongside the
// documents Include needs to resolve.
func glossaryRepo(t *testing.T, source string) *repo.Repo {
	t.Helper()
	return repotest.New(t, map[string]string{
		"spec/glossary.md":    source,
		"rfc/0001-naming.md":  decision("RFC-0001", "Naming", "accepted"),
		"adr/0002-storage.md": decision("ADR-0002", "Storage", "accepted"),
		"rfc/0003-pending.md": decision("RFC-0003", "Pending", "draft"),
	})
}

// decision renders a minimal RFC or ADR, enough for Include to resolve it.
func decision(id, title, status string) string {
	decided := ""
	if status == "accepted" {
		decided = " 2026-02-01"
	}
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: " + status +
		"\ncreated: 2026-01-01\ndecided:" + decided +
		"\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# " + id + ": " + title + "\n"
}

// terms reparses rewritten output and returns its terms, so the tests assert on
// what archdoc would read back rather than on the text it happened to emit.
func terms(t *testing.T, source []byte) []string {
	t.Helper()
	entries, ok := glossaryRepo(t, string(source)).Glossary()
	if !ok {
		t.Fatal("rewritten page is not a glossary")
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Term)
	}
	return out
}

func TestAddInsertsAlphabetically(t *testing.T) {
	out, err := glossary.Add(glossaryRepo(t, page), "Spec page", "A page describing how the project works now.")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if want := []string{"Cache", "Spec page", "Wings"}; !slices.Equal(terms(t, out), want) {
		t.Errorf("terms = %q, want %q", terms(t, out), want)
	}
	if !strings.Contains(string(out), "A page describing how the project works now.") {
		t.Errorf("the definition was not written:\n%s", out)
	}
	// The preamble and front matter, comments included, are untouched.
	if !strings.Contains(string(out), "includes: [RFC-0001]   # the naming decision") {
		t.Errorf("front matter was disturbed:\n%s", out)
	}
	if !strings.Contains(string(out), "<!-- One term per entry, alphabetical. -->") {
		t.Errorf("the preamble was disturbed:\n%s", out)
	}
}

func TestAddSortsBeforeEveryExistingTerm(t *testing.T) {
	out, err := glossary.Add(glossaryRepo(t, page), "Anchor", "A fragment identifier.")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if want := []string{"Anchor", "Cache", "Wings"}; !slices.Equal(terms(t, out), want) {
		t.Errorf("terms = %q, want %q", terms(t, out), want)
	}
}

func TestAddRefusesADuplicate(t *testing.T) {
	if _, err := glossary.Add(glossaryRepo(t, page), "cache", "Another definition."); err == nil {
		t.Error("Add accepted a term differing only in case")
	}
}

func TestAddCreatesTheFirstEntry(t *testing.T) {
	empty := "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n<!-- preamble -->\n"
	out, err := glossary.Add(glossaryRepo(t, empty), "Cache", "A store.")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got := terms(t, out); len(got) != 1 || got[0] != "Cache" {
		t.Errorf("terms = %q, want [Cache]", got)
	}
}

func TestRemoveDeletesOnlyThatEntry(t *testing.T) {
	out, err := glossary.Remove(glossaryRepo(t, page), "Cache")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if want := []string{"Wings"}; !slices.Equal(terms(t, out), want) {
		t.Errorf("terms = %q, want %q", terms(t, out), want)
	}
	if strings.Contains(string(out), "A store of expensive results.") {
		t.Errorf("the definition survived:\n%s", out)
	}
}

func TestRenameRetitlesResortsAndRecordsTheOldName(t *testing.T) {
	out, err := glossary.Rename(glossaryRepo(t, page), "Cache", "Zone cache")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Renaming to a name that sorts later must move the entry.
	if want := []string{"Wings", "Zone cache"}; !slices.Equal(terms(t, out), want) {
		t.Errorf("terms = %q, want %q", terms(t, out), want)
	}
	if !strings.Contains(string(out), "Formerly *Cache*.") {
		t.Errorf("the old name was not recorded:\n%s", out)
	}
	if !strings.Contains(string(out), "A store of expensive results.") {
		t.Errorf("the definition was lost:\n%s", out)
	}
}

func TestRenameAccumulatesFormerNames(t *testing.T) {
	once, err := glossary.Rename(glossaryRepo(t, page), "Cache", "Store")
	if err != nil {
		t.Fatalf("first rename: %v", err)
	}
	twice, err := glossary.Rename(glossaryRepo(t, string(once)), "Store", "Vault")
	if err != nil {
		t.Fatalf("second rename: %v", err)
	}

	for _, want := range []string{"Formerly *Cache*.", "Formerly *Store*."} {
		if !strings.Contains(string(twice), want) {
			t.Errorf("%q missing after two renames:\n%s", want, twice)
		}
	}
	entries, _ := glossaryRepo(t, string(twice)).Glossary()
	i := slices.IndexFunc(entries, func(e repo.GlossaryEntry) bool { return e.Term == "Vault" })
	if i < 0 {
		t.Fatalf("Vault is missing after two renames; got %v", entries)
	}
	if got := entries[i].Paragraphs; len(got) != 1 {
		t.Errorf("Vault has %d paragraphs, want 1: %q", len(got), got)
	}
}

func TestIncludeAppendsWithoutDuplicating(t *testing.T) {
	r := glossaryRepo(t, page)

	out, err := glossary.Include(r, r.ByPage(repo.GlossaryPage).Source, []string{"ADR-0002", "RFC-0001"})
	if err != nil {
		t.Fatalf("Include: %v", err)
	}
	// The trailing comment must survive: preserving it is why SetField exists.
	if want := "includes: [RFC-0001, ADR-0002]   # the naming decision"; !strings.Contains(string(out), want) {
		t.Errorf("want %q in the rewritten page:\n%s", want, out)
	}
	if strings.Count(string(out), "RFC-0001") != 1 {
		t.Errorf("RFC-0001 was added twice:\n%s", out)
	}
}

func TestIncludeRefusesWhatLintWouldReject(t *testing.T) {
	r := glossaryRepo(t, page)
	source := r.ByPage(repo.GlossaryPage).Source

	for _, tc := range []struct{ name, id, want string }{
		{"no such document", "RFC-9999", "does not exist"},
		{"not accepted", "RFC-0003", "draft"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := glossary.Include(r, source, []string{tc.id})
			if err == nil {
				t.Fatalf("Include accepted %s, which would fail lint", tc.id)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestOperationsRefuseARepositoryWithNoGlossary(t *testing.T) {
	if _, err := glossary.Add(repotest.New(t, nil), "Cache", "A store."); err == nil {
		t.Error("Add succeeded against a repository with no glossary page")
	}
}

func TestRenameKeepsEverythingInTheEntryItMoves(t *testing.T) {
	source := "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n" +
		"## Alpha\n\nThe definition.\n\n<!-- check this against the RFC -->\n\n" +
		"```go\n\tif x {\n\n\t\treturn y\n\t}\n```\n\n" +
		"## Beta\n\nAnother.\n"

	out, err := glossary.Rename(glossaryRepo(t, source), "Alpha", "Zeta")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}

	for _, want := range []string{
		"<!-- check this against the RFC -->",       // a comment is content, not noise
		"```go\n\tif x {\n\n\t\treturn y\n\t}\n```", // tabs and blank lines inside a fence
		"The definition.",
		"Formerly *Alpha*.",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("renaming lost %q:\n%s", want, out)
		}
	}
}

func TestEditsKeepCarriageReturnsInACRLFFile(t *testing.T) {
	crlf := strings.ReplaceAll("---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Beta\n\nSecond.\n", "\n", "\r\n")

	out, err := glossary.Add(glossaryRepo(t, crlf), "Alpha", "First.")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	for i, line := range strings.Split(string(out), "\n") {
		last := i == len(strings.Split(string(out), "\n"))-1
		if !last && !strings.HasSuffix(line, "\r") {
			t.Errorf("line %d lost its carriage return in a CRLF file: %q", i+1, line)
		}
	}
}

func TestAddKeepsLineEndingsInAMultiLineDefinition(t *testing.T) {
	crlf := strings.ReplaceAll("---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Zeta\n\nLast.\n", "\n", "\r\n")

	out, err := glossary.Add(glossaryRepo(t, crlf), "Alpha", "A definition\nsoft-wrapped over two lines.")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if strings.Contains(strings.ReplaceAll(string(out), "\r\n", ""), "\n") {
		t.Errorf("a bare newline was written into a CRLF file:\n%q", out)
	}
}

func TestAddRefusesADefinitionThatWouldBeTwoParagraphs(t *testing.T) {
	_, err := glossary.Add(glossaryRepo(t, page), "Alpha", "First paragraph.\n\nSecond paragraph.")
	if err == nil {
		t.Error("Add accepted a definition with a blank line, which L15 then reports")
	}
}

func TestAddRefusesATermThatWouldNotReadBack(t *testing.T) {
	// A trailing hash is heading syntax, so the entry could never be found
	// again by the name that was typed.
	if _, err := glossary.Add(glossaryRepo(t, page), "Alpha #", "A definition."); err == nil {
		t.Error("Add accepted a term that markdown will not give back")
	}
}

// pageWithTrailingSection is a glossary followed by an ordinary H1 section.
// The page is out of spec, but L15 passes it, so the destructive commands run
// on it with no warning.
const pageWithTrailingSection = `---
title: Glossary
includes: []
---

# Glossary

## Cache

A store of expensive results.

## Wings

The node daemon that runs containers.

# Maintenance

Reviewed each quarter by whoever is on call.
`

func TestRemoveKeepsWhatFollowsTheLastEntry(t *testing.T) {
	// An entry ended at the next H2 entry, or at end of file for the last one,
	// while the walk that found the entry ended it at the next heading of level
	// two or less. The two diverge on exactly one shape, an H1 inside the entry
	// region, and the divergence deleted the reader's own content.
	r := glossaryRepo(t, pageWithTrailingSection)

	out, err := glossary.Remove(r, "Wings")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"# Maintenance", "Reviewed each quarter"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("removing an entry deleted %q:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), "The node daemon") {
		t.Errorf("the entry itself survived:\n%s", out)
	}
}

func TestRenameKeepsWhatFollowsTheLastEntry(t *testing.T) {
	r := glossaryRepo(t, pageWithTrailingSection)

	out, err := glossary.Rename(r, "Wings", "Aardvark")
	if err != nil {
		t.Fatal(err)
	}

	body := string(out)
	if !strings.Contains(body, "# Maintenance\n\nReviewed each quarter by whoever is on call.") {
		t.Errorf("renaming an entry moved or broke the trailing section:\n%s", body)
	}
	if strings.Index(body, "## Aardvark") > strings.Index(body, "# Maintenance") {
		t.Errorf("the renamed entry was placed after the trailing section:\n%s", body)
	}
}

// poisoned is a glossary whose second entry holds an unterminated comment, so
// the parser reads two entries where the file has four.
const poisoned = `---
title: Glossary
includes: []
---

# Glossary

## Alpha

First.

## Bravo

Second. <!-- TODO: check this with the RFC

## Charlie

Third.

## Delta

Fourth.
`

func TestWritersRefuseAPageWhoseParseStopsShort(t *testing.T) {
	// The parse stops at the unterminated comment, so every line range computed
	// from it runs past the entries the parser could not see. Removing one term
	// used to delete every entry below it, exit 0, on a page lint passes.
	r := glossaryRepo(t, poisoned)

	for _, tc := range []struct {
		name string
		run  func() ([]byte, error)
	}{
		{"add", func() ([]byte, error) { return glossary.Add(r, "Zulu", "A letter.") }},
		{"remove", func() ([]byte, error) { return glossary.Remove(r, "Bravo") }},
		{"rename", func() ([]byte, error) { return glossary.Rename(r, "Bravo", "Zulu") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.run()
			if err == nil {
				t.Fatalf("the edit was allowed and produced:\n%s", out)
			}
			if !strings.Contains(err.Error(), "unterminated") {
				t.Errorf("error = %v, want it to name the unterminated marker", err)
			}
		})
	}
}

func TestAddRefusesADefinitionThatReadsBackAsAnExtraEntry(t *testing.T) {
	// The term was round-tripped through HeadingSurvives; the definition was
	// checked only for a blank line and then written verbatim, so a heading
	// inside it became a second, phantom entry that lint passed and link
	// resolved.
	r := glossaryRepo(t, page)

	out, err := glossary.Add(r, "Alpha", "A real definition.\n## Sneaky\nA plausible definition.")
	if err == nil {
		t.Fatalf("the definition was accepted and produced:\n%s", out)
	}
	if !strings.Contains(err.Error(), "Sneaky") {
		t.Errorf("error = %v, want it to name the entry that would have appeared", err)
	}
}

func TestAddRefusesADefinitionThatReadsBackAsNoParagraph(t *testing.T) {
	r := glossaryRepo(t, page)

	for _, definition := range []string{"Formerly *Gamma*.", "<!-- nothing -->"} {
		if out, err := glossary.Add(r, "Alpha", definition); err == nil {
			t.Errorf("%q was accepted and produced:\n%s", definition, out)
		}
	}
}

func TestAddRefusesADefinitionThatWouldTruncateThePage(t *testing.T) {
	// Add checked that the page it was about to edit parsed to its end, but not
	// that the page it was about to write did. An unclosed fence or comment in
	// the definition survived both guards, and from then on every glossary
	// command refused the page, including removing the entry that caused it.
	r := glossaryRepo(t, page)

	for _, definition := range []string{
		"```json shows the shape",
		"A real definition. <!-- note to self",
	} {
		out, err := glossary.Add(r, "Zulu", definition)
		if err == nil {
			t.Errorf("%q was accepted and produced:\n%s", definition, out)
		}
	}
}

func TestRemoveRefusesATermItCannotFind(t *testing.T) {
	// find() returns (0, false) when the term is absent, so without the refusal
	// the splice proceeds with i=0 and deletes the alphabetically first entry.
	// readBack cannot catch it, because want is computed from the same index.
	r := glossaryRepo(t, page)

	out, err := glossary.Remove(r, "Nonexistent")
	if err == nil {
		t.Fatalf("removing an absent term reported success and produced:\n%s", out)
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("error = %v, want it to name the term", err)
	}
}

func TestRenameRefusesATermItCannotFind(t *testing.T) {
	r := glossaryRepo(t, page)

	if out, err := glossary.Rename(r, "Nonexistent", "Something"); err == nil {
		t.Fatalf("renaming an absent term reported success and produced:\n%s", out)
	}
}

func TestRenameRefusesToCreateADuplicate(t *testing.T) {
	// readBack compares term sets, and after a clashing rename both sides hold
	// the name twice, so it passes. The explicit clash check is the only
	// refusal, and without it the page ends up with two entries under one name
	// and no command that can tell them apart.
	r := glossaryRepo(t, page)

	out, err := glossary.Rename(r, "Cache", "Wings")
	if err == nil {
		t.Fatalf("renaming onto an existing term reported success and produced:\n%s", out)
	}
	if !strings.Contains(err.Error(), "Wings") {
		t.Errorf("error = %v, want it to name the term already defined", err)
	}
}
