package lifecycle_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"archdoc.dev/internal/lifecycle"
	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/repotest"
)

var now = time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

// document writes one document into a throwaway repository and returns it, so
// each test states exactly the input it cares about.
func document(t *testing.T, path, source string) *repo.Document {
	t.Helper()
	return documentIn(t, repotest.DefaultConfig, path, source)
}

// documentIn is the same with a particular configuration, for the cases where
// the setting under test lives in archdoc.json.
func documentIn(t *testing.T, configuration, path, source string) *repo.Document {
	t.Helper()
	r := repotest.NewWith(t, configuration, map[string]string{path: source})
	return repotest.Document(t, r, path)
}

// rfc renders a complete, written RFC so that only the field under test varies.
func rfc(status, decided, extras string) string {
	return `---
id: RFC-0001
title: A design   # the working title, kept short
status: ` + status + `
created: 2026-01-01
decided:` + decided + `
depends: [ADR-0001]   # the storage decision
updates: []
obsoletes: []
---

# RFC-0001: A design

## Abstract

What this proposes.

## Motivation

Why it matters.

## Proposal

The design itself.

## Alternatives considered

What lost.

## Backwards compatibility

Nothing breaks.

## Open questions
` + extras + `
## Changelog

- 2026-01-01: written.
`
}

func TestPermittedTransitions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		from   repo.Status
		strict bool
		want   []repo.Status
	}{
		{"draft, strict", repo.StatusDraft, true, []repo.Status{repo.StatusProposed, repo.StatusWithdrawn}},
		{"draft, relaxed", repo.StatusDraft, false, []repo.Status{repo.StatusProposed, repo.StatusWithdrawn, repo.StatusAccepted, repo.StatusRejected}},
		{"proposed", repo.StatusProposed, true, []repo.Status{repo.StatusAccepted, repo.StatusRejected, repo.StatusWithdrawn}},
		{"accepted is terminal", repo.StatusAccepted, false, nil},
		{"withdrawn is terminal", repo.StatusWithdrawn, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lifecycle.Permitted(tc.from, tc.strict)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Permitted(%s, strict=%v) = %v, want %v", tc.from, tc.strict, got, tc.want)
			}
		})
	}
}

func TestApplyRewritesOnlyTheFieldsItChanges(t *testing.T) {
	d := document(t, "rfc/0001-a-design.md", rfc("proposed", "", "\n"))

	out, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !strings.Contains(string(out), "status: accepted\n") {
		t.Errorf("status was not rewritten:\n%s", out)
	}
	if !strings.Contains(string(out), "decided: 2026-09-11\n") {
		t.Errorf("decided was not set:\n%s", out)
	}
	// Everything else must survive exactly, comments included.
	for _, want := range []string{
		"title: A design   # the working title, kept short\n",
		"depends: [ADR-0001]   # the storage decision\n",
		"created: 2026-01-01\n",
		"obsoletes: []\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("line %q did not survive the rewrite:\n%s", want, out)
		}
	}
	// The body is untouched.
	if !strings.Contains(string(out), "## Changelog\n\n- 2026-01-01: written.\n") {
		t.Errorf("the body changed:\n%s", out)
	}
}

func TestApplyRefusesATransitionThatIsNotPermitted(t *testing.T) {
	d := document(t, "rfc/0001-a-design.md", rfc("draft", "", "\n"))

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil {
		t.Fatal("Apply allowed draft to accepted under strict mode")
	}
	if !strings.Contains(err.Error(), "proposed") {
		t.Errorf("error = %q, want it to name the permitted transitions", err)
	}
}

func TestAcceptRefusesAnUnfinishedDocument(t *testing.T) {
	empty := strings.Replace(rfc("proposed", "", "\n"), "The design itself.", "<!-- the design -->", 1)
	d := document(t, "rfc/0001-a-design.md", empty)

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil || !strings.Contains(err.Error(), "Proposal") {
		t.Errorf("error = %v, want it to name the empty Proposal section", err)
	}
}

