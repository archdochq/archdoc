package link_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ollieread/archdoc/internal/link"
	"github.com/ollieread/archdoc/internal/lint"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/repotest"
)

var now = time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

func openLinks(t *testing.T) *repo.Repo {
	t.Helper()
	return repotest.Fixture(t, "links")
}

func context(t *testing.T) lint.Context {
	t.Helper()
	return lint.NewContext(openLinks(t), nil, now)
}

func TestResolveExpandsAnIdentifierWithItsTitle(t *testing.T) {
	changes, findings := link.Resolve(context(t))
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %v", findings)
	}

	got := string(changes["rfc/0002-scheduling.md"])
	if want := "[RFC-0001: Job queues](0001-queues.md)"; !strings.Contains(got, want) {
		t.Errorf("want %q in the rewritten document:\n%s", want, got)
	}
}

func TestResolveLinksAGlossaryTermPreservingTheCaseAsWritten(t *testing.T) {
	changes, _ := link.Resolve(context(t))

	got := string(changes["rfc/0002-scheduling.md"])
	if want := "[spec page](../spec/glossary.md#spec-page)"; !strings.Contains(got, want) {
		t.Errorf("want %q in the rewritten document:\n%s", want, got)
	}
}

func TestResolveNeverTouchesCodeOrFences(t *testing.T) {
	changes, _ := link.Resolve(context(t))

	got := string(changes["rfc/0002-scheduling.md"])
	if !strings.Contains(got, "`[[RFC-0001]]` in code") {
		t.Errorf("a wiki link inside a code span was expanded:\n%s", got)
	}
	if !strings.Contains(got, "```\n[[RFC-0001]]\n```") {
		t.Errorf("a wiki link inside a fence was expanded:\n%s", got)
	}
}

func TestResolveReportsALinkThatMatchesNothing(t *testing.T) {
	r := openLinks(t)
	for _, d := range r.Documents() {
		if d.Path == "spec/queues.md" {
			d.Body = strings.Replace(d.Body, "[[Wings]]", "[[Nothing At All]]", 1)
			d.Source = []byte(strings.Replace(string(d.Source), "[[Wings]]", "[[Nothing At All]]", 1))
		}
	}

	changes, findings := link.Resolve(lint.NewContext(r, nil, now))
	if _, changed := changes["spec/queues.md"]; changed {
		t.Error("a document with an unresolvable link was rewritten anyway")
	}
	var reported bool
	for _, f := range findings {
		if strings.Contains(f.Message, "Nothing At All") && f.Severity == lint.Error {
			reported = true
		}
	}
	if !reported {
		t.Errorf("the unresolvable link was not reported: %v", findings)
	}
}

func TestSuggestWrapsTextWithoutChangingIt(t *testing.T) {
	suggestions := link.Suggest(openLinks(t))

	var found *link.Suggestion
	for i, s := range suggestions {
		if s.Path == "rfc/0002-scheduling.md" && s.Text == "RFC-0001" {
			found = &suggestions[i]
		}
	}
	if found == nil {
		t.Fatalf("no suggestion for the bare RFC-0001 mention; got %+v", suggestions)
	}
	if want := "[RFC-0001](0001-queues.md)"; found.Replacement != want {
		t.Errorf("Replacement = %q, want %q: a suggestion wraps prose, it does not retitle it",
			found.Replacement, want)
	}
}

func TestSuggestExcludesTheDocumentsOwnIdentifierAndCode(t *testing.T) {
	suggestions := link.Suggest(openLinks(t))
	if len(suggestions) == 0 {
		t.Fatal("no suggestions at all, so this asserts nothing")
	}
	for _, s := range suggestions {
		if s.Path == "rfc/0002-scheduling.md" && s.Text == "RFC-0002" {
			t.Errorf("suggested linking a document to itself: %+v", s)
		}
		if strings.Contains(s.LineText, "```") {
			t.Errorf("suggested a link inside a fence: %+v", s)
		}
	}
}

func TestSuggestOffersEachTargetOncePerDocument(t *testing.T) {
	suggestions := link.Suggest(openLinks(t))
	if len(suggestions) == 0 {
		t.Fatal("no suggestions at all, so this asserts nothing")
	}
	seen := map[string]int{}
	for _, s := range suggestions {
		seen[s.Path+" "+s.Text]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Errorf("%s suggested %d times, want once", key, count)
		}
	}
}

