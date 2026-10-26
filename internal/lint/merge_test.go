package lint_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/config"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
)

// mergeLint runs every rule over dir, the merge rules included, judging the
// working tree against main as a pull request checked out merged into it.
func mergeLint(t *testing.T, dir string) []lint.Finding {
	t.Helper()
	c, err := config.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := repo.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	g, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, err := changes.ReadBase(r, g)
	if err != nil {
		t.Fatal(err)
	}
	return lint.Run(lint.NewContext(r, g, now).WithBase(base))
}

func remove(t *testing.T, dir, path string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

func backfilled(document string) string {
	return strings.Replace(document, "created: 2026-01-01\n", "created: 2026-01-01\nbackfilled: 2026-03-01\n", 1)
}

// expect asserts that rule reports path exactly want times.
func expect(t *testing.T, findings []lint.Finding, rule, path string, want int, says string) {
	t.Helper()
	got := findingsFor(findings, rule, path)
	if len(got) != want {
		t.Fatalf("%s on %s: got %d findings, want %d: %v", rule, path, len(got), want, got)
	}
	for _, f := range got {
		if f.Severity != lint.Error || !strings.Contains(f.Message, says) {
			t.Errorf("%s on %s: got %s %q, want an error saying %q", rule, path, f.Severity, f.Message, says)
		}
	}
}

func TestM01ReportsADraftArrivingButNotOneAlreadyOnTheBase(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-old-draft.md": rfc("RFC-0001", "Old draft", "draft", "", "Merged before this rule."),
	})
	write(t, dir, "rfc/0002-new-draft.md", rfc("RFC-0002", "New draft", "draft", "", "Never proposed."))
	write(t, dir, "rfc/0003-proposed.md", rfc("RFC-0003", "Proposed", "proposed", "", "Offered."))

	got := mergeLint(t, dir)
	expect(t, got, "M01", "rfc/0002-new-draft.md", 1, "archdoc propose RFC-0002")
	expect(t, got, "M01", "rfc/0001-old-draft.md", 0, "")
	expect(t, got, "M01", "rfc/0003-proposed.md", 0, "")
}

func TestM02NamesTheArrivingDocumentForRenumbering(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0002-merged.md": rfc("RFC-0002", "Merged", "proposed", "", "Already holds the number."),
	})
	write(t, dir, "rfc/0002-arriving.md", rfc("RFC-0002", "Arriving", "proposed", "", "Claims the same number."))

	got := mergeLint(t, dir)
	expect(t, got, "M02", "rfc/0002-arriving.md", 1, "RFC-0002 is already rfc/0002-merged.md's on main; renumber this document instead: archdoc renumber rfc/0002-arriving.md")
	expect(t, got, "M02", "rfc/0002-merged.md", 0, "")
}

