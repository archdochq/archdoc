package lint_test

import (
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/lint"
)

// frozenWithTwoWarnings is accepted on the branch with an empty Proposal and
// decided before created, so L14 and L07 each report a warning on it that no
// permitted edit can clear.
var frozenWithTwoWarnings = rfc("RFC-0001", "Frozen", "accepted", " 2025-12-01", "<!-- the design -->")

func acknowledging(acknowledged string) string {
	return `{"name": "Frozen", "branch": "main", "acknowledged": ` + acknowledged + `}`
}

func messages(findings []lint.Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Message)
	}
	return out
}

func TestAnAcknowledgedWarningIsLeftOut(t *testing.T) {
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{path: frozenWithTwoWarnings})
	if got := findingsFor(lintDir(t, dir), "L14", path); len(got) != 1 || got[0].Severity != lint.Warning {
		t.Fatalf("before acknowledging, L14 = %v, want its one warning", got)
	}
	write(t, dir, "archdoc.json", acknowledging(`{"`+path+`": ["L14"]}`))

	findings := lintDir(t, dir)

	if got := findingsFor(findings, "L14", path); len(got) != 0 {
		t.Errorf("acknowledged L14 is still reported: %v", got)
	}
	if got := findingsFor(findings, "L07", path); len(got) != 1 || got[0].Severity != lint.Warning {
		t.Errorf("L07, which is not acknowledged, = %v, want its one warning", got)
	}
	if got := findingsFor(findings, "L22", "archdoc.json"); len(got) != 0 {
		t.Errorf("L22 reports an acknowledgement that matches a warning: %v", got)
	}
}

func TestAnErrorIsNeverAcknowledged(t *testing.T) {
	const path = "rfc/0001-freezing.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Freezing", "proposed", "", "<!-- the design -->"),
	})
	// Accepted in the working tree only, so the empty Proposal is still an
	// error: this is the commit that freezes it.
	write(t, dir, path, rfc("RFC-0001", "Freezing", "accepted", " 2026-02-01", "<!-- the design -->"))
	write(t, dir, "archdoc.json", acknowledging(`{"`+path+`": ["L14"]}`))

	findings := lintDir(t, dir)

	if got := findingsFor(findings, "L14", path); len(got) != 1 || got[0].Severity != lint.Error {
		t.Errorf("L14 = %v, want its error still reported", got)
	}
}

func TestAnErrorOnAFrozenDocumentIsNeverAcknowledged(t *testing.T) {
	// L11's error on a frozen document that changed is the one an
	// acknowledgement must never hide: it would let the edit through.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{path: frozenWithTwoWarnings})
	write(t, dir, path, strings.Replace(frozenWithTwoWarnings, "<!-- the design -->", "An edit.", 1))
	write(t, dir, "archdoc.json", acknowledging(`{"`+path+`": ["L11"]}`))

	findings := lintDir(t, dir)

	if got := findingsFor(findings, "L11", path); len(got) != 1 || got[0].Severity != lint.Error {
		t.Errorf("L11 = %v, want its error still reported", got)
	}
}

func TestAWarningOnADocumentThatIsNotFrozenIsNeverAcknowledged(t *testing.T) {
	// A ref has no lifecycle and is never frozen, and L13 warns on it.
	const path = "ref/0001-old.md"
	dir := gitFixture(t, map[string]string{
		path: "---\nid: REF-0001\ntitle: Old\nverified: 2020-01-01\n---\n\n# REF-0001: Old\n\n## Sources\n\n- Somewhere.\n",
	})
	write(t, dir, "archdoc.json", acknowledging(`{"`+path+`": ["L13"]}`))

	findings := lintDir(t, dir)

	if got := findingsFor(findings, "L13", path); len(got) != 1 || got[0].Severity != lint.Warning {
		t.Errorf("L13 = %v, want its warning still reported", got)
	}
}

func TestL22ReportsEachStaleAcknowledgement(t *testing.T) {
	const frozen = "rfc/0001-frozen.md"
	const draft = "rfc/0002-draft.md"
	dir := gitFixture(t, map[string]string{
		frozen: frozenWithTwoWarnings,
		draft:  rfc("RFC-0002", "Draft", "draft", "", "A design."),
	})
	write(t, dir, "archdoc.json", acknowledging(`{
		"`+frozen+`": ["L14", "L17", "M01", "X99"],
		"`+draft+`": ["L07", "L14"],
		"rfc/0003-gone.md": ["L07"]
	}`))

	got := findingsFor(lintDir(t, dir), "L22", "archdoc.json")

	want := []string{
		"acknowledged rule L17 reports no warning on rfc/0001-frozen.md: remove it from acknowledged",
		"acknowledged code M01 on rfc/0001-frozen.md is not an L rule: remove it from acknowledged",
		"acknowledged code X99 on rfc/0001-frozen.md is not an L rule: remove it from acknowledged",
		"acknowledged rfc/0002-draft.md is not frozen: remove it from acknowledged",
		"acknowledged rfc/0003-gone.md is not a document: remove it from acknowledged",
	}
	if !slices.Equal(messages(got), want) {
		t.Errorf("L22 reports\n%s\nwant\n%s", strings.Join(messages(got), "\n"), strings.Join(want, "\n"))
	}
	for _, f := range got {
		if f.Severity != lint.Error {
			t.Errorf("%q is a %s, want an error: archdoc.json is always editable", f.Message, f.Severity)
		}
	}
}

func TestL22ReportsArchdocJSONFromBeneathTheRoot(t *testing.T) {
	const path = "rfc/0001-gone.md"
	dir := gitFixture(t, nil)
	write(t, dir, "archdoc.json", `{"name": "Frozen", "branch": "main", "root": "spec", "acknowledged": {"`+path+`": ["L07"]}}`)

	if got := findingsFor(lintDir(t, dir), "L22", "../archdoc.json"); len(got) != 1 {
		t.Errorf("L22 at ../archdoc.json = %v, want one finding: paths are relative to root", got)
	}
}