func TestSuggestSkipsTextThatIsAlreadyLinked(t *testing.T) {
	suggestions := link.Suggest(openLinks(t))
	if len(suggestions) == 0 {
		t.Fatal("no suggestions at all, so this asserts nothing")
	}
	for _, s := range suggestions {
		if strings.Contains(s.LineText, "Already linked") || strings.Contains(s.LineText, "already a link") {
			t.Errorf("suggested wrapping text that is already a link, which nests them: %+v", s)
		}
	}
}

func TestSuggestOffersOncePerTargetWhateverTheSpelling(t *testing.T) {
	// Wings, wings and WINGS all name the same glossary entry.
	count := 0
	for _, s := range link.Suggest(openLinks(t)) {
		if s.Path == "rfc/0002-scheduling.md" && strings.EqualFold(s.Text, "wings") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the Wings entry was suggested %d times, want once", count)
	}
}

func TestApplyRewritesTheMatchedOccurrenceNotTheFirst(t *testing.T) {
	r := openLinks(t)
	var suggestion link.Suggestion
	for _, s := range link.Suggest(r) {
		if strings.Contains(s.LineText, "in code comes before") {
			suggestion = s
		}
	}
	if suggestion.Text == "" {
		t.Fatal("no suggestion on the line where a code span precedes the prose mention")
	}

	out := link.Apply("spec/offsets.md", repotest.Document(t, r, "spec/offsets.md").Source, []link.Suggestion{suggestion})
	if strings.Contains(string(out), "`[RFC-0001](") {
		t.Errorf("the link was written inside the code span:\n%s", excerpt(string(out), "in code comes before"))
	}
	if !strings.Contains(string(out), "a real [RFC-0001](../rfc/0001-queues.md) mention") {
		t.Errorf("the prose occurrence was not linked:\n%s", excerpt(string(out), "in code comes before"))
	}
}

func TestApplyIgnoresSuggestionsForOtherPathsAndStaleLines(t *testing.T) {
	r := openLinks(t)
	source := repotest.Document(t, r, "rfc/0002-scheduling.md").Source

	other := link.Suggestion{Path: "spec/queues.md", Line: 1, Text: "x", Replacement: "y", LineText: "x"}
	stale := link.Suggestion{Path: "rfc/0002-scheduling.md", Line: 3, Text: "title", Col: 0, Replacement: "WRONG", LineText: "a line that has since changed"}

	out := link.Apply("rfc/0002-scheduling.md", source, []link.Suggestion{other, stale})
	if string(out) != string(source) {
		t.Errorf("Apply rewrote something it should have skipped:\n%s", out)
	}
}

func TestResolveExpandsALinkOnALineThatAlsoHoldsCode(t *testing.T) {
	changes, findings := link.Resolve(context(t))
	got := string(changes["rfc/0002-scheduling.md"])

	if strings.Contains(got, "[[RFC-0001]] jobs") {
		t.Errorf("a wiki link on a line holding a code span was left unexpanded, yet the document was reported rewritten; findings: %v", findings)
	}
	if !strings.Contains(got, "`queue` runs [RFC-0001: Job queues](0001-queues.md) jobs") {
		t.Errorf("the code span or the link was mangled:\n%s", excerpt(got, "runs"))
	}
}