func TestM03JudgesEachChangeAsOneStepFromTheBase(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-proposed.md":  rfc("RFC-0001", "Proposed", "proposed", "", "On the base."),
		"rfc/0002-draft.md":     rfc("RFC-0002", "Draft", "draft", "", "On the base."),
		"rfc/0006-frozen.md":    rfc("RFC-0006", "Frozen", "accepted", " 2026-02-01", "On the base."),
		"rfc/0007-backward.md":  rfc("RFC-0007", "Backward", "proposed", "", "On the base."),
		"rfc/0008-bogus.md":     rfc("RFC-0008", "Bogus", "proposed", "", "On the base."),
		"rfc/0009-was-bogus.md": rfc("RFC-0009", "Was bogus", "nonsense", "", "On the base."),
	})
	write(t, dir, "rfc/0007-backward.md", rfc("RFC-0007", "Backward", "draft", "", "On the base."))
	write(t, dir, "rfc/0008-bogus.md", rfc("RFC-0008", "Bogus", "nonsense", "", "On the base."))
	write(t, dir, "rfc/0009-was-bogus.md", rfc("RFC-0009", "Was bogus", "proposed", "", "On the base."))
	write(t, dir, "rfc/0006-frozen.md", rfc("RFC-0006", "Frozen", "withdrawn", " 2026-02-01", "On the base."))
	write(t, dir, "rfc/0001-proposed.md", rfc("RFC-0001", "Proposed", "accepted", " 2026-02-01", "On the base."))
	write(t, dir, "rfc/0002-draft.md", rfc("RFC-0002", "Draft", "accepted", " 2026-02-01", "On the base."))
	write(t, dir, "rfc/0003-at-once.md", rfc("RFC-0003", "At once", "accepted", " 2026-02-01", "Proposed and accepted here."))
	write(t, dir, "rfc/0004-backfilled.md", backfilled(rfc("RFC-0004", "Backfilled", "accepted", " 2026-02-01", "Written terminal.")))
	write(t, dir, "rfc/0005-offered.md", rfc("RFC-0005", "Offered", "proposed", "", "Arrives proposed."))

	got := mergeLint(t, dir)
	expect(t, got, "M03", "rfc/0001-proposed.md", 0, "")
	expect(t, got, "M03", "rfc/0002-draft.md", 1, "moves from draft on main to accepted")
	expect(t, got, "M03", "rfc/0003-at-once.md", 1, "arrives on main as accepted, which is not a permitted transition from draft; with strict on")
	// The hint is about a move from draft; this one is backwards, and has none.
	expect(t, got, "M03", "rfc/0007-backward.md", 1, "moves from proposed on main to draft, which is not a permitted transition")
	if f := findingsFor(got, "M03", "rfc/0007-backward.md"); strings.Contains(f[0].Message, "strict") {
		t.Errorf("a move back to draft carries the strict hint: %q", f[0].Message)
	}
	// A status that is not one is L01's, on either side.
	expect(t, got, "M03", "rfc/0008-bogus.md", 0, "")
	expect(t, got, "M03", "rfc/0009-was-bogus.md", 0, "")
	expect(t, got, "M03", "rfc/0004-backfilled.md", 0, "")
	expect(t, got, "M03", "rfc/0005-offered.md", 0, "")
	// L11 reports every change to a frozen document, its status included.
	expect(t, got, "M03", "rfc/0006-frozen.md", 0, "")
	expect(t, got, "L11", "rfc/0006-frozen.md", 1, "changed since it was frozen")
}

func TestM03ReadsStrictFromTheBaseSoAPullRequestCannotRelaxIt(t *testing.T) {
	dir := gitFixture(t, nil) // strict is on by default
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","strict":false}`)
	write(t, dir, "rfc/0001-at-once.md", rfc("RFC-0001", "At once", "accepted", " 2026-02-01", "Accepted here."))

	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 1, "not a permitted transition")
}

func TestM03PermitsWhatTheBaseRelaxes(t *testing.T) {
	dir := gitFixture(t, nil)
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","strict":false}`)
	commit(t, dir, "relax")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main"}`)
	write(t, dir, "rfc/0001-at-once.md", rfc("RFC-0001", "At once", "accepted", " 2026-02-01", "Accepted here."))

	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 0, "")
}

func TestM03ReadsTheWorkingTreesStrictWhereTheBaseHasNoConfiguration(t *testing.T) {
	dir := gitFixture(t, nil)
	run(t, dir, "rm", "-q", "--cached", "archdoc.json")
	run(t, dir, "commit", "-q", "-m", "no configuration")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","strict":false}`)
	write(t, dir, "rfc/0001-at-once.md", rfc("RFC-0001", "At once", "accepted", " 2026-02-01", "Accepted here."))

	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 0, "")
}

func TestARenameIsJudgedFromTheDocumentOnTheBase(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-old-name.md": rfc("RFC-0001", "Old name", "draft", "", "On the base."),
		"rfc/0002-proposed.md": rfc("RFC-0002", "Proposed", "proposed", "", "On the base."),
	})
	remove(t, dir, "rfc/0001-old-name.md")
	write(t, dir, "rfc/0001-new-name.md", rfc("RFC-0001", "New name", "draft", "", "Renamed."))
	remove(t, dir, "rfc/0002-proposed.md")
	write(t, dir, "rfc/0002-renamed.md", rfc("RFC-0002", "Renamed", "accepted", " 2026-02-01", "Renamed and accepted."))

	got := mergeLint(t, dir)
	for _, rule := range []string{"M01", "M02", "M03"} {
		expect(t, got, rule, "rfc/0001-new-name.md", 0, "")
		expect(t, got, rule, "rfc/0002-renamed.md", 0, "")
	}
	expect(t, got, "M04", "rfc/0002-proposed.md", 0, "")
}

