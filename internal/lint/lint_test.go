package lint_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/archdochq/archdoc/internal/lint"
	"github.com/archdochq/archdoc/internal/repo"
	"github.com/archdochq/archdoc/internal/repotest"
)

// now is a fixed date so that rules comparing against today are stable.
var now = time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

// runLint opens one of the fixture repositories and runs every rule over it.
func runLint(t *testing.T, fixture string) []lint.Finding {
	t.Helper()
	return lint.Run(lint.NewContext(repotest.Fixture(t, fixture), nil, now))
}

// forRule returns the findings a single rule produced, as "path:line: message".
func forRule(t *testing.T, fixture, code string) []string {
	t.Helper()
	var out []string
	for _, f := range runLint(t, fixture) {
		if f.Rule == code {
			out = append(out, fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Message))
		}
	}
	return out
}

// mentions fails unless some finding contains every given substring.
//
// Each substring is matched independently across the whole finding set, so a
// test asserting a path and a symptom together should use reports instead: two
// unrelated findings can satisfy it between them, which is how two of L15's
// three checks and L08's closing-section check all ended up unpinned.
func mentions(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.ContainsFunc(got, func(g string) bool { return strings.Contains(g, w) }) {
			t.Errorf("no finding mentions %q; got %q", w, got)
		}
	}
}

// reports fails unless ONE finding contains every given substring. This is what
// a test naming a document and a symptom together means.
func reports(t *testing.T, got []string, want ...string) {
	t.Helper()
	for _, g := range got {
		all := true
		for _, w := range want {
			if !strings.Contains(g, w) {
				all = false
				break
			}
		}
		if all {
			return
		}
	}
	t.Errorf("no single finding mentions all of %q; got %q", want, got)
}

// omits fails if any finding contains one of the given substrings.
func omits(t *testing.T, got []string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		for _, g := range got {
			if strings.Contains(g, u) {
				t.Errorf("a finding mentions %q, which it should not: %q", u, g)
			}
		}
	}
}

func TestL01ReportsSchemaFaults(t *testing.T) {
	got := forRule(t, "lint", "L01")

	mentions(t, got,
		"0001-bad-front-matter.md:9: unknown", // a key belonging to no schema, on its own line
		"owner",
		"obsoletes", // a required key that was never written
		"created",   // a value that is not an ISO 8601 date
	)
}

func TestL02ChecksIdentifierAgainstTypeAndFilename(t *testing.T) {
	got := forRule(t, "lint", "L02")

	mentions(t, got,
		"notes.md",         // filename carries no number
		"0002-wrong-id.md", // front matter claims RFC-0099
		"RFC-0099",
	)
}

func TestL03ReportsAnIdentifierUsedTwice(t *testing.T) {
	got := forRule(t, "lint", "L03")

	mentions(t, got, "ADR-0001", "0001-duplicate.md")
	if len(got) < 2 {
		t.Errorf("want a finding on each document sharing the identifier, got %q", got)
	}
}

func TestL04ReportsReferencesThatResolveToNothing(t *testing.T) {
	got := forRule(t, "lint", "L04")

	mentions(t, got,
		"RFC-9999", // no such document
		"REF-0001", // a ref is not permitted in depends
	)
}

func TestL05ChecksUpdatesAndObsoletes(t *testing.T) {
	got := forRule(t, "lint", "L05")

	mentions(t, got,
		"ADR-0001", // an RFC may not update an ADR
		"itself",   // RFC-0004 updates itself
		"both",     // RFC-0005 appears in updates and obsoletes
		"accepted", // RFC-0005 is a draft, so it may not be updated or obsoleted
	)
}

func TestL06RequiresIncludesToNameAcceptedDocuments(t *testing.T) {
	got := forRule(t, "lint", "L06")

	mentions(t, got, "includes-draft.md", "RFC-0005", "draft")
}

func TestL07ChecksDecidedAgainstStatusAndCreated(t *testing.T) {
	got := forRule(t, "lint", "L07")

	mentions(t, got,
		"0006-terminal-no-decided.md",    // terminal with no decided date
		"0008-draft-with-decided.md",     // decided set while not terminal
		"0007-decided-before-created.md", // decided precedes created
	)
}

func TestL08ChecksRequiredSections(t *testing.T) {
	got := forRule(t, "lint", "L08")

	mentions(t, got,
		"Motivation",          // missing entirely
		"Proposal",            // out of order
		"Rejection rationale", // present although not rejected
	)
}

