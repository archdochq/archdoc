package index_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"archdoc.dev/internal/index"
	"archdoc.dev/internal/repotest"
)

// update rewrites the golden file. Run `go test ./internal/index -update` after
// a deliberate change to the format, then read the diff before committing it.
var update = flag.Bool("update", false, "rewrite the golden index")

const goldenPath = "../../testdata/golden/INDEX.md"

func TestGenerateMatchesTheGoldenIndex(t *testing.T) {
	got := index.Generate(repotest.Fixture(t, "repo"))

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading the golden index: %v (run with -update to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("generated index differs from the golden file.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerateIsStable(t *testing.T) {
	// The repository is reopened each time, because the flap risk is map
	// iteration inside derive, not anything Generate does to one snapshot.
	open := func() string { return string(index.Generate(repotest.Fixture(t, "repo"))) }

	first := open()
	for range 10 {
		if got := open(); got != first {
			t.Fatal("the index varies between runs, so index --check would flap")
		}
	}
}

func TestGenerateAlwaysEmitsAllFourSections(t *testing.T) {
	got := string(index.Generate(repotest.NewWith(t, `{"name":"Empty"}`, nil)))
	for _, section := range []string{"# Empty index", "## RFCs", "## ADRs", "## Spec", "## Refs"} {
		if !strings.Contains(got, section) {
			t.Errorf("a repository with no documents omitted %q:\n%s", section, got)
		}
	}
}

func TestGenerateProtectsTheTableFromTitleContent(t *testing.T) {
	// A pipe would end the cell; a newline would end the row.
	body := "---\nid: RFC-0001\ntitle: \"A | B\\nand more\"\nstatus: draft\ncreated: 2026-01-01\ndecided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# X\n"
	r := repotest.NewWith(t, `{"name":"Cells"}`, map[string]string{"rfc/0001-x.md": body})

	for _, line := range strings.Split(string(index.Generate(r)), "\n") {
		// An escaped pipe is content, not a cell boundary.
		if delimiters := strings.Count(strings.ReplaceAll(line, `\|`, ""), "|"); strings.HasPrefix(line, "| [RFC-0001]") && delimiters != 10 {
			t.Errorf("the row has %d cell boundaries, want 10: %q", delimiters, line)
		}
		if strings.Contains(line, "and more") && !strings.HasPrefix(line, "| [RFC-0001]") {
			t.Errorf("a newline in a title split the row: %q", line)
		}
	}
}

func TestStatusRendersBothQualifiers(t *testing.T) {
	// spec/spec/index.md specifies accepted (backfilled, obsolete); nothing in the
	// fixture produces it, so the comma-joining branch is otherwise untested.
	got := index.StatusOf("accepted", true, true)
	if want := "accepted (backfilled, obsolete)"; got != want {
		t.Errorf("StatusOf = %q, want %q", got, want)
	}
	if got := index.StatusOf("accepted", false, false); got != "accepted" {
		t.Errorf("StatusOf = %q, want %q", got, "accepted")
	}
}

func TestGenerateNeverTurnsATitleIntoALiveLink(t *testing.T) {
	// A title is free text. Without escaping, a document called
	// "Pwned](https://evil.example) x" renders as a working hyperlink in the
	// index, and lint reports nothing because L17 skips external targets.
	body := "---\nid: RFC-0001\ntitle: \"Pwned](https://evil.example) x\"\nstatus: draft\ncreated: 2026-01-01\ndecided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# X\n"
	r := repotest.NewWith(t, `{"name":"Escape"}`, map[string]string{"rfc/0001-x.md": body})

	got := string(index.Generate(r))
	// An escaped bracket still reads as "](" in the raw text, so the assertion
	// is that no bracket arrives unescaped.
	if regexp.MustCompile(`[^\\]\]\(https`).MatchString(got) {
		t.Errorf("a title became a live link:\n%s", got)
	}
	if !strings.Contains(got, `\]`) {
		t.Errorf("the bracket was not escaped:\n%s", got)
	}
}

func TestGenerateHandlesAPathThatNeedsQuoting(t *testing.T) {
	// A spec page's filename is unconstrained, and a space in it would end the
	// link destination early.
	r := repotest.NewWith(t, `{"name":"Paths"}`, map[string]string{
		"spec/a page.md": "---\ntitle: A page\nincludes: []\n---\n\n# A page\n",
	})

	got := string(index.Generate(r))
	if !strings.Contains(got, "(spec/a%20page.md)") {
		t.Errorf("a path with a space was not encoded:\n%s", got)
	}
}

func TestGenerateEncodesEveryCharacterThatWouldEndTheDestination(t *testing.T) {
	// The angle-bracket form this replaced could be escaped out of: a path's
	// own backslash paired with the backslash added before a ">", and the ">"
	// then closed the destination early, leaving a working hyperlink of the
	// filename's choosing inside the cell. Encoding has no such interaction.
	for _, name := range []string{
		`spec/a|b.md`, `spec/c#d.md`, `spec/e(f).md`,
		`spec/g>) [Click here](https:evil.example) h.md`,
		"spec/i\nj.md",
	} {
		r := repotest.NewWith(t, `{"name":"Paths"}`, map[string]string{
			name: "---\ntitle: A page\nincludes: []\n---\n\n# A page\n",
		})

		got := string(index.Generate(r))
		// Page, Title, Includes, Stale. The count is asserted because a
		// character escaping its cell shows up as an extra one.
		row := cellsIn(t, got, "](spec/")
		if len(row) != 4 {
			t.Errorf("%q produced %d cells, want 4:\n%s", name, len(row), got)
		}
		if strings.Contains(got, "evil.example)") && !strings.Contains(got, "%3E") {
			t.Errorf("%q rendered a live hyperlink:\n%s", name, got)
		}
	}
}

// cellsIn returns the cells of the first table row naming want, splitting the
// way GFM does: on a pipe that is not escaped. Splitting on every pipe would
// count an escaped one as a cell boundary and so report the escaping as broken
// when it is working.
func cellsIn(t *testing.T, rendered, want string) []string {
	t.Helper()
	for line := range strings.SplitSeq(rendered, "\n") {
		if !strings.Contains(line, want) || !strings.HasPrefix(line, "|") {
			continue
		}
		trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")

		var cells []string
		var cell strings.Builder
		for i := 0; i < len(trimmed); i++ {
			switch {
			case trimmed[i] == '\\' && i+1 < len(trimmed):
				cell.WriteByte(trimmed[i])
				i++
				cell.WriteByte(trimmed[i])
			case trimmed[i] == '|':
				cells = append(cells, cell.String())
				cell.Reset()
			default:
				cell.WriteByte(trimmed[i])
			}
		}
		return append(cells, cell.String())
	}
	t.Fatalf("no row naming %q in:\n%s", want, rendered)
	return nil
}

func TestGenerateKeepsAPipeInsideItsCell(t *testing.T) {
	// A pipe splits a table cell wherever it lands, and angle brackets do not
	// help: GFM counts cells before it parses links. A single spec page named
	// with one broke every row that referenced it.
	r := repotest.New(t, map[string]string{
		"spec/a|b.md": "---\ntitle: Piped\nincludes: []\n---\n\n# Piped\n\nProse.\n",
	})

	// Page, Title, Includes, Stale. A pipe escaping its cell shows up as a
	// fifth.
	if got := len(cellsIn(t, string(index.Generate(r)), "](spec/")); got != 4 {
		t.Errorf("the Spec row has %d cells, want 4", got)
	}
}

func TestGenerateKeepsAHashOutOfTheDestination(t *testing.T) {
	// An unescaped # in a path becomes a fragment, so the link points at a file
	// that does not exist and lint cannot see it.
	r := repotest.New(t, map[string]string{
		"spec/c#d.md": "---\ntitle: Hashed\nincludes: []\n---\n\n# Hashed\n\nProse.\n",
	})

	rendered := string(index.Generate(r))
	if !strings.Contains(rendered, `%23`) && !strings.Contains(rendered, `\#`) {
		t.Errorf("the hash in the path is neither encoded nor escaped:\n%s", rendered)
	}
}

func TestGenerateEscapesTheProjectName(t *testing.T) {
	// The name is free text from archdoc.json and went into the H1 raw.
	r := repotest.NewWith(t, `{"name":"Pwned](https://evil.example.com) x"}`, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\nProse.\n",
	})

	// The bracket must be escaped, not merely present: "\](" still contains
	// "](", so a plain substring check passes on escaped output too.
	first, _, _ := strings.Cut(string(index.Generate(r)), "\n")
	if strings.Contains(strings.ReplaceAll(first, `\]`, ""), "](https://evil.example.com)") {
		t.Errorf("the project name rendered as a live link: %s", first)
	}
}

func TestGenerateNeutralisesRawHTMLInATitle(t *testing.T) {
	// A title is free text and the index is rendered markdown, where raw HTML
	// passes straight through. Escaping brackets and pipes but not angle
	// brackets left a title able to put live markup into a generated file.
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: \"A <b>bold</b> idea\"\nstatus: draft\n---\n\n# RFC-0001: A <b>bold</b> idea\n",
	})

	// Both characters, asserted separately: escaping only one of them still
	// removes the literal "<b>" from the output while leaving markup that a
	// renderer acts on.
	got := string(index.Generate(r))
	row := cellsIn(t, got, "RFC-0001")
	for _, c := range []string{"<", ">"} {
		if strings.Contains(strings.Join(row, ""), c) {
			t.Errorf("a title put a raw %q into the index row: %q", c, row)
		}
	}
}