func TestAcceptRefusesWhileQuestionsAreOpen(t *testing.T) {
	d := document(t, "rfc/0001-a-design.md", rfc("proposed", "", "\nWhether this is right.\n"))

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil || !strings.Contains(err.Error(), "Open questions") {
		t.Errorf("error = %v, want it to name the open questions", err)
	}
}

func TestAcceptRefusesAnUnresolvedWikiLink(t *testing.T) {
	linked := strings.Replace(rfc("proposed", "", "\n"), "What this proposes.", "See [[RFC-0002]].", 1)
	d := document(t, "rfc/0001-a-design.md", linked)

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil || !strings.Contains(err.Error(), "RFC-0002") {
		t.Errorf("error = %v, want it to name the unresolved wiki link", err)
	}
}

func TestRejectAppendsTheRationaleSection(t *testing.T) {
	d := document(t, "rfc/0001-a-design.md", rfc("proposed", "", "\n"))

	out, err := lifecycle.Apply(d, repo.StatusRejected, now)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(out), "## Rejection rationale") {
		t.Errorf("no rationale section was appended:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimRight(string(out), "\n"), "-->") {
		t.Errorf("the rationale prompt is not the last thing in the file:\n%s", out)
	}
	if !strings.Contains(string(out), "status: rejected\n") {
		t.Error("status was not rewritten")
	}
}

func TestWithdrawChecksNothing(t *testing.T) {
	// A document abandoned before a verdict is empty by nature, and may carry
	// unresolved links; withdrawing it must still work.
	bare := strings.Replace(rfc("draft", "", "\n"), "The design itself.", "<!-- unwritten [[RFC-0002]] -->", 1)
	d := document(t, "rfc/0001-a-design.md", bare)

	out, err := lifecycle.Apply(d, repo.StatusWithdrawn, now)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(out), "status: withdrawn\n") || !strings.Contains(string(out), "decided: 2026-09-11\n") {
		t.Errorf("withdraw did not record the status and date:\n%s", out)
	}
}

func TestRejectKeepsCarriageReturnsInACRLFFile(t *testing.T) {
	crlf := strings.ReplaceAll(rfc("proposed", "", "\n"), "\n", "\r\n")
	d := document(t, "rfc/0001-a-design.md", crlf)

	out, err := lifecycle.Apply(d, repo.StatusRejected, now)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// The whole file, not only the appended section. Apply calls SetField twice
	// before appending, and SetField's carriage-return preservation is the only
	// thing keeping those two rewritten lines CRLF: with that guard broken,
	// "status: rejected" and the decided date came out bare-LF among CRLF lines
	// and this test still passed.
	for i, line := range strings.Split(string(out), "\n") {
		if i == len(strings.Split(string(out), "\n"))-1 && line == "" {
			continue // the trailing newline
		}
		if !strings.HasSuffix(line, "\r") {
			t.Errorf("line %d lost its carriage return: %q", i+1, line)
		}
	}
	for _, want := range []string{"status: rejected\r", "## Rejection rationale\r"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output is missing %q:\n%q", want, string(out))
		}
	}
}

func TestApplyRefusesADocumentWithNoLifecycle(t *testing.T) {
	d := document(t, "spec/database.md", "---\ntitle: Database\nincludes: []\n---\n\n# Database\n\nText.\n")

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil || !strings.Contains(err.Error(), "spec") {
		t.Errorf("error = %v, want it to say a spec page has no lifecycle", err)
	}
}

func TestAcceptLooksAtEveryOpenQuestionsSection(t *testing.T) {
	// A second heading with the same title is unusual but legal. Checking only
	// the first lets content through the gate unseen.
	twice := strings.Replace(rfc("proposed", "", "\n"),
		"## Changelog", "## Open questions\n\nStill unresolved.\n\n## Changelog", 1)
	d := document(t, "rfc/0001-a-design.md", twice)

	if _, err := lifecycle.Apply(d, repo.StatusAccepted, now); err == nil {
		t.Error("accepted a document whose second Open questions section is not empty")
	}
}

