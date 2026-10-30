package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/config"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/repo"
)

// movedRoot is a repository whose documents live under spec/, with RFC-0001
// accepted and RFC-0002 proposed on main, and a working tree that moves spec/
// to docs/ and points root at it.
func movedRoot(t *testing.T) string {
	t.Helper()
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
		"spec/rfc/0002-proposed.md": rfc("RFC-0002", "Proposed", "proposed", "", "The design."),
	})
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	commit(t, dir, "root")
	run(t, dir, "mv", "spec", "docs")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs"}`)
	return dir
}

// TestAMovedRootIsFollowed: the documents are matched by their path within
// root, so a move is allowed and an edit or deletion made with it is not.
func TestAMovedRootIsFollowed(t *testing.T) {
	dir := movedRoot(t)
	got := mergeLint(t, dir)
	expect(t, got, "L11", "rfc/0001-accepted.md", 0, "")
	expect(t, got, "M04", "rfc/0002-proposed.md", 0, "")

	write(t, dir, "docs/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	remove(t, dir, "docs/rfc/0002-proposed.md")
	got = mergeLint(t, dir)
	expect(t, got, "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
	expect(t, got, "M04", "rfc/0002-proposed.md", 1, "withdraw it instead")
}

func TestAMovedRootDeletingAFrozenDocumentIsReported(t *testing.T) {
	dir := movedRoot(t)
	remove(t, dir, "docs/rfc/0001-accepted.md")
	expect(t, mergeLint(t, dir), "L11", "rfc/0001-accepted.md", 1, "deleted from the working tree")
}

// TestAMovedArchdocJSONIsStillJudgedByTheBases: the base's only archdoc.json
// stands in for one at the working tree's location, so moving it does not
// escape its strict.
func TestAMovedArchdocJSONIsStillJudgedByTheBases(t *testing.T) {
	dir := gitFixture(t, nil)
	run(t, dir, "mv", "archdoc.json", "spec.json")
	write(t, dir, "docs/archdoc.json", `{"name":"Frozen","branch":"main"}`)
	run(t, dir, "rm", "-q", "--cached", "spec.json")
	remove(t, dir, "spec.json")
	commit(t, dir, "settings under docs")
	run(t, dir, "rm", "-q", "docs/archdoc.json")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs","strict":false}`)
	write(t, dir, "docs/rfc/0001-at-once.md", rfc("RFC-0001", "At once", "accepted", " 2026-02-01", "Accepted here."))

	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 1, "not a permitted transition")
}

// TestARootSpelledThroughALinkIsMatched: root names the documents through a
// symbolic link, and git lists them under the directory it points to.
func TestARootSpelledThroughALinkIsMatched(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
	})
	if err := os.Symlink("spec", filepath.Join(dir, "docs")); err != nil {
		t.Skip("no symbolic links here")
	}
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs"}`)
	commit(t, dir, "root")
	write(t, dir, "spec/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
}