func TestGenerateLeavesADanglingIdentifierAsText(t *testing.T) {
	// A name that resolves to nothing is rendered as text, and L04 reports it.
	// Linking it instead would dereference nil, and no index test had one,
	// because the golden fixture resolves everything.
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: [RFC-9999]\n---\n\n# Page\n\nProse.\n",
	})

	got := string(index.Generate(r))
	if !strings.Contains(got, "RFC-9999") {
		t.Fatalf("the dangling identifier is not in the index at all:\n%s", got)
	}
	if strings.Contains(got, "[RFC-9999](") {
		t.Errorf("a dangling identifier was rendered as a link:\n%s", got)
	}
}

// TestImplementedInOmitsTheGlossary keeps the column honest. The boolean and
// the column have to agree: a document the glossary merely names is not
// implemented, so listing the glossary under "Implemented in" would contradict
// the fact the same row is reporting.
func TestImplementedInOmitsTheGlossary(t *testing.T) {
	const accepted = "---\nid: %s\ntitle: %s\nstatus: accepted\ncreated: 2026-01-01\n" +
		"decided: 2026-01-02\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# %s: %s\n\n## Abstract\n\nWords.\n"
	r := repotest.New(t, map[string]string{
		"rfc/0001-named.md": fmt.Sprintf(accepted, "RFC-0001", "Named", "RFC-0001", "Named"),
		"rfc/0002-both.md":  fmt.Sprintf(accepted, "RFC-0002", "Both", "RFC-0002", "Both"),
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: [RFC-0001, RFC-0002]\n---\n\n# Glossary\n\n" +
			"## Binding\n\nA registered resolution.\n",
		"spec/container.md": "---\ntitle: Container\nincludes: [RFC-0002]\n---\n\n# Container\n\nWhat it does.\n",
	})
	generated := string(index.Generate(r))

	for _, line := range strings.Split(generated, "\n") {
		if !strings.Contains(line, "RFC-0001") || !strings.HasPrefix(line, "| [RFC-0001]") {
			continue
		}
		if strings.Contains(line, "glossary") {
			t.Errorf("RFC-0001 is listed as implemented in the glossary:\n%s", line)
		}
	}
	// A document in both is still implemented, by the page that is not the
	// glossary.
	var row string
	for _, line := range strings.Split(generated, "\n") {
		if strings.HasPrefix(line, "| [RFC-0002]") {
			row = line
		}
	}
	if !strings.Contains(row, "container") {
		t.Errorf("RFC-0002 lost the page that does implement it:\n%s", row)
	}
	if strings.Contains(row, "glossary") {
		t.Errorf("RFC-0002 still lists the glossary as implementing it:\n%s", row)
	}
}