func TestApplyAllowsDraftToAcceptedWhenStrictIsOff(t *testing.T) {
	// strict exists solely to forbid this transition, so the relaxed path is
	// worth running end to end rather than only through the graph.
	d := documentIn(t, `{"name":"T","strict":false}`, "rfc/0001-a-design.md", rfc("draft", "", "\n"))

	out, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(out), "status: accepted\n") || !strings.Contains(string(out), "decided: 2026-09-11\n") {
		t.Errorf("the relaxed transition did not record the status and date:\n%s", out)
	}
}

func TestAcceptRefusesASectionHoldingAnUnterminatedComment(t *testing.T) {
	// Deleting the closing marker of the template's own prompt used to carry a
	// document straight through the gate, and L11 then forbade the edit that
	// would have fixed it.
	// The last gated section, because that is where the hole is silent. An
	// unterminated comment earlier in the document swallows the headings below
	// it, and the gate then refuses for a different and louder reason.
	open := strings.Replace(rfc("proposed", "", "\n"), "Nothing breaks.", "<!-- what breaks", 1)
	d := document(t, "rfc/0001-a-design.md", open)

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil || !strings.Contains(err.Error(), "Backwards compatibility") {
		t.Errorf("error = %v, want it to name the empty Backwards compatibility section", err)
	}
}

func TestApplyNamesAnUnusableStatusRatherThanCallingItTerminal(t *testing.T) {
	// "Terminal" was derived from the transition graph returning nothing, and
	// it returns nothing for anything that is not draft or proposed. So an
	// empty, absent or miscapitalised status produced "  is terminal, so it
	// cannot become proposed: write a new document that updates or obsoletes
	// it", which is wrong twice over: the document is not frozen, and the fault
	// is a one-line front matter edit.
	for _, status := range []string{"", "Draft", "nonsense"} {
		d := document(t, "rfc/0001-a-design.md", rfc(status, "", "\n"))

		_, err := lifecycle.Apply(d, repo.StatusProposed, now)
		if err == nil {
			t.Errorf("status %q was accepted", status)
			continue
		}
		if strings.Contains(err.Error(), "is terminal") {
			t.Errorf("status %q: %v, want it to name the status as unusable", status, err)
		}
		if !strings.Contains(err.Error(), "status") {
			t.Errorf("status %q: %v, want it to say what is wrong", status, err)
		}
	}
}

func TestApplyStillExplainsATerminalDocument(t *testing.T) {
	d := document(t, "rfc/0001-a-design.md", rfc("accepted", " 2026-02-01", "\n"))

	_, err := lifecycle.Apply(d, repo.StatusProposed, now)
	if err == nil || !strings.Contains(err.Error(), "is terminal") {
		t.Errorf("error = %v, want it to say the document is terminal", err)
	}
}

func TestAcceptRefusesADocumentMissingARequiredSection(t *testing.T) {
	// The suite only ever emptied a section, never removed one, so deleting
	// this branch left everything green. A document frozen with a section
	// absent produces an L08 finding that L11 then forbids anyone to fix.
	without := strings.Replace(rfc("proposed", "", "\n"), "## Alternatives considered\n\nWhat lost.\n\n", "", 1)
	d := document(t, "rfc/0001-a-design.md", without)

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil || !strings.Contains(err.Error(), "Alternatives considered") {
		t.Errorf("error = %v, want it to name the missing section", err)
	}
	if err != nil && !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to say the section is missing rather than empty", err)
	}
}

func TestRejectRefusesADocumentWhoseParseStopsShort(t *testing.T) {
	// The rationale is appended to the end of the file. When the body ends
	// inside an unterminated comment that position is inside the comment, so
	// the command reported writing a section that did not then exist.
	open := strings.Replace(rfc("proposed", "", "\n"), "- 2026-01-01: written.", "- 2026-01-01: written. <!-- TODO", 1)
	d := document(t, "rfc/0001-a-design.md", open)

	_, err := lifecycle.Apply(d, repo.StatusRejected, now)
	if err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("error = %v, want it to refuse and name the unterminated marker", err)
	}
}