func TestL08RequiresTheClosingSectionToBeLast(t *testing.T) {
	// Asserted as a whole message against one path, not through mentions: that
	// helper matches each wanted substring independently across every finding
	// in the run, so a bare "Sources" was already satisfied by a
	// missing-section finding on a different fixture. This rule had a fixture
	// written for it and no assertion that could fail, and deleting the rule
	// left the suite green.
	var got []string
	for _, f := range runLint(t, "lint") {
		if f.Rule == "L08" && f.Path == "ref/0002-sources-not-last.md" {
			got = append(got, f.Message)
		}
	}

	want := `"Sources" must be the last H2, but "Afterword" follows it`
	if !slices.Contains(got, want) {
		t.Errorf("L08 on the ref said %q, want one of them to be %q", got, want)
	}
}

func TestL09RequiresOpenQuestionsEmptyToAccept(t *testing.T) {
	got := forRule(t, "lint", "L09")

	mentions(t, got, "0010-open-questions.md", "Open questions")
	omits(t, got, "0005-plain-draft.md") // a draft may leave questions open
}

func TestL10RequiresTheH1ToMatchTheFrontMatter(t *testing.T) {
	got := forRule(t, "lint", "L10")

	mentions(t, got, "0011-wrong-h1.md")
}

// severities returns the severity of every finding a rule produced, keyed by
// the document it was reported against.
func severities(t *testing.T, fixture, code string) map[string]lint.Severity {
	t.Helper()
	out := map[string]lint.Severity{}
	for _, f := range runLint(t, fixture) {
		if f.Rule == code {
			out[f.Path] = f.Severity
		}
	}
	return out
}

func TestL12WarnsThatASpecPageIsStale(t *testing.T) {
	got := forRule(t, "repo", "L12")

	mentions(t, got, "spec/database.md", "RFC-0001")
	// caching's only obsolescence claim comes from a draft, which has no effect.
	omits(t, got, "spec/caching.md")
	if s := severities(t, "repo", "L12")["spec/database.md"]; s != lint.Warning {
		t.Errorf("severity = %q, want %q", s, lint.Warning)
	}
}

func TestL13WarnsThatARefIsUnverified(t *testing.T) {
	got := forRule(t, "repo", "L13")

	mentions(t, got, "ref/0002-queue-survey.md")
	omits(t, got, "0001-engine-comparison.md") // verified within the window
}

func TestL14ReportsEmptyRequiredSections(t *testing.T) {
	got := forRule(t, "lint", "L14")
	mentions(t, got, "0012-empty-proposal.md", "Proposal")

	sev := severities(t, "lint", "L14")
	if sev["rfc/0012-empty-proposal.md"] != lint.Error {
		t.Errorf("terminal document severity = %q, want %q", sev["rfc/0012-empty-proposal.md"], lint.Error)
	}
	if sev["rfc/0013-proposed-empty.md"] != lint.Warning {
		t.Errorf("proposed document severity = %q, want %q", sev["rfc/0013-proposed-empty.md"], lint.Warning)
	}
}

func TestL14IsNotAppliedToWithdrawnDocuments(t *testing.T) {
	// RFC-0006 in the clean fixture was withdrawn with every section empty,
	// which is the normal state of a document abandoned before a verdict.
	for _, f := range runLint(t, "repo") {
		if f.Rule == "L14" && strings.Contains(f.Path, "0006-federated-search.md") {
			t.Errorf("L14 fired on a withdrawn document: %s: %s", f.Path, f.Message)
		}
	}
}

func TestL15ChecksGlossaryStructure(t *testing.T) {
	got := forRule(t, "lint", "L15")

	// One finding per symptom, not three substrings scattered across the set:
	// the out-of-order findings quote the term name, so they satisfied all
	// three on their own and two of L15's three checks were pinned by nothing.
	reports(t, got, "out of alphabetical order")
	reports(t, got, "repeats the entry on line")
	reports(t, got, "Two paragraphs", "paragraphs, want exactly one")
}

func TestL15AcceptsAWellFormedGlossary(t *testing.T) {
	if got := forRule(t, "repo", "L15"); len(got) != 0 {
		t.Errorf("L15 fired on a well-formed glossary: %q", got)
	}
}