func TestM04ReportsAProposedDocumentDeletedOrRenumbered(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-deleted.md":    rfc("RFC-0001", "Deleted", "proposed", "", "On the base."),
		"rfc/0002-renumbered.md": rfc("RFC-0002", "Renumbered", "proposed", "", "On the base."),
		"rfc/0003-draft.md":      rfc("RFC-0003", "Draft", "draft", "", "On the base."),
		"rfc/0004-accepted.md":   rfc("RFC-0004", "Accepted", "accepted", " 2026-02-01", "On the base."),
		"rfc/misnamed.md":        rfc("RFC-0006", "Misnamed", "proposed", "", "No number in its name."),
	})
	remove(t, dir, "rfc/misnamed.md")
	remove(t, dir, "rfc/0001-deleted.md")
	remove(t, dir, "rfc/0002-renumbered.md")
	write(t, dir, "rfc/0005-renumbered.md", rfc("RFC-0005", "Renumbered", "proposed", "", "On the base."))
	remove(t, dir, "rfc/0003-draft.md")
	remove(t, dir, "rfc/0004-accepted.md")

	got := mergeLint(t, dir)
	expect(t, got, "M04", "rfc/0001-deleted.md", 1, "archdoc withdraw RFC-0001")
	expect(t, got, "M04", "rfc/0002-renumbered.md", 1, "archdoc withdraw RFC-0002")
	expect(t, got, "M04", "rfc/0003-draft.md", 0, "")
	expect(t, got, "M04", "rfc/0004-accepted.md", 0, "")
	// With no identifier there is nothing to withdraw by; L02 named it.
	expect(t, got, "M04", "rfc/misnamed.md", 0, "")
	expect(t, got, "L11", "rfc/0004-accepted.md", 1, "deleted")
}

func TestTheBaseIsWhereTheBranchLeftTheConfiguredBranch(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-deleted.md": rfc("RFC-0001", "Deleted", "proposed", "", "On the base."),
	})
	run(t, dir, "checkout", "-q", "-b", "feature")
	remove(t, dir, "rfc/0001-deleted.md")
	commit(t, dir, "feature")
	run(t, dir, "checkout", "-q", "main")
	write(t, dir, "rfc/0002-later.md", rfc("RFC-0002", "Later", "proposed", "", "Merged after the branch was cut."))
	commit(t, dir, "later")
	run(t, dir, "checkout", "-q", "feature")

	got := mergeLint(t, dir)
	expect(t, got, "M04", "rfc/0001-deleted.md", 1, "archdoc withdraw RFC-0001")
	expect(t, got, "M04", "rfc/0002-later.md", 0, "")
}

func TestTheMergeRulesRunOnlyWithABase(t *testing.T) {
	dir := gitFixture(t, nil)
	write(t, dir, "rfc/0001-new-draft.md", rfc("RFC-0001", "New draft", "draft", "", "Never proposed."))

	for _, f := range lintDir(t, dir) {
		if strings.HasPrefix(f.Rule, "M") {
			t.Errorf("lint without a base ran %s: %s", f.Rule, f.Message)
		}
	}
}

func TestReadBaseRefusesAMissingBranch(t *testing.T) {
	dir := gitFixture(t, nil)
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"trunk"}`)
	c, err := config.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := repo.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	g, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := changes.ReadBase(r, g); !errors.Is(err, changes.ErrNoBase) || !strings.Contains(err.Error(), `"trunk"`) {
		t.Errorf("ReadBase on a missing branch = %v, want ErrNoBase naming it", err)
	}
	if _, err := changes.ReadBase(r, nil); !errors.Is(err, changes.ErrNoBase) {
		t.Errorf("ReadBase outside git = %v, want ErrNoBase", err)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commit(t *testing.T, dir, message string) {
	t.Helper()
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", message)
}

func TestReadBaseNamesAShallowCloneWhereHEADSharesNoHistory(t *testing.T) {
	dir := gitFixture(t, nil)
	run(t, dir, "checkout", "-q", "--orphan", "unrelated")
	commit(t, dir, "no shared history")
	c, err := config.Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := repo.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	g, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := changes.ReadBase(r, g); !errors.Is(err, changes.ErrNoBase) || !strings.Contains(err.Error(), "fetch-depth: 0") {
		t.Errorf("ReadBase with no shared history = %v, want ErrNoBase naming fetch-depth: 0", err)
	}
}
