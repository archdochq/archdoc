package repo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/repotest"
)

// fixturePath is the shared fixture repository, used by every package that
// needs a realistic set of documents.
const fixturePath = "../../testdata/repo/archdoc.json"

func openFixture(t *testing.T) *repo.Repo {
	t.Helper()
	return repotest.Fixture(t, "repo")
}

func TestOpenDiscoversDocumentsByDirectory(t *testing.T) {
	r := openFixture(t)

	var got []string
	for _, d := range r.Documents() {
		got = append(got, d.Path)
	}

	want := []string{
		"adr/0001-storage-engine.md",
		"adr/0002-no-orm.md",
		"ref/0001-engine-comparison.md",
		"ref/0002-queue-survey.md",
		"rfc/0001-database.md",
		"rfc/0002-caching.md",
		"rfc/0003-cache-expiry.md",
		"rfc/0004-database-component-on-postgresql.md",
		"rfc/0005-job-queues.md",
		"rfc/0006-federated-search.md",
		"rfc/0007-scheduling.md",
		"rfc/0008-caching-rewrite.md",
		"rfc/0009-legacy-authentication.md",
		"rfc/0010-queue-durability.md",
		"spec/caching.md",
		"spec/database.md",
		"spec/glossary.md",
		"spec/http/routing.md",
	}

	if !slices.Equal(got, want) {
		t.Errorf("discovered documents:\n got %q\nwant %q", got, want)
	}
}

func TestOpenDerivesIdentifiersForNumberedTypes(t *testing.T) {
	r := openFixture(t)

	for p, want := range map[string]string{
		"rfc/0001-database.md":                         "RFC-0001",
		"rfc/0004-database-component-on-postgresql.md": "RFC-0004",
		"adr/0002-no-orm.md":                           "ADR-0002",
		"ref/0001-engine-comparison.md":                "REF-0001",
	} {
		if got := repotest.Document(t, r, p).ID; got != want {
			t.Errorf("%s: ID = %q, want %q", p, got, want)
		}
	}
}

func TestOpenNamesSpecPagesByPathBelowSpec(t *testing.T) {
	r := openFixture(t)

	for p, want := range map[string]string{
		"spec/database.md":     "database",
		"spec/glossary.md":     "glossary",
		"spec/http/routing.md": "http/routing",
	} {
		d := repotest.Document(t, r, p)
		if d.Page != want {
			t.Errorf("%s: Page = %q, want %q", p, d.Page, want)
		}
		if d.ID != "" {
			t.Errorf("%s: ID = %q, want empty: spec pages are not numbered", p, d.ID)
		}
	}
}

func TestOpenParsesRFCFrontMatter(t *testing.T) {
	d := repotest.Document(t, openFixture(t), "rfc/0004-database-component-on-postgresql.md")
	fm := d.FrontMatter

	if fm.ID != "RFC-0004" {
		t.Errorf("id = %q, want %q", fm.ID, "RFC-0004")
	}
	if fm.Title != "Database component on PostgreSQL" {
		t.Errorf("title = %q, want %q", fm.Title, "Database component on PostgreSQL")
	}
	if fm.Status != repo.StatusAccepted {
		t.Errorf("status = %q, want %q", fm.Status, repo.StatusAccepted)
	}
	if got := fm.Created.Format("2006-01-02"); got != "2026-04-01" {
		t.Errorf("created = %q, want %q", got, "2026-04-01")
	}
	if got := fm.Decided.Format("2006-01-02"); got != "2026-05-02" {
		t.Errorf("decided = %q, want %q", got, "2026-05-02")
	}
	if want := []string{"RFC-0002", "ADR-0001"}; !slices.Equal(fm.Depends, want) {
		t.Errorf("depends = %q, want %q", fm.Depends, want)
	}
	if want := []string{"RFC-0001"}; !slices.Equal(fm.Obsoletes, want) {
		t.Errorf("obsoletes = %q, want %q", fm.Obsoletes, want)
	}
	if len(fm.Updates) != 0 {
		t.Errorf("updates = %q, want empty", fm.Updates)
	}
}

func TestOpenDistinguishesAnEmptyKeyFromAnAbsentOne(t *testing.T) {
	r := openFixture(t)

	// A draft writes `decided:` with no value. The key is present, the value
	// empty. L07 needs to tell this from a key that was never written.
	draft := repotest.Document(t, r, "rfc/0007-scheduling.md").FrontMatter
	if !draft.Has("decided") {
		t.Error(`draft: Has("decided") = false, want true: the key is written`)
	}
	if !draft.Decided.IsZero() {
		t.Errorf("draft: decided = %v, want the zero time", draft.Decided)
	}

	// A ref has no decided key at all.
	ref := repotest.Document(t, r, "ref/0001-engine-comparison.md").FrontMatter
	if ref.Has("decided") {
		t.Error(`ref: Has("decided") = true, want false: the key is absent`)
	}
	if got := ref.Verified.Format("2006-01-02"); got != "2026-08-01" {
		t.Errorf("ref: verified = %q, want %q", got, "2026-08-01")
	}
}

func TestOpenParsesSpecFrontMatter(t *testing.T) {
	fm := repotest.Document(t, openFixture(t), "spec/database.md").FrontMatter

	if fm.Title != "Database" {
		t.Errorf("title = %q, want %q", fm.Title, "Database")
	}
	if want := []string{"RFC-0001", "RFC-0004", "ADR-0001"}; !slices.Equal(fm.Includes, want) {
		t.Errorf("includes = %q, want %q", fm.Includes, want)
	}
}

func TestHeadingsIgnoreFencedCodeBlocks(t *testing.T) {
	d := repotest.Document(t, openFixture(t), "rfc/0007-scheduling.md")

	var got []string
	for _, h := range d.Headings() {
		got = append(got, h.Text)
	}
	want := []string{
		"RFC-0007: Job scheduling", "Abstract", "Motivation", "Proposal",
		"Alternatives considered", "Backwards compatibility", "Open questions", "Changelog",
	}
	if !slices.Equal(got, want) {
		t.Errorf("headings:\n got %q\nwant %q\n(the Proposal section holds a fenced block whose first line starts with #)", got, want)
	}
}