func TestAcceptRefusesToWriteADecidedDateBeforeCreated(t *testing.T) {
	// Apply set decided to now without comparing it to created, and no rule
	// rejects a created date in the future, so a document lints clean until the
	// transition writes the value that makes L07 fail forever: L11 then forbids
	// editing either date, and deleting the file is an L11 error too.
	//
	// It needs no hand-editing to reach: `new` in one zone and `accept` in
	// another at the same instant differ by a day.
	ahead := strings.Replace(rfc("proposed", "", "\n"), "created: 2026-01-01", "created: 2027-01-01", 1)
	d := document(t, "rfc/0001-a-design.md", ahead)

	_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
	if err == nil {
		t.Fatal("the transition was allowed, writing a decided date before created")
	}
	if !strings.Contains(err.Error(), "created") {
		t.Errorf("error = %v, want it to name the created date", err)
	}
}

func TestAcceptAllowsADecisionOnTheDayOfCreation(t *testing.T) {
	// The boundary: created today, decided today, which is the ordinary case
	// for a small decision and must not be refused.
	sameDay := strings.Replace(rfc("proposed", "", "\n"), "created: 2026-01-01",
		"created: "+now.Format(repo.DateLayout), 1)
	d := document(t, "rfc/0001-a-design.md", sameDay)

	if _, err := lifecycle.Apply(d, repo.StatusAccepted, now); err != nil {
		t.Errorf("a same-day decision was refused: %v", err)
	}
}

func TestAcceptRefusesWhatTheLinterWillRejectForever(t *testing.T) {
	// The gate checked sections present, sections non-empty and wiki links.
	// L08 and L10 check more, both report errors, and neither softens against a
	// frozen document, so anything the gate let through became a permanently
	// red lint that L11 forbids anyone repairing.
	for _, tc := range []struct{ name, source, want string }{
		{
			"an H1 that does not match the front matter",
			strings.Replace(rfc("proposed", "", "\n"), "# RFC-0001: A design", "# RFC-0001: A Design", 1),
			"H1",
		},
		{
			"required sections out of order",
			strings.Replace(
				strings.Replace(rfc("proposed", "", "\n"), "## Abstract\n\nWhat this proposes.\n\n", "", 1),
				"## Motivation\n\nWhy it matters.\n",
				"## Motivation\n\nWhy it matters.\n\n## Abstract\n\nWhat this proposes.\n", 1),
			"Abstract",
		},
		{
			"a required section missing entirely, even an ungated one",
			strings.Replace(rfc("proposed", "", "\n"), "## Changelog\n\n- 2026-01-01: written.\n", "", 1),
			"Changelog",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := document(t, "rfc/0001-a-design.md", tc.source)

			_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
			if err == nil {
				t.Fatalf("accept allowed it")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

func TestAcceptRefusesADocumentWithNoH1AtAll(t *testing.T) {
	// The gate has two H1 arms. The mismatched-text one is pinned; the
	// "no H1, or a first heading that is not an H1" one was pinned by nothing,
	// because every other test's document opens with a correct H1.
	for _, name := range []string{"no heading at all", "first heading is an H2"} {
		source := rfc("proposed", "", "\n")
		switch name {
		case "no heading at all":
			source = strings.Replace(source, "# RFC-0001: A design\n\n", "", 1)
		case "first heading is an H2":
			source = strings.Replace(source, "# RFC-0001: A design", "## RFC-0001: A design", 1)
		}
		d := document(t, "rfc/0001-a-design.md", source)

		_, err := lifecycle.Apply(d, repo.StatusAccepted, now)
		if err == nil {
			t.Errorf("%s: accept allowed it", name)
			continue
		}
		// The exact message, because the arm below this one also refuses these
		// documents: it compares the H1's text and a missing H1 has none, so it
		// reports `the H1 reads ""`. This arm is not what makes the document
		// refused, it is what makes the refusal say something useful, and only
		// an assertion on the wording pins that.
		if !strings.Contains(err.Error(), "has no H1") {
			t.Errorf("%s: error = %v, want it to say the document has no H1", name, err)
		}
	}
}