func TestARootSpelledInAnotherCaseIsMatched(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
	})
	if _, err := os.Stat(filepath.Join(dir, "SPEC")); err != nil {
		t.Skip("the filesystem is case-sensitive")
	}
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"Spec"}`)
	commit(t, dir, "root")
	write(t, dir, "spec/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
}

// TestAStackedBranchPassesAfterItsParentIsSquashMerged: the branch holds the
// tip's content exactly, so merging changes nothing, although the merge base
// still holds the document as it was before it froze.
func TestAStackedBranchPassesAfterItsParentIsSquashMerged(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-two.md": rfc("RFC-0001", "Two", "proposed", "", "The design."),
	})
	run(t, dir, "checkout", "-q", "-b", "accept")
	write(t, dir, "rfc/0001-two.md", rfc("RFC-0001", "Two", "accepted", " 2026-02-01", "The design."))
	commit(t, dir, "accept")
	run(t, dir, "checkout", "-q", "-b", "stacked")
	write(t, dir, "notes.txt", "unrelated\n")
	commit(t, dir, "stacked")
	run(t, dir, "checkout", "-q", "main")
	run(t, dir, "merge", "-q", "--squash", "accept")
	run(t, dir, "commit", "-q", "-m", "accept (#1)")
	run(t, dir, "checkout", "-q", "stacked")

	expect(t, lintDir(t, dir), "L11", "rfc/0001-two.md", 0, "")

	write(t, dir, "rfc/0001-two.md", rfc("RFC-0001", "Two", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0001-two.md", 1, "changed since it was frozen")
}

func TestReadBaseTakesRootFromTheBase(t *testing.T) {
	dir := movedRoot(t)
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
	if _, ok := base.Documents["rfc/0001-accepted.md"]; !ok {
		var paths []string
		for p := range base.Documents {
			paths = append(paths, p)
		}
		t.Errorf("base documents = %s, want rfc/0001-accepted.md among them", strings.Join(paths, ", "))
	}
}

// TestSeveralArchdocJSONOnTheBaseLeaveTheWorkingTreesSettings: which of them
// a moved file was cannot be told, so none stands in for it.
func TestSeveralArchdocJSONOnTheBaseLeaveTheWorkingTreesSettings(t *testing.T) {
	dir := gitFixture(t, nil)
	run(t, dir, "rm", "-q", "archdoc.json")
	write(t, dir, "docs/archdoc.json", `{"name":"Frozen","branch":"main"}`)
	write(t, dir, "other/archdoc.json", `{"name":"Other","branch":"main"}`)
	commit(t, dir, "two specifications")
	run(t, dir, "rm", "-q", "docs/archdoc.json")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs","strict":false}`)
	write(t, dir, "docs/rfc/0001-at-once.md", rfc("RFC-0001", "At once", "accepted", " 2026-02-01", "Accepted here."))

	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 0, "")
}

func TestChangesFollowAMovedRoot(t *testing.T) {
	dir := movedRoot(t)
	write(t, dir, "docs/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
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
	got, err := changes.Of(r, g, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "rfc/0001-accepted.md" || got[0].Change != changes.Edited {
		t.Errorf("changes = %+v, want RFC-0001 edited alone", got)
	}
}

// movedAfterTheCut moves spec/ to docs/ on a branch, then accepts RFC-0002 on
// main, so the tip holds it frozen under spec/ and the merge base proposed.
func movedAfterTheCut(t *testing.T) string {
	t.Helper()
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0002-later.md": rfc("RFC-0002", "Later", "proposed", "", "The design."),
	})
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	commit(t, dir, "root")
	run(t, dir, "checkout", "-q", "-b", "move")
	run(t, dir, "mv", "spec", "docs")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs"}`)
	commit(t, dir, "move")
	run(t, dir, "checkout", "-q", "main")
	write(t, dir, "spec/rfc/0002-later.md", rfc("RFC-0002", "Later", "accepted", " 2026-02-01", "The design."))
	commit(t, dir, "accept")
	run(t, dir, "checkout", "-q", "move")
	return dir
}

// TestAMovedRootIsJudgedAtEachSidesOwnRoot: the merge base is read under its
// root and the tip under its own, so a document left as the branch was cut,
// and one taken exactly as the tip holds it, both pass.
func TestAMovedRootIsJudgedAtEachSidesOwnRoot(t *testing.T) {
	dir := movedAfterTheCut(t)
	expect(t, lintDir(t, dir), "L11", "rfc/0002-later.md", 0, "")

	write(t, dir, "docs/rfc/0002-later.md", rfc("RFC-0002", "Later", "accepted", " 2026-02-01", "The design."))
	expect(t, lintDir(t, dir), "L11", "rfc/0002-later.md", 0, "")

	write(t, dir, "docs/rfc/0002-later.md", rfc("RFC-0002", "Later", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0002-later.md", 1, "changed since it was frozen")
}