func TestH1IsTheFirstHeading(t *testing.T) {
	d := repotest.Document(t, openFixture(t), "rfc/0004-database-component-on-postgresql.md")

	h, ok := d.H1()
	if !ok {
		t.Fatal("H1() reported no heading")
	}
	if want := "RFC-0004: Database component on PostgreSQL"; h.Text != want {
		t.Errorf("H1 = %q, want %q", h.Text, want)
	}
	if h.Level != 1 {
		t.Errorf("H1 level = %d, want 1", h.Level)
	}
}

func TestSectionIsEmptyWhenItHoldsOnlyWhitespaceAndComments(t *testing.T) {
	r := openFixture(t)

	// RFC-0006 was withdrawn before it was written; its Abstract holds only the
	// template comment.
	withdrawn := repotest.Document(t, r, "rfc/0006-federated-search.md")
	s, ok := repotest.Section(withdrawn, "Abstract")
	if !ok {
		t.Fatal("withdrawn RFC has no Abstract section")
	}
	if !s.Empty() {
		t.Errorf("Abstract holding only an HTML comment is not empty; text = %q", s.Text)
	}

	// A written section is not empty.
	written, ok := repotest.Section(repotest.Document(t, r, "rfc/0004-database-component-on-postgresql.md"), "Abstract")
	if !ok {
		t.Fatal("accepted RFC has no Abstract section")
	}
	if written.Empty() {
		t.Error("a written Abstract reports itself empty")
	}
}

func TestAnchorsSuffixRepeatedHeadings(t *testing.T) {
	d := repotest.Document(t, openFixture(t), "spec/database.md")

	var got []string
	for _, h := range d.Headings() {
		got = append(got, h.Anchor)
	}
	want := []string{"database", "configuration", "configuration-1"}
	if !slices.Equal(got, want) {
		t.Errorf("anchors:\n got %q\nwant %q", got, want)
	}
}

func TestDerivesReverseRelationships(t *testing.T) {
	r := openFixture(t)

	for _, tc := range []struct {
		id                                             string
		updatedBy, obsoletedBy, dependedOnBy, included []string
	}{
		{
			id:          "RFC-0001",
			obsoletedBy: []string{"RFC-0004"},
			included:    []string{"database"},
		},
		{
			id:           "RFC-0002",
			updatedBy:    []string{"RFC-0003"},
			obsoletedBy:  []string{"RFC-0008"},
			dependedOnBy: []string{"RFC-0003", "RFC-0004"},
			included:     []string{"caching"},
		},
		{
			id:           "ADR-0001",
			dependedOnBy: []string{"ADR-0002", "RFC-0004"},
			included:     []string{"database"},
		},
		{
			id:           "RFC-0004",
			dependedOnBy: []string{"RFC-0005", "RFC-0007"},
			included:     []string{"database", "glossary"},
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			d := r.ByID(tc.id)
			if d == nil {
				t.Fatalf("no document %s", tc.id)
			}
			if !slices.Equal(d.UpdatedBy, tc.updatedBy) {
				t.Errorf("UpdatedBy = %q, want %q", d.UpdatedBy, tc.updatedBy)
			}
			if !slices.Equal(d.ObsoletedBy, tc.obsoletedBy) {
				t.Errorf("ObsoletedBy = %q, want %q", d.ObsoletedBy, tc.obsoletedBy)
			}
			if !slices.Equal(d.DependedOnBy, tc.dependedOnBy) {
				t.Errorf("DependedOnBy = %q, want %q", d.DependedOnBy, tc.dependedOnBy)
			}
			if !slices.Equal(d.IncludedIn, tc.included) {
				t.Errorf("IncludedIn = %q, want %q", d.IncludedIn, tc.included)
			}
		})
	}
}

func TestObsolescenceCountsOnlyWhenTheObsoletingDocumentIsAccepted(t *testing.T) {
	r := openFixture(t)

	// RFC-0004 is accepted and obsoletes RFC-0001.
	if d := r.ByID("RFC-0001"); !d.EffectivelyObsolete {
		t.Error("RFC-0001 obsoleted by an accepted RFC is not effectively obsolete")
	}
	// RFC-0008 is only a draft, so its claim on RFC-0002 has no effect.
	if d := r.ByID("RFC-0002"); d.EffectivelyObsolete {
		t.Error("RFC-0002 obsoleted only by a draft is effectively obsolete")
	}
}

func TestStalenessFollowsEffectiveObsolescence(t *testing.T) {
	r := openFixture(t)

	if d := r.ByPage("database"); !d.Stale {
		t.Error("spec/database.md includes the effectively obsolete RFC-0001 but is not stale")
	}
	if d := r.ByPage("caching"); d.Stale {
		t.Error("spec/caching.md is stale, but the only claim on RFC-0002 is from a draft")
	}
}

func TestImplementedMeansAcceptedAndIncludedBySomeSpecPage(t *testing.T) {
	r := openFixture(t)

	if d := r.ByID("RFC-0004"); !d.Implemented {
		t.Error("RFC-0004 is accepted and included by spec/database.md but not implemented")
	}
	if d := r.ByID("RFC-0005"); d.Implemented {
		t.Error("RFC-0005 is only proposed but reports itself implemented")
	}
	if d := r.ByID("RFC-0007"); d.Implemented {
		t.Error("RFC-0007 is a draft included by no page but reports itself implemented")
	}
}