func TestL16ReportsUnresolvedWikiLinks(t *testing.T) {
	got := forRule(t, "lint", "L16")

	mentions(t, got, "0014-wiki-links.md", "Some Unknown Thing")
	omits(t, got, "inline code", "not a link either") // code spans and fences
}

func TestL17ChecksRelativeLinksAndAnchors(t *testing.T) {
	got := forRule(t, "lint", "L17")

	mentions(t, got, "nowhere.md", "no-such-heading")
	omits(t, got, "#zebra", "includes-draft.md", "#present-heading") // these resolve
}

func TestL18WarnsWhenAnAcceptedDocumentDependsOnOne(t *testing.T) {
	got := forRule(t, "lint", "L18")

	mentions(t, got, "0015-accepted-on-draft.md", "RFC-0005")
	if s := severities(t, "lint", "L18")["rfc/0015-accepted-on-draft.md"]; s != lint.Warning {
		t.Errorf("severity = %q, want %q", s, lint.Warning)
	}
}

func TestTheCleanFixtureHasNoErrors(t *testing.T) {
	for _, f := range runLint(t, "repo") {
		if f.Severity == lint.Error {
			t.Errorf("unexpected error %s at %s:%d: %s", f.Rule, f.Path, f.Line, f.Message)
		}
	}
}

func TestEveryRuleHasAFixtureCase(t *testing.T) {
	fired := map[string]bool{}
	for _, fixture := range []string{"repo", "lint"} {
		for _, f := range runLint(t, fixture) {
			fired[f.Rule] = true
		}
	}
	for _, rule := range lint.Rules {
		if !fired[rule.Code] {
			t.Errorf("no fixture document makes %s fire", rule.Code)
		}
	}
	if len(lint.Rules) != 18 {
		t.Errorf("registered %d rules, want 18", len(lint.Rules))
	}
}

func TestL01ReportsARequiredKeyWrittenWithNoValue(t *testing.T) {
	got := forRule(t, "lint", "L01")

	mentions(t, got,
		"0016-empty-scalars.md:4: front matter key \"status\" is present but empty",
		"0016-empty-scalars.md:5: front matter key \"created\" is present but empty",
	)
	// decided and the relationship lists are legitimately empty.
	omits(t, got, `"decided" is present but empty`, `"depends" is present but empty`)
}

func TestAByteOrderMarkDoesNotHideTheFrontMatter(t *testing.T) {
	for _, f := range runLint(t, "lint") {
		if strings.Contains(f.Path, "byte-order-mark.md") {
			t.Errorf("a BOM produced a finding: %s:%d: %s (%s)", f.Path, f.Line, f.Message, f.Rule)
		}
	}
}

func TestL05ReportsBothListEntriesInAStableOrder(t *testing.T) {
	// Two findings that share a path, a line and a rule are ties under the
	// output sort, so their order is whatever order the rule emitted them in.
	first := forRule(t, "lint", "L05")
	for range 10 {
		if got := forRule(t, "lint", "L05"); !slices.Equal(got, first) {
			t.Fatalf("L05 output varies between runs:\n first %q\n then  %q", first, got)
		}
	}
	mentions(t, first, "RFC-0012 appears in both", "RFC-0015 appears in both")
}

func TestL14ReportsAnEmptySourcesSectionOnARef(t *testing.T) {
	got := forRule(t, "lint", "L14")

	mentions(t, got, "0003-empty-sources.md", "Sources")
	if s := severities(t, "lint", "L14")["ref/0003-empty-sources.md"]; s != lint.Error {
		t.Errorf("severity = %q, want %q: a ref is always editable", s, lint.Error)
	}
}

func TestL08NamesBothSectionsWhenTwoAreTransposed(t *testing.T) {
	got := forRule(t, "lint", "L08")

	mentions(t, got, `"Alternatives considered" appears before "Proposal"`)
}

func TestL02RejectsAFilenameWhoseSlugIsNotASlug(t *testing.T) {
	got := forRule(t, "lint", "L02")

	mentions(t, got,
		"Bad Slug.md",  // uppercase and a space are not a slug
		"0000-zero.md", // sequences start at 1
	)
}

func TestL07ChecksBackfilledAgainstStatusAndDecided(t *testing.T) {
	got := forRule(t, "lint", "L07")

	mentions(t, got,
		"0019-backfilled-draft.md",     // backfilled implies a decision already taken
		"0020-backfilled-too-early.md", // written before the decision it records
	)
}

