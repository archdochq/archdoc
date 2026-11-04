package lint_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/config"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/lint"
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

// TestTheBasesArchdocJSONGoneFromTheWorkingTreeIsTheMovedOne: of two on the
// base, the one still in the working tree is not the one the working tree's
// moved from. With both gone, which one moved cannot be told, so neither
// stands in for it.
func TestTheBasesArchdocJSONGoneFromTheWorkingTreeIsTheMovedOne(t *testing.T) {
	dir := gitFixture(t, nil)
	run(t, dir, "rm", "-q", "archdoc.json")
	write(t, dir, "docs/archdoc.json", `{"name":"Frozen","branch":"main"}`)
	write(t, dir, "other/archdoc.json", `{"name":"Other","branch":"main"}`)
	commit(t, dir, "two specifications")
	run(t, dir, "rm", "-q", "docs/archdoc.json")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs","strict":false}`)
	write(t, dir, "docs/rfc/0001-at-once.md", rfc("RFC-0001", "At once", "accepted", " 2026-02-01", "Accepted here."))
	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 1, "not a permitted transition")

	run(t, dir, "rm", "-q", "other/archdoc.json")
	expect(t, mergeLint(t, dir), "M03", "rfc/0001-at-once.md", 0, "")
}

// changesOf is what `archdoc changes` lists for dir.
func changesOf(t *testing.T, dir string) []changes.Change {
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
	got, err := changes.Of(r, g, base)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestChangesFollowAMovedRoot(t *testing.T) {
	dir := movedRoot(t)
	write(t, dir, "docs/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	if got := changesOf(t, dir); len(got) != 1 || got[0].Path != "rfc/0001-accepted.md" || got[0].Change != changes.Edited {
		t.Errorf("changes = %+v, want RFC-0001 edited alone", got)
	}
}

// TestAnExamplesArchdocJSONIsNotAMove: main holds an example specification
// with its own archdoc.json, and the pull request adopts ArchDoc at the top
// level beside it. Where the example's file is still there, the working tree's
// did not move from it. Where the pull request deletes it, none of the
// example's documents is under the working tree's root, so it did not move
// from it either. The example's documents are not the record's.
func TestAnExamplesArchdocJSONIsNotAMove(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		dir := gitFixture(t, map[string]string{
			"examples/archdoc.json":         `{"name":"Example","branch":"main"}`,
			"examples/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
			"examples/rfc/0002-proposed.md": rfc("RFC-0002", "Proposed", "proposed", "", "The design."),
		})
		run(t, dir, "rm", "-q", "archdoc.json")
		commit(t, dir, "an example alone")
		write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
		write(t, dir, "spec/rfc/0001-adopted.md", rfc("RFC-0001", "Adopted", "proposed", "", "The design."))
		if deleted {
			run(t, dir, "rm", "-q", "examples/archdoc.json")
		}

		for _, f := range mergeLint(t, dir) {
			if f.Severity == lint.Error {
				t.Errorf("deleted %t: %s %s: %s", deleted, f.Rule, f.Path, f.Message)
			}
		}
		if got := changesOf(t, dir); len(got) != 1 || got[0].Path != "rfc/0001-adopted.md" || got[0].Change != changes.Arrives {
			t.Errorf("deleted %t: changes = %+v, want RFC-0001 arriving alone", deleted, got)
		}
	}
}