// TestTheSpecTableCarriesATitle gives a reader the page's name. The slug stays
// as the identity, the way an identifier does for the other types, so this is
// the column the table was missing rather than a change to what it renders.
func TestTheSpecTableCarriesATitle(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n" +
			"## Binding\n\nA registered resolution.\n",
	})
	generated := string(index.Generate(r))

	var header, row string
	for _, line := range strings.Split(generated, "\n") {
		if strings.HasPrefix(line, "| Page |") {
			header = line
		}
		if strings.HasPrefix(line, "| [glossary]") {
			row = line
		}
	}
	if header == "" {
		t.Fatalf("no Spec table header:\n%s", generated)
	}
	if !strings.Contains(header, "| Title |") {
		t.Errorf("the Spec table has no Title column:\n%s", header)
	}
	if row == "" {
		t.Fatalf("no glossary row, so the slug is no longer the identity:\n%s", generated)
	}
	if !strings.Contains(row, "| Glossary |") {
		t.Errorf("the row carries no title:\n%s", row)
	}
}

// TestASpecPageTitleCannotBreakItsRow covers the column added alongside the
// slug. A title is free text a person writes, and the existing pipe and
// encoding tests exercise the path rather than the title, so the new cell was
// rendering unescaped with nothing to notice.
func TestASpecPageTitleCannotBreakItsRow(t *testing.T) {
	for _, title := range []string{
		`Piped | Title`,
		`Brackets [here](https://evil.example)`,
		`Angle <b>bold</b>`,
	} {
		r := repotest.NewWith(t, `{"name":"Titles"}`, map[string]string{
			"spec/page.md": "---\ntitle: " + `"` + strings.ReplaceAll(title, `"`, `\"`) + `"` +
				"\nincludes: []\n---\n\n# " + title + "\n\nProse.\n",
		})
		got := string(index.Generate(r))

		if cells := len(cellsIn(t, got, "](spec/")); cells != 4 {
			t.Errorf("title %q produced %d cells, want 4:\n%s", title, cells, got)
		}
		if strings.Contains(got, "evil.example)") && !strings.Contains(got, `\[`) {
			t.Errorf("title %q rendered a live hyperlink:\n%s", title, got)
		}
		if strings.Contains(got, "<b>") {
			t.Errorf("title %q reached the index as live markup:\n%s", title, got)
		}
	}
}