func TestL08RequiresSourcesOnABackfilledDocument(t *testing.T) {
	got := forRule(t, "lint", "L08")

	mentions(t, got, "0021-backfilled-no-sources.md", "Sources")
	omits(t, got, "0009-legacy-authentication.md") // this one cites its sources
}

func TestL14ChecksSourcesOnABackfilledDocument(t *testing.T) {
	// L08 makes Sources required once a document is backfilled, so L14 must
	// hold it to the same standard as every other required section. A
	// reconstruction citing nothing is the case the section exists to prevent.
	got := forRule(t, "lint", "L14")

	mentions(t, got, "0003-backfilled-empty-sources.md", "Sources")
	omits(t, got, "0009-legacy-authentication.md") // this one cites real sources
}

func TestL14ChecksTheRejectionRationale(t *testing.T) {
	// spec/spec/lint.md says the rejection prompt left in place is reported as an empty
	// section: that is the whole mechanism stopping a rejected document from
	// being frozen with "Why?" as its permanent rationale.
	got := forRule(t, "lint", "L14")

	mentions(t, got, "0022-rejected-no-rationale.md", repo.RejectionRationale)
}

func TestHeadingsInsideHTMLCommentsAreNotSections(t *testing.T) {
	// A section that exists only inside a comment is not a section. Otherwise
	// commenting one out satisfies L08 while leaving the document unwritten.
	got := forRule(t, "lint", "L08")

	mentions(t, got, "0023-commented-section.md", "Motivation")
}

func TestL17IgnoresExternalLinks(t *testing.T) {
	// The clean fixture is the guard against false positives, so it has to
	// contain the shapes a real repository contains. A regression here turns
	// every downstream CI run red.
	got := forRule(t, "repo", "L17")

	omits(t, got, "https://", "mailto:")
}

func TestL18StaysQuietForWorkInFlight(t *testing.T) {
	// A proposed document depending on another proposed document is the normal
	// state of a repository with two designs in progress, and an accepted
	// target is always fine. Neither exception was asserted anywhere.
	got := forRule(t, "repo", "L18")

	omits(t, got, "RFC-0005", "RFC-0010", "RFC-0004")
}

func TestL13IsDisabledByRefStaleDaysOfZero(t *testing.T) {
	r := repotest.NewWith(t, `{"name":"T","ref_stale_days":0}`, map[string]string{
		"ref/0001-ancient.md": "---\nid: REF-0001\ntitle: Ancient\nverified: 2001-01-01\n---\n\n# REF-0001: Ancient\n\n## Sources\n\n- Somewhere.\n",
	})

	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L13" {
			t.Errorf("ref_stale_days of 0 disables the check, but it fired: %s", f.Message)
		}
	}
}

func TestL14LooksAtEveryInstanceOfASection(t *testing.T) {
	// The accept gate reads every section with a given title; L14 read only the
	// first. A document reaching a terminal status by any path other than the
	// gate was judged differently by the two.
	r := repotest.New(t, map[string]string{
		"rfc/0001-twice.md": "---\nid: RFC-0001\ntitle: Twice\nstatus: accepted\ncreated: 2026-01-01\ndecided: 2026-02-01\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n" +
			"# RFC-0001: Twice\n\n## Abstract\n\nx\n\n## Motivation\n\nx\n\n## Proposal\n\nWritten.\n\n## Proposal\n\n## Alternatives considered\n\nx\n\n" +
			"## Backwards compatibility\n\nx\n\n## Open questions\n\n## Changelog\n",
	})

	var found bool
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L14" && strings.Contains(f.Message, "Proposal") {
			found = true
		}
	}
	if !found {
		t.Error("a second, empty Proposal section was not reported")
	}
}

// ref renders a complete ref verified on a given date, so that L13 is the only
// rule with anything to say about it.
func ref(verified string) map[string]string {
	return map[string]string{
		"ref/0001-survey.md": "---\nid: REF-0001\ntitle: Survey\nverified: " + verified +
			"\n---\n\n# REF-0001: Survey\n\n## Summary\n\nA survey.\n\n## Sources\n\n- Somewhere.\n",
	}
}

// l13 runs the rules at a given instant and returns L13's messages.
func l13(t *testing.T, r *repo.Repo, at time.Time) []string {
	t.Helper()
	var out []string
	for _, f := range lint.Run(lint.NewContext(r, nil, at)) {
		if f.Rule == "L13" {
			out = append(out, f.Message)
		}
	}
	return out
}