// TestAnArchdocJSONMovedDownIsJudgedByTheBases: the file and the documents
// move from the top level into spec/, where lint runs, so the base's file is
// outside the directory git is run in.
func TestAnArchdocJSONMovedDownIsJudgedByTheBases(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
		"rfc/0002-frozen.md":   rfc("RFC-0002", "Frozen", "accepted", " 2026-02-01", "The design."),
	})
	write(t, dir, "spec/.keep", "")
	run(t, dir, "mv", "archdoc.json", "rfc", "spec/")
	write(t, dir, "spec/archdoc.json", `{"name":"Frozen","branch":"main","strict":false}`)
	write(t, dir, "spec/rfc/0003-at-once.md", rfc("RFC-0003", "At once", "accepted", " 2026-02-01", "Accepted here."))
	write(t, dir, "spec/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	remove(t, dir, "spec/rfc/0002-frozen.md")

	got := mergeLint(t, filepath.Join(dir, "spec"))
	expect(t, got, "M03", "rfc/0003-at-once.md", 1, "not a permitted transition")
	expect(t, got, "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
	expect(t, got, "L11", "rfc/0002-frozen.md", 1, "deleted from the working tree")
}

// TestARootCorrectedToItsSpellingIsFollowed: the base names its root in
// another case than git lists it, and the pull request corrects it.
func TestARootCorrectedToItsSpellingIsFollowed(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
		"spec/rfc/0002-proposed.md": rfc("RFC-0002", "Proposed", "proposed", "", "The design."),
	})
	if _, err := os.Stat(filepath.Join(dir, "SPEC")); err != nil {
		t.Skip("the filesystem is case-sensitive")
	}
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"Spec"}`)
	commit(t, dir, "root")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	write(t, dir, "spec/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	remove(t, dir, "spec/rfc/0002-proposed.md")

	got := mergeLint(t, dir)
	expect(t, got, "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
	expect(t, got, "M04", "rfc/0002-proposed.md", 1, "withdraw it instead")
}

// TestARootInAnotherNormalisationIsMatched: root is written decomposed, and
// git lists the directory composed.
func TestARootInAnotherNormalisationIsMatched(t *testing.T) {
	composed, decomposed := "sp\u00e9c", "spe\u0301c"
	dir := gitFixture(t, map[string]string{
		composed + "/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
	})
	if _, err := os.Stat(filepath.Join(dir, decomposed)); err != nil {
		t.Skip("the filesystem tells the two normalisations apart")
	}
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"`+decomposed+`"}`)
	commit(t, dir, "root")
	write(t, dir, composed+"/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
}

// TestALinkLeftWhereTheRootWasDoesNotHideTheBase: the documents move to docs/
// and a link named spec is left pointing at them, root unchanged. The base
// lists its documents under spec as written, so that is where they are read.
func TestALinkLeftWhereTheRootWasDoesNotHideTheBase(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
	})
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	commit(t, dir, "root")
	run(t, dir, "mv", "spec", "docs")
	if err := os.Symlink("docs", filepath.Join(dir, "spec")); err != nil {
		t.Skip("no symbolic links here")
	}
	write(t, dir, "docs/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
}