func TestGlossaryParsesEntriesAndIgnoresPreamble(t *testing.T) {
	entries, ok := openFixture(t).Glossary()
	if !ok {
		t.Fatal("Glossary() reported no glossary page")
	}

	var terms []string
	for _, e := range entries {
		terms = append(terms, e.Term)
	}
	if want := []string{"Ref", "Spec page", "Wings"}; !slices.Equal(terms, want) {
		t.Fatalf("terms = %q, want %q (the leading comment is preamble, not an entry)", terms, want)
	}

	ref := entries[0]
	if len(ref.Paragraphs) != 1 {
		t.Errorf("Ref paragraphs = %d, want 1: %q", len(ref.Paragraphs), ref.Paragraphs)
	}
	if ref.Anchor != "ref" {
		t.Errorf("Ref anchor = %q, want %q", ref.Anchor, "ref")
	}

	page := entries[1]
	if len(page.Paragraphs) != 1 {
		t.Errorf("Spec page paragraphs = %d, want 1: a Formerly line is not a paragraph; got %q", len(page.Paragraphs), page.Paragraphs)
	}
	if want := []string{"Specification page"}; !slices.Equal(page.Formerly, want) {
		t.Errorf("Spec page Formerly = %q, want %q", page.Formerly, want)
	}
	if page.Anchor != "spec-page" {
		t.Errorf("Spec page anchor = %q, want %q", page.Anchor, "spec-page")
	}
}

func TestGlossaryIsAbsentWhenThePageIsMissing(t *testing.T) {
	if _, ok := repotest.New(t, nil).Glossary(); ok {
		t.Error("Glossary() reported a glossary for a repository that has none")
	}
}

func TestMalformedFilenameYieldsNoIdentifier(t *testing.T) {
	d := repotest.Document(t, openLintFixture(t), "rfc/notes.md")

	if d.ID != "" {
		t.Errorf("ID = %q, want empty: the filename carries no number", d.ID)
	}
	if d.Number != 0 {
		t.Errorf("Number = %d, want 0", d.Number)
	}
	// The front matter is still parsed, so lint can compare the two and report
	// the mismatch.
	if d.FrontMatter.ID != "RFC-0100" {
		t.Errorf("front matter id = %q, want %q", d.FrontMatter.ID, "RFC-0100")
	}
}

func TestDocumentWithoutFrontMatterRecordsAProblem(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"spec/bare.md": "# Bare\n\nA page that opens with no front matter block at all.\n",
	})

	d := repotest.Document(t, r, "spec/bare.md")
	if len(d.Problems) == 0 {
		t.Fatal("a document with no front matter recorded no problem")
	}
	if !strings.Contains(d.Problems[0].Message, "front matter") {
		t.Errorf("problem = %q, want it to mention front matter", d.Problems[0].Message)
	}
}

func TestLinkScanningIgnoresCodeSpansAndFences(t *testing.T) {
	d := repotest.Document(t, openLintFixture(t), "rfc/0014-wiki-links.md")

	var names []string
	for _, w := range d.WikiLinks() {
		names = append(names, w.Name)
	}
	want := []string{"Some Unknown Thing"}
	if !slices.Equal(names, want) {
		t.Errorf("WikiLinks = %q, want %q: a [[...]] in a code span or fence is not a link", names, want)
	}
}