func TestL13IsAFunctionOfTheDatesAndNotTheClock(t *testing.T) {
	// verified is parsed to midnight UTC, so comparing it against a wall-clock
	// instant made "ten days old" depend on the hour the run happened and the
	// zone it happened in. A ref verified exactly ref_stale_days ago is within
	// the window: the setting is how old the date may be.
	r := repotest.NewWith(t, `{"name":"T","ref_stale_days":10}`, ref("2026-09-01"))

	for _, zone := range []*time.Location{
		time.UTC,
		time.FixedZone("UTC+14", 14*60*60),
		time.FixedZone("UTC-11", -11*60*60),
	} {
		for _, hour := range []int{0, 2, 9, 14, 23} {
			at := time.Date(2026, 9, 11, hour, 30, 0, 0, zone)
			if fired := l13(t, r, at); len(fired) != 0 {
				t.Errorf("run at %s: exactly ten days old is within ref_stale_days of 10, but L13 said %q", at, fired)
			}
		}
	}
}

func TestL13StillWarnsBeyondTheWindow(t *testing.T) {
	// The other side of the boundary, so that the fix cannot be "never fire".
	r := repotest.NewWith(t, `{"name":"T","ref_stale_days":10}`, ref("2026-08-31"))

	at := time.Date(2026, 9, 11, 9, 0, 0, 0, time.FixedZone("BST", 60*60))
	fired := l13(t, r, at)
	if len(fired) != 1 {
		t.Fatalf("want one L13 finding for a ref eleven days old, got %d: %q", len(fired), fired)
	}
	if !strings.Contains(fired[0], "more than 10 days ago") {
		t.Errorf("message = %q, want it to say how old the window is", fired[0])
	}
}

func TestL18SaysNothingAboutARefInDepends(t *testing.T) {
	// A ref has no status, so there is nothing to compare it against. L04
	// already reports that a ref may not be named in depends; L18 rendered the
	// empty status into its message and produced "which is , while".
	got := forRule(t, "lint", "L18")

	omits(t, got, "which is ,", "REF-0001")
}

// backfilledRejected renders a complete backfilled, rejected RFC with Sources
// followed by the rationale, which is the arrangement PROCESS.md describes.
const backfilledRejected = `---
id: RFC-0001
title: Old idea
status: rejected
created: 2026-01-01
decided: 2026-02-01
backfilled: 2026-03-01
depends: []
updates: []
obsoletes: []
---

# RFC-0001: Old idea

## Abstract

What it proposed.

## Motivation

Why it came up.

## Proposal

The design.

## Alternatives considered

What lost.

## Backwards compatibility

Nothing.

## Open questions

## Changelog

- 2026-01-01: written.

## Sources

- The old changelog.

## Rejection rationale

Turned down on its merits.
`

func TestABackfilledRejectedDocumentCanLintClean(t *testing.T) {
	// PROCESS.md's Document structure section makes the rationale final; its
	// Backfilling section asks only that a backfilled document carry Sources. The two demands used to be read as
	// "Sources last" and "rationale after Sources", which no document could
	// satisfy, so a documented flag combination produced a permanently red
	// repository that L11 then forbade anyone repairing.
	r := repotest.New(t, map[string]string{"rfc/0001-old-idea.md": backfilledRejected})

	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Severity == lint.Error {
			t.Errorf("unexpected error %s at %s:%d: %s", f.Rule, f.Path, f.Line, f.Message)
		}
	}
}

func TestL08DoesNotDemandAClosingSectionItAlreadySaidWasMissing(t *testing.T) {
	// Two findings for one fault, the second contradicting the first.
	got := forRule(t, "lint", "L08")

	mentions(t, got, `missing required section "Sources"`)
	omits(t, got, `must be the last H2, but "Changelog" follows it`)
}

func TestL17ChecksAnchorsWithinTheSameDocument(t *testing.T) {
	// The same-document branch was executed by no test: disabling it left the
	// suite green, and a link to a heading that does not exist in the file that
	// names it went unreported.
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n" +
			"## Present heading\n\nSee [here](#present-heading) and [there](#no-such-heading).\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			got = append(got, f.Message)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one L17 finding, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "no-such-heading") {
		t.Errorf("finding = %q, want it to name the missing anchor", got[0])
	}
	if strings.Contains(got[0], "present-heading") {
		t.Errorf("finding = %q, but that anchor is present", got[0])
	}
}