// excerpt returns the line containing needle, for readable failures.
func excerpt(haystack, needle string) string {
	for _, line := range strings.Split(haystack, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return haystack
}

func TestSuggestNeverOffersInsideAWikiLink(t *testing.T) {
	// A suggestion applied inside [[...]] produces markup that no longer
	// matches the wiki-link pattern, so the error it was reporting disappears.
	for _, s := range link.Suggest(openLinks(t)) {
		if strings.Contains(s.LineText[:min(s.Col, len(s.LineText))], "[[") &&
			!strings.Contains(s.LineText[:min(s.Col, len(s.LineText))], "]]") {
			t.Errorf("suggested a link inside an unresolved wiki link: %+v", s)
		}
	}
}

func TestSuggestNeverOffersInsideAReferenceLink(t *testing.T) {
	r := openLinks(t)
	for _, s := range link.Suggest(r) {
		if strings.Contains(s.LineText, "][") && strings.Contains(s.LineText, s.Text+"][") {
			t.Errorf("suggested a link over reference-link text: %+v", s)
		}
	}
}

func TestApplyRewritesEverySuggestionOnALine(t *testing.T) {
	const line = "Both ADR-0001 and ADR-0002 are relevant here."
	source := []byte("---\ntitle: T\nincludes: []\n---\n\n# T\n\n" + line + "\n")

	first := strings.Index(line, "ADR-0001")
	second := strings.Index(line, "ADR-0002")
	accepted := []link.Suggestion{
		{Path: "spec/t.md", Line: 8, Text: "ADR-0001", Col: first, Replacement: "[ADR-0001](a.md)", LineText: line},
		{Path: "spec/t.md", Line: 8, Text: "ADR-0002", Col: second, Replacement: "[ADR-0002](b.md)", LineText: line},
	}

	out := string(link.Apply("spec/t.md", source, accepted))
	for _, want := range []string{"[ADR-0001](a.md)", "[ADR-0002](b.md)"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q was dropped; --apply must apply all:\n%s", want, out)
		}
	}
}

func TestResolveDoesNotShiftADocumentWithNoFrontMatter(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Wings\n\nThe daemon.\n",
		"spec/orphan.md":   "# Orphan\n\nSee [[Wings]] for detail.\n",
	})

	changes, _ := link.Resolve(lint.NewContext(r, nil, now))
	got, rewritten := changes["spec/orphan.md"]
	if !rewritten {
		t.Skip("nothing to check: the document was not rewritten")
	}
	if strings.HasPrefix(string(got), "\n") {
		t.Errorf("a blank line was prepended to a document with no front matter:\n%q", got)
	}
}

// commented builds two documents, the second holding a live wiki link, the same
// link inside a one-line comment, and the same link inside a comment spanning
// several lines.
func commented(t *testing.T) *repo.Repo {
	t.Helper()
	return repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: Alpha\nstatus: draft\n---\n\n# RFC-0001: Alpha\n\n## Abstract\n\nA.\n",
		"rfc/0002-b.md": "---\nid: RFC-0002\ntitle: Beta\nstatus: draft\n---\n\n# RFC-0002: Beta\n\n## Abstract\n\n" +
			"Live [[RFC-0001]] here.\n\n" +
			"<!-- one line [[RFC-0001]] and a bare RFC-0001 -->\n\n" +
			"<!--\nacross lines [[RFC-0001]] and a bare RFC-0001\n-->\n",
	})
}

func TestResolveNeverTouchesACommentedLink(t *testing.T) {
	// A comment is nothing whether it spans one line or several. The two used
	// to differ: the one-line form was rewritten, the other was not.
	changes, findings := link.Resolve(lint.NewContext(commented(t), nil, now))
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %v", findings)
	}

	got := string(changes["rfc/0002-b.md"])
	if !strings.Contains(got, "Live [RFC-0001: Alpha](0001-a.md) here.") {
		t.Errorf("the live link was not resolved:\n%s", got)
	}
	for _, want := range []string{
		"<!-- one line [[RFC-0001]] and a bare RFC-0001 -->",
		"across lines [[RFC-0001]] and a bare RFC-0001",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("a commented link was rewritten; %q is gone:\n%s", want, got)
		}
	}
}

func TestSuggestOffersNothingInsideAComment(t *testing.T) {
	// Every bare identifier in the document is inside a comment, and the one
	// live mention is already a wiki link, so there is nothing to offer at all.
	// Asserted as a count rather than by inspecting each suggestion's text: the
	// line inside a multi-line comment does not itself contain <!--, so a test
	// looking for that marker would not have caught a regression there.
	if got := link.Suggest(commented(t)); len(got) != 0 {
		t.Errorf("Suggest offered %d suggestions, want none: %+v", len(got), got)
	}
}

// suggestable builds a repository with a glossary and one document whose body
// is given, so a test can aim a single line at Suggest.
func suggestable(t *testing.T, terms map[string]string, body string) *repo.Repo {
	t.Helper()
	page := "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n"
	for _, term := range slices.Sorted(maps.Keys(terms)) {
		page += "\n## " + term + "\n\n" + terms[term] + "\n"
	}
	return repotest.New(t, map[string]string{
		"spec/glossary.md": page,
		"spec/page.md":     "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n" + body,
	})
}