func TestTypeOfClassifiesPathsTheWayDiscoveryDoes(t *testing.T) {
	for path, want := range map[string]repo.Type{
		"rfc/0001-a.md":    repo.TypeRFC,
		"adr/0001-a.md":    repo.TypeADR,
		"ref/0001-a.md":    repo.TypeRef,
		"spec/a.md":        repo.TypeSpec,
		"spec/http/a.md":   repo.TypeSpec, // spec pages may nest
		"rfc/archive/a.md": "",            // no other type may
		"rfc/notes.txt":    "",
		"README.md":        "",
		"elsewhere/a.md":   "",
	} {
		got, ok := repo.TypeOf(path)
		if want == "" {
			if ok {
				t.Errorf("TypeOf(%q) = %q, true; want not a document", path, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("TypeOf(%q) = %q, %v; want %q, true", path, got, ok, want)
		}
	}
}

func TestStatusInReadsTheStatusOfRawSource(t *testing.T) {
	source := []byte("---\nid: RFC-0001\nstatus: accepted\n---\n\n# RFC-0001: X\n")
	if got := repo.StatusIn(source); got != repo.StatusAccepted {
		t.Errorf("StatusIn = %q, want %q", got, repo.StatusAccepted)
	}
	if got := repo.StatusIn([]byte("# No front matter\n")); got != "" {
		t.Errorf("StatusIn = %q, want empty for a document with no front matter", got)
	}
}

func TestAnchorsInSkipsNestedFencesOfDifferentLengths(t *testing.T) {
	// A three-backtick example inside a four-backtick block. CommonMark closes a
	// fence only on a run at least as long as the one that opened it.
	source := []byte("---\ntitle: T\nincludes: []\n---\n\n# Doc\n\n````\n```\n# Not a heading\n```\n````\n\n## Real\n")

	got := repo.AnchorsIn(source)
	want := []string{"doc", "real"}
	if !slices.Equal(got, want) {
		t.Errorf("AnchorsIn = %q, want %q: the inner fence must not close the outer one", got, want)
	}
}

func TestAnchorsInLoopsUntilTheSuffixIsFree(t *testing.T) {
	// GitHub re-checks the suffixed anchor against the ones already taken, so a
	// heading that collides with an earlier suffix moves on to the next number.
	source := []byte("---\ntitle: T\nincludes: []\n---\n\n# T\n\n## Foo\n\n## Foo-1\n\n## Foo\n")

	got := repo.AnchorsIn(source)
	want := []string{"t", "foo", "foo-1", "foo-2"}
	if !slices.Equal(got, want) {
		t.Errorf("AnchorsIn = %q, want %q: anchors must be unique within a document", got, want)
	}
}

func openLintFixture(t *testing.T) *repo.Repo {
	t.Helper()
	return repotest.Fixture(t, "lint")
}

func glossaryEntry(t *testing.T, entries []repo.GlossaryEntry, term string, nth int) repo.GlossaryEntry {
	t.Helper()
	seen := 0
	for _, e := range entries {
		if e.Term != term {
			continue
		}
		if seen == nth {
			return e
		}
		seen++
	}
	t.Fatalf("no entry %d for term %q", nth, term)
	return repo.GlossaryEntry{}
}

func TestGlossaryReadsDuplicateTermsSeparately(t *testing.T) {
	entries, ok := openLintFixture(t).Glossary()
	if !ok {
		t.Fatal("no glossary")
	}

	// Two entries share the exact term "Cache". Looking a section up by title
	// would hand both of them the first entry's body.
	first := glossaryEntry(t, entries, "Cache", 0)
	second := glossaryEntry(t, entries, "Cache", 1)

	if len(first.Paragraphs) != 1 {
		t.Errorf("first Cache has %d paragraphs, want 1: %q", len(first.Paragraphs), first.Paragraphs)
	}
	if len(second.Paragraphs) != 2 {
		t.Errorf("second Cache has %d paragraphs, want 2: %q", len(second.Paragraphs), second.Paragraphs)
	}
}

func TestGlossaryAccumulatesAdjacentFormerlyLines(t *testing.T) {
	entries, ok := openLintFixture(t).Glossary()
	if !ok {
		t.Fatal("no glossary")
	}
	e := glossaryEntry(t, entries, "Renamed twice", 0)

	if want := []string{"First Name", "Second Name"}; !slices.Equal(e.Formerly, want) {
		t.Errorf("Formerly = %q, want %q: repeated renames accumulate lines", e.Formerly, want)
	}
	if len(e.Paragraphs) != 1 {
		t.Errorf("paragraphs = %d, want 1: a Formerly line is not a paragraph; got %q", len(e.Paragraphs), e.Paragraphs)
	}
}

func TestGlossarySplitsParagraphsInCRLFFiles(t *testing.T) {
	entries, ok := repotest.Fixture(t, "crlf").Glossary()
	if !ok {
		t.Fatal("no glossary")
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if len(entries[0].Paragraphs) != 2 {
		t.Errorf("paragraphs = %d, want 2: CRLF blank lines still separate paragraphs; got %q",
			len(entries[0].Paragraphs), entries[0].Paragraphs)
	}
}

func TestBackfilledIsOptionalAndParsed(t *testing.T) {
	r := openFixture(t)

	backfilled := repotest.Document(t, r, "rfc/0009-legacy-authentication.md").FrontMatter
	if !backfilled.Has("backfilled") {
		t.Error(`Has("backfilled") = false, want true`)
	}
	if got := backfilled.Backfilled.Format("2006-01-02"); got != "2026-09-11" {
		t.Errorf("backfilled = %q, want %q", got, "2026-09-11")
	}
	if got := backfilled.Created.Format("2006-01-02"); got != "2023-04-02" {
		t.Errorf("created = %q, want the historical date, not the date it was written", got)
	}

	// Every other document omits the key entirely, and must stay valid.
	ordinary := repotest.Document(t, r, "rfc/0004-database-component-on-postgresql.md").FrontMatter
	if ordinary.Has("backfilled") {
		t.Error(`Has("backfilled") = true on a document that does not carry it`)
	}
	if !slices.Contains(repo.Schema(repo.TypeRFC), "backfilled") {
		t.Error("backfilled is not in the RFC schema, so it would be reported as an unknown field")
	}
	if slices.Contains(repo.RequiredKeys(repo.TypeRFC), "backfilled") {
		t.Error("backfilled is required; a required key cannot be added, because frozen documents cannot gain one")
	}
}

func TestSlugFollowsTheFilenameRules(t *testing.T) {
	for title, want := range map[string]string{
		"Database":                         "database",
		"Database component on PostgreSQL": "database-component-on-postgresql",
		"Database: PostgreSQL":             "database-postgresql",
		"  Leading and trailing  ":         "leading-and-trailing",
		"Multiple---hyphens":               "multiple-hyphens",
		"Ollie's plan (v2)":                "ollie-s-plan-v2",
		"café":                             "caf",
	} {
		if got := repo.Slug(title); got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestNextNumbersFromTheHighestExisting(t *testing.T) {
	r := openFixture(t)

	// The fixture's highest RFC is 0010, and gaps do not matter.
	id, path, err := r.Next(repo.TypeRFC, "A new design")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if id != "RFC-0011" {
		t.Errorf("id = %q, want %q", id, "RFC-0011")
	}
	if want := "rfc/0011-a-new-design.md"; path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

func TestNextRefusesATitleThatSlugsToNothing(t *testing.T) {
	if _, _, err := openFixture(t).Next(repo.TypeRFC, "   !!!   "); err == nil {
		t.Error("Next accepted a title with no usable characters")
	}
}

func TestReverseListsNeverHoldAnEmptyIdentifier(t *testing.T) {
	// A numbered document whose filename carries no number has no identifier.
	// Contributing an empty string to a target's reverse list puts a stray
	// comma in the index of a repository that is already failing L02.
	const real = "---\nid: RFC-0001\ntitle: Real\nstatus: accepted\ncreated: 2026-01-01\ndecided: 2026-02-01\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# RFC-0001: Real\n"
	const unnumbered = "---\nid: RFC-0002\ntitle: Notes\nstatus: draft\ncreated: 2026-01-01\ndecided:\ndepends: [RFC-0001]\nupdates: []\nobsoletes: []\n---\n\n# RFC-0002: Notes\n"

	r := repotest.New(t, map[string]string{
		"rfc/0001-real.md": real,
		"rfc/notes.md":     unnumbered,
	})

	target := repotest.Document(t, r, "rfc/0001-real.md")
	if slices.Contains(target.DependedOnBy, "") {
		t.Errorf("DependedOnBy holds an empty identifier: %q", target.DependedOnBy)
	}
}

// The body used by the Prose tests: a code span, a fenced block, a commented
// heading, and a plain line after all three.
const proseBody = "Read [[RFC-0001]] and `[[RFC-0002]]` today.\n" +
	"```\n" +
	"[[RFC-0003]]\n" +
	"```\n" +
	"<!--\n" +
	"## Commented out\n" +
	"-->\n" +
	"The end.\n"

func TestProseSeparatesTheLineAsWrittenFromTheLineMatchedAgainst(t *testing.T) {
	// The pair is the point: a decision is made from Masked, and the edit it
	// leads to is applied to Raw at the same offsets. Three shipped defects
	// came from applying a match found in one to the other.
	var first repo.ProseLine
	for line := range repo.Prose(proseBody, 10) {
		first = line
		break
	}

	if first.Raw != "Read [[RFC-0001]] and `[[RFC-0002]]` today." {
		t.Errorf("Raw = %q, want the line as written", first.Raw)
	}
	// The span is blanked delimiters and all, which is why the assertion is
	// built rather than written out: what matters is the length, not the
	// backticks.
	blanked := "Read [[RFC-0001]] and " + strings.Repeat(" ", len("`[[RFC-0002]]`")) + " today."
	if first.Masked != blanked {
		t.Errorf("Masked = %q, want %q", first.Masked, blanked)
	}
	if len(first.Raw) != len(first.Masked) {
		t.Errorf("Raw is %d bytes and Masked is %d: an offset in one must mean the same column in the other",
			len(first.Raw), len(first.Masked))
	}
	if first.Number != 10 {
		t.Errorf("Number = %d, want 10: the first body line is at bodyLine", first.Number)
	}
}

func TestProseYieldsNoContentFromFencesOrComments(t *testing.T) {
	// A fenced block is not yielded at all. A comment is yielded blank rather
	// than dropped, so that prose sharing a line with one survives and every
	// offset still lines up with the line as written.
	var got []repo.ProseLine
	for line := range repo.Prose(proseBody, 1) {
		got = append(got, line)
	}

	if len(got) != 6 {
		t.Fatalf("Prose yielded %d lines, want 6: the three lines of the fence are skipped", len(got))
	}
	for _, line := range got {
		if len(line.Raw) != len(line.Masked) {
			t.Errorf("line %d: Raw is %d bytes and Masked %d, so an offset in one is not the other",
				line.Number, len(line.Raw), len(line.Masked))
		}
		for _, hidden := range []string{"RFC-0002", "RFC-0003", "Commented out"} {
			if strings.Contains(line.Masked, hidden) {
				t.Errorf("line %d masked as %q, which still shows %q", line.Number, line.Masked, hidden)
			}
		}
	}
	if !strings.Contains(got[0].Masked, "RFC-0001") {
		t.Errorf("the live link outside the code span was masked away: %q", got[0].Masked)
	}
	if got[4].Masked != "The end." {
		t.Errorf("the line after the comment is %q, want %q", got[4].Masked, "The end.")
	}
}

func TestProseStopsWhenTheCallerBreaks(t *testing.T) {
	// The callback form could not express this: its `return` meant "next line",
	// which read as "stop" and was misread that way once in link.Suggest.
	seen := 0
	for range repo.Prose(proseBody, 1) {
		seen++
		break
	}

	if seen != 1 {
		t.Errorf("the loop body ran %d times after breaking on the first, want 1", seen)
	}
}

func TestDocumentProseNumbersLinesFromTheFile(t *testing.T) {
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\nProse.\n",
	}).ByPath("rfc/0001-a.md")

	var numbers []int
	for line := range d.Prose() {
		if strings.TrimSpace(line.Raw) != "" {
			numbers = append(numbers, line.Number)
		}
	}

	if want := []int{6, 8}; !slices.Equal(numbers, want) {
		t.Errorf("prose lines are at %v, want %v: the H1 and the body line as numbered in the file", numbers, want)
	}
}

func TestASingleLineCommentIsNothingLikeAMultiLineOne(t *testing.T) {
	// A comment is nothing wherever it appears. Until this held, a [[...]] in a
	// <!-- --> on one line was rewritten by archdoc link and reported by L16,
	// while the same text in a comment spanning two lines was invisible to
	// both.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n" +
			"<!-- one line [[RFC-0002]] -->\n\n" +
			"<!--\nacross lines [[RFC-0003]]\n-->\n\n" +
			"Live [[RFC-0004]] here.\n",
	}).ByPath("rfc/0001-a.md")

	var names []string
	for _, l := range d.WikiLinks() {
		names = append(names, l.Name)
	}
	if want := []string{"RFC-0004"}; !slices.Equal(names, want) {
		t.Errorf("WikiLinks = %q, want %q: only the uncommented link is live", names, want)
	}
}

func TestProseKeepsWhatFollowsAClosingComment(t *testing.T) {
	// The closing line was skipped whole, so anything after --> was lost with
	// the comment it followed.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n" +
			"<!--\nhidden [[RFC-0002]]\n--> but [[RFC-0003]] is not.\n",
	}).ByPath("rfc/0001-a.md")

	var names []string
	for _, l := range d.WikiLinks() {
		names = append(names, l.Name)
	}
	if want := []string{"RFC-0003"}; !slices.Equal(names, want) {
		t.Errorf("WikiLinks = %q, want %q: text after --> is prose", names, want)
	}
}