func TestL17SkipsAnAnchorIntoSomethingThatIsNotMarkdown(t *testing.T) {
	// A non-markdown file has no headings to check, so only its existence is
	// in question. Inverting this guard left the suite green.
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n" +
			"See [the diagram](diagram.svg#layer1) and [the missing one](absent.svg).\n",
		"spec/diagram.svg": "<svg></svg>\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			got = append(got, f.Message)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one L17 finding, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "absent.svg") {
		t.Errorf("finding = %q, want it to name the file that is not there", got[0])
	}
}

func TestNoRuleRendersAnEmptyStatusIntoItsMessage(t *testing.T) {
	// Three rules formatted a status with no guard. A ref has none by design,
	// and a document may carry the key with nothing after it, so the message
	// came out as "which is , while" or "which is : a spec page". L01 and L04
	// report the real fault; these rules only had to keep quiet.
	r := repotest.New(t, map[string]string{
		// A ref, which has no status at all, named where only a design may be.
		"ref/0001-survey.md": "---\nid: REF-0001\ntitle: Survey\nverified: 2026-09-01\n---\n\n" +
			"# REF-0001: Survey\n\n## Sources\n\n- Somewhere.\n",
		// A spec page including it.
		"spec/page.md": "---\ntitle: Page\nincludes: [REF-0001]\n---\n\n# Page\n\nProse.\n",
		// A document whose own status is present but empty, depending on a draft.
		"rfc/0001-blank.md": "---\nid: RFC-0001\ntitle: Blank\nstatus:\ncreated: 2026-01-01\n" +
			"decided:\ndepends: [RFC-0002]\nupdates: [RFC-0002]\nobsoletes: []\n---\n\n" +
			"# RFC-0001: Blank\n\n## Abstract\n\nA.\n\n## Motivation\n\nB.\n\n## Proposal\n\nC.\n\n" +
			"## Alternatives considered\n\nD.\n\n## Backwards compatibility\n\nE.\n\n## Open questions\n\n## Changelog\n\n- x\n",
		"rfc/0002-draft.md": "---\nid: RFC-0002\ntitle: Draft\nstatus: draft\ncreated: 2026-01-01\n" +
			"decided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n" +
			"# RFC-0002: Draft\n\n## Abstract\n\nA.\n\n## Motivation\n\nB.\n\n## Proposal\n\nC.\n\n" +
			"## Alternatives considered\n\nD.\n\n## Backwards compatibility\n\nE.\n\n## Open questions\n\n## Changelog\n\n- x\n",
	})

	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		for _, bad := range []string{"which is ,", "which is :", "which is  ", "this document is "} {
			if strings.Contains(f.Message, bad) {
				t.Errorf("%s rendered an empty status: %q", f.Rule, f.Message)
			}
		}
		if strings.HasSuffix(f.Message, " is ") {
			t.Errorf("%s rendered an empty status: %q", f.Rule, f.Message)
		}
	}
}

func TestL17RefusesALinkTargetOutsideTheRepository(t *testing.T) {
	// path.Join cleans leading ".." away and filepath.Join then climbs out of
	// root, so the existence check read any path on the machine a document
	// cared to name. What it read never reached the output, but whether the
	// file exists did, and reading it at all is not this tool's business.
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n" +
			"See [outside](../../outside.md#secret).\n",
	})

	// A real file two levels above the repository, so that resolving the link
	// would succeed and the anchor check would read it.
	outside := filepath.Join(filepath.Dir(r.Config().Dir()), "outside.md")
	if err := os.WriteFile(outside, []byte("# Secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			got = append(got, f.Message)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one finding, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "leaves the repository") {
		t.Errorf("finding = %q, want it to say the target leaves the repository", got[0])
	}
}

func TestL10ReportsADocumentWithNoHeadingAtAll(t *testing.T) {
	// No fixture had a document without headings, so this branch ran in no
	// test: replacing it with a bare continue left the suite green, and a page
	// with prose and no H1 passed lint in silence.
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\nProse with no heading at all.\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L10" {
			got = append(got, f.Message)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one L10 finding, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "first heading must be an H1") {
		t.Errorf("finding = %q, want it to say an H1 is required", got[0])
	}
}