func suggestionsFor(t *testing.T, r *repo.Repo) []string {
	t.Helper()
	var out []string
	for _, s := range link.Suggest(r) {
		out = append(out, fmt.Sprintf("%s:%d %q", s.Path, s.Line, s.Text))
	}
	return out
}

func TestSuggestLeavesBareURLsAlone(t *testing.T) {
	// A URL is not prose. Splicing a link into the middle of one destroys it,
	// and nothing afterwards can tell: lint only parses markdown links, so a
	// bare URL is not a link to any rule.
	r := suggestable(t, map[string]string{"Widget": "A small thing."},
		"See https://example.com/docs/widget/overview for more.\n")

	if got := suggestionsFor(t, r); len(got) != 0 {
		t.Errorf("Suggest offered %v inside a URL", got)
	}
}

func TestSuggestLeavesLinkReferenceDefinitionsAlone(t *testing.T) {
	// The glossary term is the definition's LABEL, not part of its destination.
	// With the term only in the destination this test passed on the bare-URL
	// mask alone, so deleting the reference-definition mask entirely left the
	// suite green and the guard it is named for was never exercised.
	for _, body := range []string{
		"[Widget]: https://example.com/guide\n\nSee [the guide][Widget].\n",
		"[Widget]: ./other.md\n\nSee [the guide][Widget].\n",
		"   [Widget]: https://example.com/guide \"A title\"\n\nSee [the guide][Widget].\n",
	} {
		r := suggestable(t, map[string]string{"Widget": "A small thing."}, body)
		if got := suggestionsFor(t, r); len(got) != 0 {
			t.Errorf("Suggest offered %v inside %q", got, body)
		}
	}
}

func TestSuggestMatchesATermWhoseEdgesAreNotWordCharacters(t *testing.T) {
	// Go's \b is an ASCII word boundary, so for a term ending in "+" the
	// assertion inverted: it failed where the term stood alone and succeeded
	// where it was glued to a digit.
	r := suggestable(t, map[string]string{"C++": "A language."},
		"We write C++ every day and the C++11 standard applies.\n")

	assertStandalone(t, link.Suggest(r), "C++", "We write ")
}

// assertStandalone fails unless there is exactly one suggestion, it matched the
// given text, and the text is glued to no letter or digit on either side. The
// position matters and the text alone does not show it: "C++" is the matched
// text whether the match was the standalone occurrence or the one inside
// "C++11", so an assertion on the text passes either way.
func assertStandalone(t *testing.T, got []link.Suggestion, text, precededBy string) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("want one suggestion for the standalone %q, got %d: %+v", text, len(got), got)
	}
	s := got[0]
	if s.Text != text {
		t.Fatalf("matched %q, want %q", s.Text, text)
	}
	if before := s.LineText[:s.Col]; before != precededBy {
		t.Errorf("matched the occurrence after %q, want the one after %q", before, precededBy)
	}
	after := s.LineText[s.Col+len(s.Text):]
	if r, size := utf8.DecodeRuneInString(after); size > 0 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
		t.Errorf("matched an occurrence glued to %q", after)
	}
}

func TestSuggestDoesNotMatchInsideALongerWord(t *testing.T) {
	r := suggestable(t, map[string]string{"Café": "A place."},
		"Several Cafés exist, but we visit the Café often.\n")

	assertStandalone(t, link.Suggest(r), "Café", "Several Cafés exist, but we visit the ")
}

func TestSuggestLeavesHeadingsAlone(t *testing.T) {
	// A heading's raw text is a section's identity everywhere else: L08, L09,
	// L14 and the freeze gate all compare it with ==. Wrapping one in a link
	// deletes the section from every rule at once.
	r := suggestable(t, map[string]string{"Decision": "A choice made."},
		"## Decision\n\nWe chose the thing.\n")

	if got := suggestionsFor(t, r); len(got) != 0 {
		t.Errorf("Suggest offered %v over a heading", got)
	}
}