func TestAHeadingIsNotItsTrailingComment(t *testing.T) {
	// The comment is nothing, so it is not part of the title and not part of
	// the anchor derived from it.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n" +
			"## Open questions <!-- none left -->\n",
	}).ByPath("rfc/0001-a.md")

	titles := d.SectionTitles()
	if want := []string{"Open questions"}; !slices.Equal(titles, want) {
		t.Errorf("SectionTitles = %q, want %q", titles, want)
	}
}

// wikiNames is the wiki links of a one-document repository with the given body,
// which is how these tests ask what the walk considered prose.
func wikiNames(t *testing.T, body string) []string {
	t.Helper()
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n" + body,
	}).ByPath("rfc/0001-a.md")

	var names []string
	for _, l := range d.WikiLinks() {
		names = append(names, l.Name)
	}
	return names
}

func TestACommentMarkerInsideACodeSpanIsLiteral(t *testing.T) {
	// A code span binds tighter than raw HTML, so the marker is text and no
	// comment opens. It used to open one, which swallowed the rest of the
	// document.
	got := wikiNames(t, "A `<!--` marker, then [[RFC-0002]].\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q: the marker in a code span opened a comment", got, want)
	}
}

func TestACommentMarkerInsideAFenceIsLiteral(t *testing.T) {
	// The fence opens first, so everything until it closes is literal.
	got := wikiNames(t, "```\n<!--\n```\n\nLive [[RFC-0002]] here.\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q: the marker in a fence opened a comment", got, want)
	}
}