func TestL08SurvivesADocumentWithNoHeadings(t *testing.T) {
	// The len(titles) > 0 guard: without it the closing-section check indexes
	// titles[-1] and panics, and no fixture reached it.
	r := repotest.New(t, map[string]string{
		"ref/0001-survey.md": "---\nid: REF-0001\ntitle: Survey\nverified: 2026-09-01\n---\n\nNo headings here.\n",
	})

	lint.Run(lint.NewContext(r, nil, now)) // must not panic
}

func TestL17UnderstandsAngleBracketAndEncodedDestinations(t *testing.T) {
	// The destination was captured as [^)\s]+, so CommonMark's angle-bracket
	// form was read as "<my" for every path, not only one containing a space,
	// and a percent-encoded path was stat'd without decoding. Both are valid
	// links to files that exist, and both were reported as missing: a false
	// error at exit 2, which becomes permanently unclearable once frozen.
	r := repotest.New(t, map[string]string{
		"spec/my page.md": "---\ntitle: My page\nincludes: []\n---\n\n# My page\n\nProse.\n",
		"spec/links.md": "---\ntitle: Links\nincludes: []\n---\n\n# Links\n\n" +
			"See [a](<my page.md>), [b](my%20page.md) and [c](really-not-here.md).\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			got = append(got, f.Message)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one L17 finding, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "really-not-here.md") {
		t.Errorf("finding = %q, want it to name only the target that is missing", got[0])
	}
}

func TestL17ChecksReferenceStyleLinkDestinations(t *testing.T) {
	// Link reference definitions were not parsed at all, so a whole class of
	// genuinely broken links went unreported.
	r := repotest.New(t, map[string]string{
		"spec/links.md": "---\ntitle: Links\nincludes: []\n---\n\n# Links\n\n" +
			"See [the guide][gone] and [the other][here].\n\n" +
			"[gone]: does-not-exist.md\n[here]: links.md\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			got = append(got, f.Message)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one L17 finding, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "does-not-exist.md") {
		t.Errorf("finding = %q, want it to name the missing definition target", got[0])
	}
}

func TestL01ReportsAStatusThatIsNotOneOfTheFive(t *testing.T) {
	// The only status-validity check in lint, and no fixture reached it: with
	// the branch removed, a document with status "approved" lints clean while
	// the same document with "accepted" reports two errors.
	got := forRule(t, "lint", "L01")

	reports(t, got, "0024-bad-status.md", `status "approved" is not one of`)
}

func TestL01DoesNotCallAKeyEmptyWhenItCouldNotParseIt(t *testing.T) {
	// The emptiness test reads the decoded value, which stays zero when parsing
	// failed, so an accurate error was followed by a false one saying the key
	// was empty when the file plainly shows it is not.
	r := repotest.New(t, map[string]string{
		"ref/0001-survey.md": "---\nid: REF-0001\ntitle: Survey\nverified: not-a-date\n---\n\n" +
			"# REF-0001: Survey\n\n## Sources\n\n- Somewhere.\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L01" {
			got = append(got, f.Message)
		}
	}

	reports(t, got, "is not an ISO 8601 date")
	for _, m := range got {
		if strings.Contains(m, "present but empty") {
			t.Errorf("L01 also called the key empty: %q", m)
		}
	}
}

func TestL15RequiresEveryHeadingAfterThePreambleToBeAnEntry(t *testing.T) {
	// spec/spec/lint.md gives L15 four clauses and three were implemented. A page with
	// an H1 among the entries lints clean while the prose under it is silently
	// not a term, and that same shape is what made term remove destructive
	// before the entry extents were fixed.
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n" +
			"## Alpha\n\nFirst.\n\n# Interloper\n\nNot a term.\n\n## Beta\n\nSecond.\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L15" {
			got = append(got, f.Message)
		}
	}

	reports(t, got, "Interloper")
}

func TestL15AllowsThePreambleAndTheTitle(t *testing.T) {
	// The H1 and anything before the first entry are preamble, which is
	// specified and must stay silent.
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n" +
			"<!-- One term per entry. -->\n\n## Alpha\n\nFirst.\n\n## Beta\n\nSecond.\n",
	})

	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L15" {
			t.Errorf("L15 reported %q on a well-formed page", f.Message)
		}
	}
}