// TestAMovedArchdocJSONsRootIsTakenAsWritten: the base's file, under old/,
// names a root holding nothing yet, and the pull request moves it to the top
// level, where spec/ already held a document. That was never the base's
// record, so it was never frozen; git's spelling of the working tree's spec/
// is no answer for a root relative to old/.
func TestAMovedArchdocJSONsRootIsTakenAsWritten(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
	})
	run(t, dir, "mv", "archdoc.json", "spec.json")
	write(t, dir, "old/archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	run(t, dir, "rm", "-q", "--cached", "spec.json")
	remove(t, dir, "spec.json")
	commit(t, dir, "settings under old")
	run(t, dir, "rm", "-q", "old/archdoc.json")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	write(t, dir, "spec/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 0, "")
}

// TestAMovedArchdocJSONsRootIsMatchedAsGitListsIt: the base's file, at the
// top level, names its root in another case or normalisation than git lists
// it, and the pull request moves the file into the root. The match is made
// against git's listing, so it holds whether or not the filesystem tells the
// spellings apart.
func TestAMovedArchdocJSONsRootIsMatchedAsGitListsIt(t *testing.T) {
	for _, c := range []struct{ name, listed, written string }{
		{"case", "spec", "Spec"},
		{"normalisation", "sp\u00e9c", "spe\u0301c"},
		{"decomposed listing", "spe\u0301c", "sp\u00e9c"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// git on macOS composes the names it lists unless told not to,
			// which a listing made elsewhere need not be.
			dir := gitFixture(t, nil)
			run(t, dir, "config", "core.precomposeUnicode", "false")
			write(t, dir, c.listed+"/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."))
			write(t, dir, c.listed+"/rfc/0002-proposed.md", rfc("RFC-0002", "Proposed", "proposed", "", "The design."))
			write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"`+c.written+`"}`)
			commit(t, dir, "root")
			run(t, dir, "rm", "-q", "archdoc.json")
			write(t, dir, c.listed+"/archdoc.json", `{"name":"Frozen","branch":"main"}`)
			write(t, dir, c.listed+"/rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
			remove(t, dir, c.listed+"/rfc/0002-proposed.md")

			in := filepath.Join(dir, c.listed)
			got := mergeLint(t, in)
			expect(t, got, "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
			expect(t, got, "M04", "rfc/0002-proposed.md", 1, "withdraw it instead")
			listed := changesOf(t, in)
			if !slices.ContainsFunc(listed, func(ch changes.Change) bool {
				return ch.Path == "rfc/0001-accepted.md" && ch.Change == changes.Edited
			}) {
				t.Errorf("changes = %+v, want RFC-0001 edited among them", listed)
			}
		})
	}
}

// TestADeletionIsJudgedAtTheBasesRoot: main moved its root after the branch
// was cut, and the branch deletes a document frozen before the cut, which the
// merge base holds under the old root. Left as it was at the cut, it is read
// at the merge base's root and found unchanged, though the tip holds it only
// under its own.
func TestADeletionIsJudgedAtTheBasesRoot(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"spec/rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
	})
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"spec"}`)
	commit(t, dir, "root")
	run(t, dir, "checkout", "-q", "-b", "work")
	run(t, dir, "checkout", "-q", "main")
	run(t, dir, "mv", "spec", "docs")
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main","root":"docs"}`)
	commit(t, dir, "move")
	run(t, dir, "checkout", "-q", "work")
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 0, "")

	remove(t, dir, "spec/rfc/0001-accepted.md")
	expect(t, lintDir(t, dir), "L11", "rfc/0001-accepted.md", 1, "deleted from the working tree")
}

// TestAnUnreadableArchdocJSONOnTheBranchIsReportedOnce: the working tree's
// settings stand in for it, with one warning, so a pull request mending the
// branch's file can pass. It is reported whichever of the tip and the merge
// base holds it.
func TestAnUnreadableArchdocJSONOnTheBranchIsReportedOnce(t *testing.T) {
	const broken, mended = `{"name":"Frozen",`, `{"name":"Frozen","branch":"main"}`
	for _, c := range []struct {
		name        string
		atBase, tip string
	}{
		{"both", broken, broken},
		{"merge base", broken, mended},
		{"tip", mended, broken},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := gitFixture(t, map[string]string{
				"rfc/0001-accepted.md": rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "The design."),
			})
			write(t, dir, "archdoc.json", c.atBase)
			write(t, dir, "notes.txt", "at the cut\n")
			commit(t, dir, "at the cut")
			run(t, dir, "checkout", "-q", "-b", "work")
			run(t, dir, "checkout", "-q", "main")
			write(t, dir, "notes.txt", "later\n")
			write(t, dir, "archdoc.json", c.tip)
			commit(t, dir, "since")
			run(t, dir, "checkout", "-q", "work")
			write(t, dir, "archdoc.json", mended)

			got := mergeLint(t, dir)
			for _, f := range got {
				if f.Severity == lint.Error {
					t.Errorf("%s %s: %s", f.Rule, f.Path, f.Message)
				}
			}
			reported := findingsFor(got, "L11", ".")
			if len(reported) != 1 || reported[0].Severity != lint.Warning ||
				!strings.Contains(reported[0].Message, "archdoc.json on main cannot be read") {
				t.Errorf("L11 at . = %v, want one warning that archdoc.json on main cannot be read", reported)
			}

			write(t, dir, "rfc/0001-accepted.md", rfc("RFC-0001", "Accepted", "accepted", " 2026-02-01", "Rewritten."))
			expect(t, mergeLint(t, dir), "L11", "rfc/0001-accepted.md", 1, "changed since it was frozen")
		})
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