func TestResolveEscapesAGlossaryTermUsedAsALabel(t *testing.T) {
	// The identifier branch escapes its title; the term branch did not, so a
	// term carrying a bracket reopened an injection class already closed once.
	// A pipe, not a bracket: a term carrying a bracket can no longer reach a
	// label at all, because the line holding it has unbalanced brackets and the
	// masker withholds the whole line. A pipe still reaches one, and it is what
	// ends a table cell, so the escaping is what keeps INDEX.md intact.
	r := suggestable(t, map[string]string{`Zed|x`: "The odd one."},
		"The Zed|x matters.\n")

	suggestions := link.Suggest(r)
	if len(suggestions) != 1 {
		t.Fatalf("want one suggestion, got %d: %+v", len(suggestions), suggestions)
	}
	if !strings.HasPrefix(suggestions[0].Replacement, `[Zed\|x](`) {
		t.Errorf("the label was not escaped: %s", suggestions[0].Replacement)
	}
}

// countingGit records how often the branch is consulted, so a test can assert
// that work nobody asked for is not done.
type countingGit struct{ listed, read int }

func (g *countingGit) FileAt(string, string) ([]byte, bool, error) { g.read++; return nil, false, nil }
func (g *countingGit) ListFiles(string) ([]string, error)          { g.listed++; return nil, nil }
func (g *countingGit) BranchExists(string) (bool, error)           { return true, nil }

func (g *countingGit) HasCommits() (bool, error) { return true, nil }

func (g *countingGit) Changed(string, []string) (map[string]bool, error) { return nil, nil }

func (g *countingGit) FilesAt(_ string, p []string) (map[string][]byte, error) {
	g.read += len(p)
	return nil, nil
}
func (g *countingGit) CurrentBranch() (string, error) { return "main", nil }
func (g *countingGit) RepoRoot() (string, error)      { return "/", nil }

func TestResolveDoesNotBuildABranchSnapshotItNeverReads(t *testing.T) {
	// Resolve consults the branch only to decide whether a document that
	// contains a wiki link is frozen. When none does, the answer is never
	// wanted, and building it costs a git subprocess per document.
	g := &countingGit{}
	link.Resolve(lint.NewContext(openLinks(t), g, now))

	if g.listed != 0 || g.read != 0 {
		t.Errorf("the branch was read %d times and listed %d times, want none of either",
			g.read, g.listed)
	}
}

func TestTheSnapshotIsStillBuiltWhenItIsNeeded(t *testing.T) {
	// The other side: a lazily built snapshot that is never built is just a bug
	// with better timing.
	g := &countingGit{}
	ctx := lint.NewContext(openLinks(t), g, now)

	if _, ok := ctx.OnBranch("rfc/0001-scheduling.md"); ok {
		t.Error("the stub reports nothing on the branch, so ok should be false")
	}
	if g.listed == 0 {
		t.Error("asking about the branch did not build the snapshot")
	}
}

func TestResolveLeavesHeadingsAlone(t *testing.T) {
	// L10 compares the H1's raw text against the front matter, and L08, L09 and
	// the freeze gate compare an H2's against the required list. Expanding a
	// wiki link inside one changes the text all four read.
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: Alpha\nstatus: draft\n---\n\n# RFC-0001: Alpha\n\n## Abstract\n\nA.\n",
		"rfc/0002-b.md": "---\nid: RFC-0002\ntitle: Beta\nstatus: draft\n---\n\n# RFC-0002: Beta\n\n" +
			"## See [[RFC-0001]]\n\nLive [[RFC-0001]] here.\n",
	})

	changes, _ := link.Resolve(lint.NewContext(r, nil, now))
	got := string(changes["rfc/0002-b.md"])

	if !strings.Contains(got, "## See [[RFC-0001]]") {
		t.Errorf("the heading was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "Live [RFC-0001: Alpha](0001-a.md) here.") {
		t.Errorf("the prose link was not resolved:\n%s", got)
	}
}