func TestL17IgnoresLinesThatOnlyLookLikeDefinitions(t *testing.T) {
	// The definition pattern had no end anchor, no title parse and no
	// non-empty-label requirement, so a footnote or a citation, which is the
	// shape a Sources section naturally takes, became a link target and an
	// exit-2 error an author could only clear by rewording the line.
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n" +
			"[^1]: Measured on 2026-09-01.\n\n" +
			"[1]: Smith, J. (2020), Wings internals.\n\n" +
			"[Decision]: we will use Postgres for the store.\n\n" +
			"[]: nothing at all\n\n" +
			"Some prose\n[notadef]: because this interrupts a paragraph\n",
	})

	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			t.Errorf("L17 read an ordinary line as a link definition: %q", f.Message)
		}
	}
}

func TestL17StillChecksARealDefinition(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"spec/page.md": "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n" +
			"See [a][gone], [b][here] and [c][titled].\n\n" +
			"[gone]: does-not-exist.md\n\n[here]: page.md\n\n[titled]: page.md \"A title\"\n",
	})

	var got []string
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L17" {
			got = append(got, f.Message)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one L17 finding, got %d: %q", len(got), got)
	}
	reports(t, got, "does-not-exist.md")
}

func TestL14LetsADraftBeAsEmptyAsItLikes(t *testing.T) {
	// The first thing every user does: `archdoc new rfc <title>` writes a
	// template whose sections hold only HTML comments, which count as nothing.
	// Without the draft skip, create-then-lint exits 2 on a document the tool
	// itself just wrote.
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\nstatus: draft\ncreated: 2026-01-01\n" +
			"decided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# RFC-0001: A\n\n" +
			"## Abstract\n\n<!-- Two or three sentences. -->\n\n## Motivation\n\n<!-- Why. -->\n\n" +
			"## Proposal\n\n<!-- The design. -->\n\n## Alternatives considered\n\n<!-- What lost. -->\n\n" +
			"## Backwards compatibility\n\n<!-- What breaks. -->\n\n## Open questions\n\n## Changelog\n",
	})

	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L14" {
			t.Errorf("L14 reported a draft's empty section: %s", f.Message)
		}
	}
}

func TestL14StillReportsAProposedDocument(t *testing.T) {
	// The other side of the same switch, so the fix cannot be "never fire".
	r := repotest.New(t, map[string]string{
		"rfc/0001-a.md": "---\nid: RFC-0001\ntitle: A\nstatus: proposed\ncreated: 2026-01-01\n" +
			"decided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# RFC-0001: A\n\n" +
			"## Abstract\n\n<!-- Two or three sentences. -->\n\n## Motivation\n\nWhy.\n\n" +
			"## Proposal\n\nThe design.\n\n## Alternatives considered\n\nWhat lost.\n\n" +
			"## Backwards compatibility\n\nNothing.\n\n## Open questions\n\n## Changelog\n",
	})

	var fired bool
	for _, f := range lint.Run(lint.NewContext(r, nil, now)) {
		if f.Rule == "L14" {
			fired = true
		}
	}
	if !fired {
		t.Error("L14 said nothing about a proposed document with an empty required section")
	}
}

func TestL11IgnoresFilesOutsideArchdocsOwnRoot(t *testing.T) {
	// strings.CutPrefix returns the path unchanged when the prefix does not
	// match, so the !inRoot skip is the only thing keeping files archdoc does
	// not own out of L11's deletion scan. Without it a repository with
	// root: docs and any markdown file in a top-level rfc/ fails lint with an
	// error about a file that is not archdoc's.
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "Fixture")

	write(t, dir, "docs/archdoc.json", `{"name":"Nested","branch":"main"}`)
	write(t, dir, "docs/rfc/0001-mine.md", rfc("RFC-0001", "Mine", "accepted", " 2026-02-01", "Committed."))
	// A terminal-looking document outside archdoc's root, which it does not own.
	write(t, dir, "rfc/0001-not-mine.md", rfc("RFC-0001", "Not mine", "accepted", " 2026-02-01", "Committed."))
	run("add", "-A")
	run("commit", "-m", "initial")

	for _, f := range lintDir(t, filepath.Join(dir, "docs")) {
		if f.Rule == "L11" && strings.Contains(f.Path, "not-mine") {
			t.Errorf("L11 reported a file outside archdoc's root: %s: %s", f.Path, f.Message)
		}
	}
}