func TestAFenceMarkerInsideACommentIsNotAFence(t *testing.T) {
	// The comment opens first, so it claims the lines that follow. Both are
	// leaf blocks, and the one that opens first wins.
	got := wikiNames(t, "<!--\n```\nhidden [[RFC-0003]]\n-->\n\nLive [[RFC-0002]] here.\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q", got, want)
	}
}

func TestAWholeCommentInsideACodeSpanIsLiteral(t *testing.T) {
	got := wikiNames(t, "A `<!-- x -->` span, then [[RFC-0002]].\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q", got, want)
	}
}

func TestAHeadingKeepsACommentMarkerInACodeSpan(t *testing.T) {
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n## The `<!--` marker\n",
	}).ByPath("rfc/0001-a.md")

	if got, want := d.SectionTitles(), []string{"The `<!--` marker"}; !slices.Equal(got, want) {
		t.Errorf("SectionTitles = %q, want %q", got, want)
	}
}

func FuzzProseNeverMovesAByte(f *testing.F) {
	// The whole design rests on this: a caller finds a match in Masked and
	// splices into Raw at the same offsets. If masking ever shortened a line,
	// moved a byte or wrote anything but a space, every edit built on it would
	// land in the wrong column, silently.
	for _, seed := range []string{
		proseBody,
		"A `<!--` marker, then [[X]].\n",
		"<!-- one line --> and [[X]]\n",
		"<!--\nacross\n--> after\n",
		"```\n<!--\n```\ntext\n",
		"<!-->empty\n",
		"`a` <!-- `b` --> `c`\n",
		"unterminated <!-- runs on\nand on\n",
		"é <!-- é --> é\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		raw := strings.Split(body, "\n")
		for line := range repo.Prose(body, 1) {
			if line.Number < 1 || line.Number > len(raw) {
				t.Fatalf("line number %d is outside the body's %d lines", line.Number, len(raw))
			}
			if line.Raw != raw[line.Number-1] {
				t.Fatalf("line %d: Raw is %q, but the body has %q", line.Number, line.Raw, raw[line.Number-1])
			}
			if len(line.Masked) != len(line.Raw) {
				t.Fatalf("line %d: Masked is %d bytes and Raw is %d", line.Number, len(line.Masked), len(line.Raw))
			}
			for i := range len(line.Raw) {
				if line.Masked[i] != line.Raw[i] && line.Masked[i] != ' ' {
					t.Fatalf("line %d, byte %d: masked to %q, which is neither the original %q nor a space",
						line.Number, i, line.Masked[i], line.Raw[i])
				}
			}
		}
	})
}

func TestASectionHoldingAnUnterminatedCommentIsEmpty(t *testing.T) {
	// The prose walk reads an unterminated comment as running to the end of the
	// body. Section.Empty asked a regular expression that required a closing
	// marker, so it answered the opposite: deleting a "-->" made an empty
	// section read as full, which let an unfinished document past the freeze
	// gate and silenced L14 on it forever.
	d := repotest.New(t, map[string]string{
		"adr/0001-a.md": "---\nid: ADR-0001\ntitle: A\n---\n\n# ADR-0001: A\n\n" +
			"## Consequences\n\n<!-- What becomes easier, what becomes harder\n",
	}).ByPath("adr/0001-a.md")

	sections := d.Sections("Consequences")
	if len(sections) != 1 {
		t.Fatalf("want one Consequences section, got %d", len(sections))
	}
	if !sections[0].Empty() {
		t.Errorf("a section holding only an unterminated comment is not empty; its text is %q", sections[0].Text)
	}
}

func TestASectionHoldingACommentMarkerInCodeIsNotEmpty(t *testing.T) {
	// The other direction: code is content, and a marker inside it opens
	// nothing, so the section holds something.
	d := repotest.New(t, map[string]string{
		"adr/0001-a.md": "---\nid: ADR-0001\ntitle: A\n---\n\n# ADR-0001: A\n\n" +
			"## Consequences\n\nWrite `<!--` to open one.\n",
	}).ByPath("adr/0001-a.md")

	if d.Sections("Consequences")[0].Empty() {
		t.Error("a section holding a code span reads as empty")
	}
}

func TestASectionHoldingAFenceIsNotEmpty(t *testing.T) {
	d := repotest.New(t, map[string]string{
		"adr/0001-a.md": "---\nid: ADR-0001\ntitle: A\n---\n\n# ADR-0001: A\n\n" +
			"## Consequences\n\n```\n<!--\n```\n",
	}).ByPath("adr/0001-a.md")

	if d.Sections("Consequences")[0].Empty() {
		t.Error("a section holding a fenced block reads as empty")
	}
}

func TestAGlossaryEntryReadsAnUnterminatedCommentAsNothing(t *testing.T) {
	// entryBlocks asked the same weak pattern Section.Empty did, so the
	// unterminated comment counted as a second paragraph and L15 reported a
	// fault that was not there.
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\n---\n\n# Glossary\n\n" +
			"## Widget\n\nA small thing.\n\n<!-- TODO: expand this\n",
	})

	entries, ok := r.Glossary()
	if !ok || len(entries) != 1 {
		t.Fatalf("want one entry, got %d (ok=%v)", len(entries), ok)
	}
	if n := len(entries[0].Paragraphs); n != 1 {
		t.Errorf("paragraphs = %d, want 1: %q", n, entries[0].Paragraphs)
	}
}