func TestSuggestLeavesEveryBracketedLinkFormAlone(t *testing.T) {
	// maskLinks blanks spans a suggestion must never be offered inside, and it
	// works one line at a time. Three forms escaped it, and --apply then spliced
	// a link inside a link, which CommonMark forbids: the author's link is
	// demoted to literal text and its destination is printed as prose.
	for _, name := range []string{"wrapped label", "shortcut reference", "definition title"} {
		var body string
		switch name {
		case "wrapped label":
			// Ordinary wrapped prose, which this project itself does.
			body = "The analysis in the [Widget\ncapacity review](../ref/0001-notes.md) settles it.\n"
		case "shortcut reference":
			body = "The [Widget] thread settled this.\n\n[Widget]: https://example.com/discussion\n"
		case "definition title":
			body = "[Widget]: https://example.com/guide (The Widget guide)\n\nSee [it][Widget].\n"
		}
		r := suggestable(t, map[string]string{"Widget": "A small thing."}, body)

		if got := suggestionsFor(t, r); len(got) != 0 {
			t.Errorf("%s: Suggest offered %v", name, got)
		}
	}
}

func TestSuggestStillOffersOrdinaryProse(t *testing.T) {
	// The masking is deliberately generous, so this pins that it has not become
	// generous enough to stop suggesting anything at all.
	r := suggestable(t, map[string]string{"Widget": "A small thing."},
		"The widget is the unit of work here.\n")

	if got := suggestionsFor(t, r); len(got) != 1 {
		t.Errorf("Suggest offered %d suggestions on ordinary prose, want 1: %v", len(got), got)
	}
}

// frozenRepo is a repository whose one RFC is accepted and on the branch, with
// a wiki link left in it.
func frozenRepo(t *testing.T) (*repo.Repo, *countingGit) {
	t.Helper()
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: Alpha\nstatus: accepted\n---\n\n# RFC-0001: Alpha\n\n" +
			"## Abstract\n\nSee [[RFC-0002]] and the widget.\n",
		"rfc/0002-b.md":    "---\nid: RFC-0002\ntitle: Beta\nstatus: accepted\n---\n\n# RFC-0002: Beta\n\n## Abstract\n\nB.\n",
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Widget\n\nA small thing.\n",
	})
	return r, &countingGit{}
}

func TestResolveNeverRewritesAFrozenDocument(t *testing.T) {
	// editable() is the only thing stopping Resolve from rewriting a document
	// L11 forbids anyone modifying. If it went, `archdoc link` would edit it and
	// the repository would fail lint permanently until someone hand-reverted.
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: Alpha\nstatus: accepted\n---\n\n# RFC-0001: Alpha\n\n" +
			"## Abstract\n\nSee [[RFC-0002]].\n",
		"rfc/0002-b.md": "---\nid: RFC-0002\ntitle: Beta\nstatus: accepted\n---\n\n# RFC-0002: Beta\n\n## Abstract\n\nB.\n",
	})

	changes, findings := link.Resolve(lint.NewContext(r, nil, now))
	if _, rewritten := changes["rfc/0001-a.md"]; rewritten {
		t.Error("Resolve rewrote a document whose status is terminal")
	}
	if len(findings) == 0 {
		t.Error("Resolve rewrote nothing and reported nothing, so the link is simply lost")
	}
}

func TestSuggestSkipsATerminalDocument(t *testing.T) {
	r, _ := frozenRepo(t)

	for _, s := range link.Suggest(r) {
		if s.Path == "rfc/0001-a.md" {
			t.Errorf("a suggestion was offered in a document whose status is terminal: %+v", s)
		}
	}
}

func TestSuggestSkipsTheGlossaryPageItself(t *testing.T) {
	// Its own half, with a glossary whose entries mention each other: the
	// previous version of this test shared one condition with the terminal
	// check above, and its glossary fixture defined a single term that appeared
	// nowhere in its own prose, so no suggestion could ever be offered there
	// and this half was pinned by nothing.
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n" +
			"## Daemon\n\nThe process a widget runs inside.\n\n" +
			"## Widget\n\nThe unit of work a daemon carries.\n",
	})

	for _, s := range link.Suggest(r) {
		if s.Path == "spec/glossary.md" {
			t.Errorf("a suggestion was offered on the glossary page itself: %+v", s)
		}
	}
}