func TestADoubleBacktickSpanHoldsItsInnerBackticks(t *testing.T) {
	// A code span closes on a run of exactly the length that opened it. The old
	// pattern, `+[^`]*`+, accepted a run of any length and so cut a
	// double-backtick span short at its first inner backtick, letting whatever
	// followed escape the span.
	got := wikiNames(t, "A `` `[[RFC-0002]]` `` span, then [[RFC-0003]].\n")

	if want := []string{"RFC-0003"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q: the link inside the span escaped it", got, want)
	}
}

func TestAnUnmatchedBacktickRunIsLiteral(t *testing.T) {
	// With no closing run of the same length, the backticks are text, and the
	// link after them is prose rather than code.
	got := wikiNames(t, "An `` unmatched run, then [[RFC-0002]].\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q", got, want)
	}
}

func TestAShorterSpanOpensAfterAnUnmatchedRun(t *testing.T) {
	// The unmatched run is literal, so scanning resumes inside it rather than
	// swallowing the rest of the line.
	got := wikiNames(t, "`` then `[[RFC-0002]]` and [[RFC-0003]].\n")

	if want := []string{"RFC-0003"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q", got, want)
	}
}

func TestAHeadingKeepsADoubleBacktickSpan(t *testing.T) {
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n## The `` `<!--` `` marker\n",
	}).ByPath("rfc/0001-a.md")

	if got, want := d.SectionTitles(), []string{"The `` `<!--` `` marker"}; !slices.Equal(got, want) {
		t.Errorf("SectionTitles = %q, want %q", got, want)
	}
}

func TestAClosingFenceMayNotCarryAnInfoString(t *testing.T) {
	// CommonMark permits an info string on the opening fence only, so a line
	// reading "``` inside" within a ```text block is content. Treating it as a
	// closer ended the block early and made the real closer open a new one,
	// which swallowed everything after it.
	got := wikiNames(t, "```text\n``` inside\n```\n\nLive [[RFC-0002]] here.\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q: the fence closed on a line carrying an info string", got, want)
	}
}

func TestAClosingFenceMayBeLongerThanItsOpener(t *testing.T) {
	// Length at least as long still closes, which is the rule that lets a
	// shorter example nest inside a longer block.
	got := wikiNames(t, "```\nhidden [[RFC-0009]]\n`````\n\nLive [[RFC-0002]] here.\n")

	if want := []string{"RFC-0002"}; !slices.Equal(got, want) {
		t.Errorf("WikiLinks = %q, want %q", got, want)
	}
}

func TestImplementedNeedsBothAcceptanceAndInclusion(t *testing.T) {
	// Both conjuncts, asserted separately: no fixture was included-but-
	// unaccepted or accepted-but-unincluded, so the expression could be
	// rewritten as either half, or with || in place of &&, and stay green.
	r := repotest.New(t, map[string]string{
		"rfc/0001-both.md":       "---\nid: RFC-0001\ntitle: Both\nstatus: accepted\n---\n\n# RFC-0001: Both\n",
		"rfc/0002-unaccepted.md": "---\nid: RFC-0002\ntitle: Unaccepted\nstatus: draft\n---\n\n# RFC-0002: Unaccepted\n",
		"rfc/0003-unincluded.md": "---\nid: RFC-0003\ntitle: Unincluded\nstatus: accepted\n---\n\n# RFC-0003: Unincluded\n",
		"spec/page.md":           "---\ntitle: Page\nincludes: [RFC-0001, RFC-0002]\n---\n\n# Page\n\nProse.\n",
	})

	for _, want := range []struct {
		path        string
		implemented bool
		why         string
	}{
		{"rfc/0001-both.md", true, "accepted and included"},
		{"rfc/0002-unaccepted.md", false, "included but not accepted"},
		{"rfc/0003-unincluded.md", false, "accepted but not included"},
	} {
		if got := repotest.Document(t, r, want.path).Implemented; got != want.implemented {
			t.Errorf("%s: Implemented = %v, want %v (%s)", want.path, got, want.implemented, want.why)
		}
	}
}

func TestADateWrittenAsAYAMLAliasIsRead(t *testing.T) {
	// The date closure read n.Value directly where the string and list closures
	// decode through n.Decode. For an alias node Value is the anchor's name, so
	// "decided: *d" reported `"d" is not an ISO 8601 date` and named something
	// the author never wrote.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\nstatus: accepted\n" +
			"created: &d 2026-01-05\ndecided: *d\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# RFC-0001: A\n",
	}).ByPath("rfc/0001-a.md")

	for _, p := range d.Problems {
		t.Errorf("parsing reported %s:%d: %s", d.Path, p.Line, p.Message)
	}
	if got := d.FrontMatter.Decided.Format(repo.DateLayout); got != "2026-01-05" {
		t.Errorf("decided = %q, want the date the anchor holds", got)
	}
}

func TestASectionIsEmptyWhenItsCommentOpensOnTheHeadingLine(t *testing.T) {
	// Section.Text was sliced from the raw body and then scanned from a clean
	// state, so a comment opened on the heading line and closed inside the
	// section was invisible to the section-level check: the walk that had
	// already masked it was thrown away and the stray "-->" left behind read as
	// content. scanBody was the single definition of a comment but not the
	// single walk.
	d := repotest.New(t, map[string]string{
		"adr/0001-a.md": "---\nid: ADR-0001\ntitle: A\n---\n\n# ADR-0001: A\n\n" +
			"## Consequences <!-- I will fill this in later\n\n-->\n",
	}).ByPath("adr/0001-a.md")

	sections := d.Sections("Consequences")
	if len(sections) != 1 {
		t.Fatalf("want one Consequences section, got %d", len(sections))
	}
	if !sections[0].Empty() {
		t.Errorf("the section holds only a comment but does not read as empty; its text is %q", sections[0].Text)
	}
}