func TestResolveEscapesATitleBeforeExpandingItIntoALink(t *testing.T) {
	// The expansion writes the target's title into another document as a link
	// label. Unescaped, a title carrying markdown becomes live markup in a file
	// its author never touched.
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: 'Pwned](https://evil.example) x'\nstatus: draft\n---\n\n" +
			"# RFC-0001: Pwned](https://evil.example) x\n\n## Abstract\n\nA.\n",
		"rfc/0002-b.md": "---\nid: RFC-0002\ntitle: Beta\nstatus: draft\n---\n\n# RFC-0002: Beta\n\n## Abstract\n\nSee [[RFC-0001]].\n",
	})

	changes, _ := link.Resolve(lint.NewContext(r, nil, now))
	got := string(changes["rfc/0002-b.md"])
	if strings.Contains(strings.ReplaceAll(got, `\]`, ""), "](https://evil.example)") {
		t.Errorf("the expanded title rendered as a live link:\n%s", got)
	}
}

func TestSuggestDoesNotMatchAtTheEndOfALongerWord(t *testing.T) {
	// Only the right-hand boundary was pinned. Without the left-hand one a term
	// occurring at the end of a longer word is offered, and --apply rewrites
	// prose the author wrote into markup that means something else.
	r := suggestable(t, map[string]string{"Widget": "A small thing."},
		"The subwidget is not the thing.\n")

	if got := suggestionsFor(t, r); len(got) != 0 {
		t.Errorf("Suggest offered %v inside a longer word", got)
	}
}

func TestSuggestPrefersTheStandaloneOccurrenceOverAGluedOne(t *testing.T) {
	// The left-hand boundary only matters when the term's first word is a word
	// of the line, so the glued occurrence is reached at all. Without it the
	// first match wins and --apply rewrites the inside of a longer word.
	r := suggestable(t, map[string]string{"Widget": "A small thing."},
		"The subwidget and the widget are different.\n")

	assertStandalone(t, link.Suggest(r), "widget", "The subwidget and the ")
}

func TestSuggestLeavesRawHTMLAlone(t *testing.T) {
	// Markdown does not re-parse the contents of an HTML block, so a link
	// spliced into one renders as its literal source characters, and a link
	// spliced into an attribute breaks the attribute. Nothing afterwards sees
	// it: the inserted link resolves, so L17 passes and lint exits 0.
	for _, body := range []string{
		`<img src="a.png" alt="Widget diagram">` + "\n",
		"<details>\n<summary>Widget configuration</summary>\n\nBody.\n\n</details>\n",
		"<table>\n  <tr><td>Widget</td></tr>\n</table>\n",
		`<a href="/widget">Widget</a>` + "\n",
	} {
		r := suggestable(t, map[string]string{"Widget": "A small thing."}, body)
		if got := suggestionsFor(t, r); len(got) != 0 {
			t.Errorf("Suggest offered %v inside raw HTML:\n%s", got, body)
		}
	}
}

func TestSuggestStillOffersProseBetweenHTMLBlocks(t *testing.T) {
	// A blank line ends an HTML block, so what follows is ordinary markdown and
	// a suggestion there is correct. The masking must not swallow it.
	r := suggestable(t, map[string]string{"Widget": "A small thing."},
		"<div>\n\nThe widget is the unit of work.\n\n</div>\n")

	if got := suggestionsFor(t, r); len(got) != 1 {
		t.Errorf("Suggest offered %d suggestions in prose between HTML blocks, want 1: %v", len(got), got)
	}
}

func TestSuggestIgnoresAGlossaryEntryWithNoTerm(t *testing.T) {
	// An empty H2 on the glossary page compiles to a pattern that matches the
	// empty string, so every prose line matched at offset 0 and --apply wrote
	// "[](glossary.md#)" into it, including into blank lines.
	r := suggestable(t, map[string]string{" ": "An entry with no term."},
		"Some ordinary prose here.\n")

	for _, s := range link.Suggest(r) {
		if strings.TrimSpace(s.Text) == "" {
			t.Errorf("a suggestion was offered for an empty term: %+v", s)
		}
	}
}

func TestSuggestLeavesAnInlineLinkDestinationAlone(t *testing.T) {
	// "bracketed" masks the label half of every link form, so the destination
	// of an inline link is covered only by MarkdownLinkPattern. The existing
	// test for this could not fail: its fixture spent the term earlier in the
	// document, and Suggest offers one suggestion per target.
	r := suggestable(t, map[string]string{"Widget": "A small thing."},
		"See [the guide](../spec/widget-notes.md) for more.\n")

	if got := suggestionsFor(t, r); len(got) != 0 {
		t.Errorf("Suggest offered %v inside an inline link's destination", got)
	}
}