func TestASectionKeepsItsRawTextForRewriting(t *testing.T) {
	// Empty() reads the masked form; everything that rewrites a section needs
	// the bytes as written, comments and all.
	d := repotest.New(t, map[string]string{
		"adr/0001-a.md": "---\nid: ADR-0001\ntitle: A\n---\n\n# ADR-0001: A\n\n" +
			"## Consequences\n\nReal. <!-- a note -->\n",
	}).ByPath("adr/0001-a.md")

	if text := d.Sections("Consequences")[0].Text; !strings.Contains(text, "<!-- a note -->") {
		t.Errorf("Text = %q, want the comment kept as written", text)
	}
}

func TestWriteFileRefusesASymbolicLink(t *testing.T) {
	// The guard had no test at all: replacing its condition with a constant
	// false left all eleven packages green, and os.Lstat appears exactly once
	// in the codebase, at the guard.
	dir := t.TempDir()
	outside := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(outside, []byte("PRECIOUS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "INDEX.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := repo.WriteFile(link, []byte("generated\n"), "INDEX.md"); err == nil {
		t.Error("WriteFile reported success through a symbolic link")
	}
	if got, _ := os.ReadFile(outside); string(got) != "PRECIOUS\n" {
		t.Errorf("the target was overwritten: %q", got)
	}
}

func TestWriteFileStillWritesAnOrdinaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "INDEX.md")

	if err := repo.WriteFile(path, []byte("generated\n"), "INDEX.md"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "generated\n" {
		t.Errorf("contents = %q", got)
	}
}

func TestASectionIncludesItsOwnSubsections(t *testing.T) {
	// sectionAt ends a section at the next heading of the same level or higher,
	// which is what lets an author structure Proposal with ### subsections.
	// Without it a document organised that way reads as empty and the accept
	// gate refuses it, with no way to satisfy the tool but to flatten it.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n# RFC-0001: A\n\n" +
			"## Proposal\n\n### The first part\n\nContent here.\n\n### The second part\n\nMore.\n\n## Changelog\n\n- x\n",
	}).ByPath("rfc/0001-a.md")

	proposal := d.Sections("Proposal")
	if len(proposal) != 1 {
		t.Fatalf("want one Proposal section, got %d", len(proposal))
	}
	if proposal[0].Empty() {
		t.Error("a section whose content sits under H3 subsections reads as empty")
	}
	if !strings.Contains(proposal[0].Text, "The second part") {
		t.Errorf("the section stopped at its first subsection:\n%s", proposal[0].Text)
	}
}

func TestH1MustActuallyBeAnH1(t *testing.T) {
	// The text comparison cannot enforce this: a heading at any level carries
	// the same text. Without the level test, "### RFC-0001: A" satisfies L10
	// and the freeze gate, and the document freezes that way.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\n---\n\n### RFC-0001: A\n\n## Abstract\n\nProse.\n",
	}).ByPath("rfc/0001-a.md")

	if _, ok := d.H1(); ok {
		t.Error("an H3 was accepted as the document's H1")
	}
}

func TestParsingReportsADuplicateFrontMatterKey(t *testing.T) {
	// yaml.v3 yields both pairs and the later value wins, so without this the
	// document lints clean while the value archdoc reads is not the one the
	// author would expect to matter.
	d := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\ncreated: 2026-01-01\ncreated: 2026-06-01\n---\n\n# RFC-0001: A\n",
	}).ByPath("rfc/0001-a.md")

	var said bool
	for _, p := range d.Problems {
		if strings.Contains(p.Message, "created") {
			said = true
		}
	}
	if !said {
		t.Errorf("a repeated key was not reported; problems were %+v", d.Problems)
	}
}

func TestWriteFileRefusesASymlinkedDirectory(t *testing.T) {
	// The file case was refused; the directory case was not. A document
	// directory pointing elsewhere passes the root check, because the root
	// itself is real, and every write into it then lands outside the
	// repository while the tool reports a path inside it.
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "specdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "specdir"), filepath.Join(dir, "spec")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := repo.Contains(dir, filepath.Join(dir, "spec", "glossary.md")); err == nil {
		t.Error("a write into a symlinked document directory was allowed")
	}
	if err := repo.Contains(dir, filepath.Join(dir, "rfc", "0001-a.md")); err != nil {
		t.Errorf("an ordinary path was refused: %v", err)
	}
}

// TestTheGlossaryDoesNotImplementAnything separates terminology from
// implementation.
//
// ArchDoc's own glossary template asks for the RFC that named a term to go in
// includes, so the tool instructs the inclusion that then reported the RFC as
// built. PROCESS.md is explicit that this list records implementation: an
// accepted RFC included by no spec page is not yet implemented. Defining a word
// is not describing behaviour.
func TestTheGlossaryDoesNotImplementAnything(t *testing.T) {
	const accepted = "---\nid: %s\ntitle: %s\nstatus: accepted\ncreated: 2026-01-01\n" +
		"decided: 2026-01-02\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# %s: %s\n\n## Abstract\n\nWords.\n"
	r := repotest.New(t, map[string]string{
		"rfc/0001-named.md": fmt.Sprintf(accepted, "RFC-0001", "Named", "RFC-0001", "Named"),
		"rfc/0002-built.md": fmt.Sprintf(accepted, "RFC-0002", "Built", "RFC-0002", "Built"),
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: [RFC-0001]\n---\n\n# Glossary\n\n" +
			"## Binding\n\nA registered resolution.\n",
		"spec/container.md": "---\ntitle: Container\nincludes: [RFC-0002]\n---\n\n# Container\n\nWhat it does.\n",
	})

	named := r.ByID("RFC-0001")
	if named.Implemented {
		t.Error("an RFC included only by the glossary is reported as implemented")
	}
	// The relationship itself is still a fact worth keeping: the export answers
	// "which spec pages reference this", and the glossary does.
	if got := named.IncludedIn; len(got) != 1 || got[0] != "glossary" {
		t.Errorf("included_in = %v, want [glossary]; the inclusion still happened", got)
	}

	if built := r.ByID("RFC-0002"); !built.Implemented {
		t.Error("an RFC included by an ordinary spec page is not reported as implemented")
	}
}
