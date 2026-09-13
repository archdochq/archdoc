package lint_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ollieread/archdoc/internal/config"
	"github.com/ollieread/archdoc/internal/git"
	"github.com/ollieread/archdoc/internal/link"
	"github.com/ollieread/archdoc/internal/lint"
	"github.com/ollieread/archdoc/internal/repo"
	"github.com/ollieread/archdoc/internal/repotest"
)

// rfc renders a complete RFC, so that the only rule under test is the one the
// case is built for.
func rfc(id, title, status, decided, proposal string) string {
	return fmt.Sprintf(`---
id: %s
title: %s
status: %s
created: 2026-01-01
decided:%s
depends: []
updates: []
obsoletes: []
---

# %s: %s

## Abstract

A document used to test the frozen-document rules.

## Motivation

To exercise L11 and L14 together.

## Proposal

%s

## Alternatives considered

None.

## Backwards compatibility

Nothing.

## Open questions

## Changelog

- 2026-01-01: written.
`, id, title, status, decided, id, title, proposal)
}

// gitFixture builds a real repository holding a small spec repository, commits
// it, and returns the directory.
func gitFixture(t *testing.T, files map[string]string) string {
	t.Helper()
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

	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"main"}`)
	for path, contents := range files {
		write(t, dir, path, contents)
	}
	run("add", "-A")
	run("commit", "-m", "initial")
	return dir
}

func write(t *testing.T, dir, path, contents string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// lintDir runs every rule over a spec repository on disk, with git. It does not
// use repotest, because the repository must be a real git checkout.
func lintDir(t *testing.T, dir string) []lint.Finding {
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
	return lint.Run(lint.NewContext(r, g, now))
}

// findingsFor returns the findings of one rule against one path.
func findingsFor(findings []lint.Finding, code, path string) []lint.Finding {
	var out []lint.Finding
	for _, f := range findings {
		if f.Rule == code && f.Path == path {
			out = append(out, f)
		}
	}
	return out
}

func TestL11ReportsAFrozenDocumentThatChanged(t *testing.T) {
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The original design."),
	})
	write(t, dir, path, rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "An edit after freezing."))

	got := findingsFor(lintDir(t, dir), "L11", path)
	if len(got) != 1 {
		t.Fatalf("want one L11 finding, got %d: %v", len(got), got)
	}
	if got[0].Severity != lint.Error {
		t.Errorf("severity = %q, want %q", got[0].Severity, lint.Error)
	}
}

func TestL11IsSilentForAnUnchangedFrozenDocument(t *testing.T) {
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The original design."),
	})

	if got := findingsFor(lintDir(t, dir), "L11", path); len(got) != 0 {
		t.Errorf("L11 fired on an unchanged frozen document: %v", got)
	}
}

func TestL11IsSilentForADocumentNotYetOnTheBranch(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-frozen.md": rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "Committed."),
	})
	// A document that exists only in the working tree is new, not frozen.
	write(t, dir, "rfc/0002-new.md", rfc("RFC-0002", "New", "draft", "", "Uncommitted."))

	if got := findingsFor(lintDir(t, dir), "L11", "rfc/0002-new.md"); len(got) != 0 {
		t.Errorf("L11 fired on a document absent from the branch: %v", got)
	}
}

func TestL11ReportsAFrozenDocumentDeletedFromTheWorkingTree(t *testing.T) {
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "Committed."),
	})
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}

	got := findingsFor(lintDir(t, dir), "L11", path)
	if len(got) != 1 {
		t.Fatalf("want one L11 finding for the deletion, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "deleted") {
		t.Errorf("message = %q, want it to say the document was deleted", got[0].Message)
	}
}

func TestL11ReportsABranchThatDoesNotExist(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-frozen.md": rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "Committed."),
	})
	write(t, dir, "archdoc.json", `{"name":"Frozen","branch":"trunk"}`)

	var got []lint.Finding
	for _, f := range lintDir(t, dir) {
		if f.Rule == "L11" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one L11 finding naming the branch, got %d: %v", len(got), got)
	}
	if got[0].Severity != lint.Error || !strings.Contains(got[0].Message, "trunk") {
		t.Errorf("finding = %+v, want an error naming trunk", got[0])
	}
}

func TestL11SkipsWithAWarningOutsideARepository(t *testing.T) {
	var got []lint.Finding
	for _, f := range runLint(t, "repo") { // the fixture is not a git repository
		if f.Rule == "L11" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one L11 finding, got %d: %v", len(got), got)
	}
	if got[0].Severity != lint.Warning {
		t.Errorf("severity = %q, want %q: the rule is skipped, not violated", got[0].Severity, lint.Warning)
	}
}

// The interaction the two rules were designed around: a document reaching a
// terminal status is committed once, and that commit is the last point at which
// it can be corrected.
func TestL14IsAnErrorWhileFreezingAndAWarningOnceFrozen(t *testing.T) {
	const freezing = "rfc/0001-freezing.md"
	const frozen = "rfc/0002-frozen.md"
	dir := gitFixture(t, map[string]string{
		// Proposed on the branch, so not yet frozen.
		freezing: rfc("RFC-0001", "Freezing", "proposed", "", "<!-- the design -->"),
		// Already accepted on the branch, so no edit can clear the fault.
		frozen: rfc("RFC-0002", "Frozen", "accepted", " 2026-02-01", "<!-- the design -->"),
	})
	// The working tree accepts the first: this is the freezing commit.
	write(t, dir, freezing, rfc("RFC-0001", "Freezing", "accepted", " 2026-02-01", "<!-- the design -->"))

	findings := lintDir(t, dir)

	freezingFindings := findingsFor(findings, "L14", freezing)
	if len(freezingFindings) != 1 {
		t.Fatalf("want one L14 finding while freezing, got %d: %v", len(freezingFindings), freezingFindings)
	}
	if freezingFindings[0].Severity != lint.Error {
		t.Errorf("freezing severity = %q, want %q: the fault is still fixable",
			freezingFindings[0].Severity, lint.Error)
	}

	frozenFindings := findingsFor(findings, "L14", frozen)
	if len(frozenFindings) != 1 {
		t.Fatalf("want one L14 finding once frozen, got %d: %v", len(frozenFindings), frozenFindings)
	}
	if frozenFindings[0].Severity != lint.Warning {
		t.Errorf("frozen severity = %q, want %q: L11 forbids the edit that would clear it",
			frozenFindings[0].Severity, lint.Warning)
	}
}

// stubGit fails every read, standing in for a repository that has become
// unreadable partway through a run.
type stubGit struct{ err error }

func (s stubGit) FileAt(string, string) ([]byte, bool, error) { return nil, false, s.err }
func (s stubGit) BranchExists(string) (bool, error)           { return true, nil }

func (s stubGit) HasCommits() (bool, error) { return true, nil }

func (s stubGit) Changed(string, []string) (map[string]bool, error) { return nil, s.err }

func (s stubGit) FilesAt(string, []string) (map[string][]byte, error) { return nil, s.err }

// ListFiles succeeds, so only the FileAt path is under test.
func (s stubGit) ListFiles(string) ([]string, error) { return nil, nil }
func (s stubGit) CurrentBranch() (string, error)     { return "main", nil }
func (s stubGit) RepoRoot() (string, error)          { return "/", nil }

func (s stubGit) Prefix() (string, error) { return "", nil }

func TestL11ReportsAGitFailureRatherThanTreatingItAsUnfrozen(t *testing.T) {
	findings := lint.Run(lint.NewContext(
		repotest.Fixture(t, "repo"),
		stubGit{err: errors.New("object store unreadable")},
		now,
	))

	// Against a document's own path, not against ".": several error paths in
	// this rule say "unreadable", so a test that only matches the message is
	// satisfied by whichever one happens to fire. This one pins the path that
	// records a failed branch read against each document, which is the one that
	// must not be mistaken for "this document is new and therefore editable".
	var reported bool
	for _, f := range findings {
		if f.Rule == "L11" && f.Severity == lint.Error && f.Path != "." &&
			strings.Contains(f.Message, "unreadable") {
			reported = true
		}
	}
	if !reported {
		t.Errorf("a failed branch read was not reported against the documents; L11 said %v", findings)
	}
}

// failingList reads fine but cannot enumerate the branch.
type failingList struct{ stubGit }

func (failingList) ListFiles(string) ([]string, error) {
	return nil, errors.New("ls-tree exploded")
}
func (failingList) FilesAt(string, []string) (map[string][]byte, error) { return nil, nil }
func (failingList) Changed(string, []string) (map[string]bool, error)   { return nil, nil }

func TestL11ReportsAFailureToEnumerateTheBranch(t *testing.T) {
	// The branch listing is the only way L11 can see a frozen document deleted
	// from the working tree. Without the error check a failing ls-tree yields an
	// empty listing and the deletion passes in silence.
	findings := lint.Run(lint.NewContext(repotest.Fixture(t, "repo"), failingList{}, now))

	var reported bool
	for _, f := range findings {
		if f.Rule == "L11" && f.Severity == lint.Error && strings.Contains(f.Message, "ls-tree exploded") {
			reported = true
		}
	}
	if !reported {
		t.Errorf("a failed branch listing was swallowed; L11 said %v", findings)
	}
}

// failingChanged reads the branch fine, and reports a frozen document, but
// cannot answer whether it differs from the working tree.
type failingChanged struct{ stubGit }

func (failingChanged) ListFiles(string) ([]string, error) { return nil, nil }
func (failingChanged) FilesAt(_ string, paths []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, p := range paths {
		out[p] = []byte("---\nstatus: accepted\n---\n")
	}
	return out, nil
}
func (failingChanged) Changed(string, []string) (map[string]bool, error) {
	return nil, errors.New("diff exploded")
}

func TestL11ReportsAFailureToAskWhetherADocumentChanged(t *testing.T) {
	// Changed passes every frozen path as a command-line argument, so on a large
	// repository it can fail with E2BIG, and it can fail for any other git
	// reason. Without the error check the map is nil and every frozen document
	// reads as unmodified.
	findings := lint.Run(lint.NewContext(repotest.Fixture(t, "repo"), failingChanged{}, now))

	var reported bool
	for _, f := range findings {
		if f.Rule == "L11" && f.Severity == lint.Error && strings.Contains(f.Message, "diff exploded") {
			reported = true
		}
	}
	if !reported {
		t.Errorf("a failed change check was swallowed; L11 said %v", findings)
	}
}

func TestL11FindsADeletionWhenArchdocJSONIsBelowTheGitRoot(t *testing.T) {
	// A spec repository inside a code repository is an explicitly supported
	// layout, and the generated workflow runs archdoc from exactly there.
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

	const nested = "docs"
	write(t, dir, nested+"/archdoc.json", `{"name":"Nested","branch":"main"}`)
	write(t, dir, nested+"/rfc/0001-frozen.md", rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "Committed."))
	run("add", "-A")
	run("commit", "-m", "initial")

	if err := os.Remove(filepath.Join(dir, nested, "rfc", "0001-frozen.md")); err != nil {
		t.Fatal(err)
	}

	got := findingsFor(lintDir(t, filepath.Join(dir, nested)), "L11", "rfc/0001-frozen.md")
	if len(got) != 1 {
		t.Fatalf("want one L11 finding for the deletion, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "deleted") {
		t.Errorf("message = %q, want it to say the document was deleted", got[0].Message)
	}
}

func TestL11IsSilentForANonTerminalDocumentDeletedFromTheWorkingTree(t *testing.T) {
	// The rule is about frozen documents. A draft may be deleted freely, and
	// nothing pinned that: both deletions in the suite remove an accepted
	// document, so reducing the guard to "if !ok { continue }" left it green
	// while making every deleted draft an error.
	const path = "rfc/0001-draft.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Draft", "draft", "", "Still being written."),
	})
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}

	if got := findingsFor(lintDir(t, dir), "L11", path); len(got) != 0 {
		t.Errorf("L11 fired on a deleted draft: %v", got)
	}
}

func TestL11IsSilentWhenGitItselfSaysTheFileIsUnchanged(t *testing.T) {
	// L11 compared the raw stored blob against the file on disk. git applies
	// checkout filters between the two, so wherever .gitattributes or
	// core.autocrlf normalises line endings the bytes differ legitimately and
	// every frozen document was reported as modified. On Git for Windows that
	// is the default configuration, so the rule the tool exists to enforce
	// failed on every document in every repository there.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		".gitattributes": "*.md text eol=crlf\n",
		path:             rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The design."),
	})
	// Re-checkout so the filter is applied, as a fresh clone would.
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
	run := exec.Command("git", "checkout", "--", path)
	run.Dir = dir
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git checkout: %v\n%s", err, out)
	}

	status := exec.Command("git", "status", "--porcelain")
	status.Dir = dir
	if out, _ := status.Output(); len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("the fixture is not clean, so this tests nothing: %s", out)
	}

	if got := findingsFor(lintDir(t, dir), "L11", path); len(got) != 0 {
		t.Errorf("git reports the tree clean but L11 reported: %v", got)
	}
}

func TestL11StillReportsARealEditUnderACheckoutFilter(t *testing.T) {
	// The other side: normalisation must not become a way to hide a change.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		".gitattributes": "*.md text eol=crlf\n",
		path:             rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The design."),
	})
	write(t, dir, path, rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "An edit after freezing."))

	if got := findingsFor(lintDir(t, dir), "L11", path); len(got) != 1 {
		t.Errorf("want one L11 finding for a real edit, got %d: %v", len(got), got)
	}
}

func TestL11SkipsWithAWarningBeforeTheFirstCommit(t *testing.T) {
	// `archdoc init` then `archdoc lint` is the first thing anyone does, and
	// until something is committed the branch does not exist. Nothing can be
	// frozen yet, so there is nothing for L11 to miss and no reason to fail.
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.invalid"},
		{"config", "user.name", "Fixture"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write(t, dir, "archdoc.json", `{"name":"Fresh","branch":"main"}`)
	write(t, dir, "rfc/0001-new.md", rfc("RFC-0001", "New", "draft", "", "Being written."))

	var got []lint.Finding
	for _, f := range lintDir(t, dir) {
		if f.Rule == "L11" {
			got = append(got, f)
		}
	}

	if len(got) != 1 {
		t.Fatalf("want one L11 finding, got %d: %v", len(got), got)
	}
	if got[0].Severity != lint.Warning {
		t.Errorf("severity = %q, want %q: nothing is committed, so nothing can be frozen",
			got[0].Severity, lint.Warning)
	}
	if !strings.Contains(got[0].Message, "commit") {
		t.Errorf("message = %q, want it to say why it was skipped", got[0].Message)
	}
}

func TestL11FindsAChangeWhenArchdocJSONIsBelowTheGitRoot(t *testing.T) {
	// The layout `archdoc init` scaffolds in a subdirectory, and the one its own
	// generated workflow runs from with working-directory. git resolves a
	// pathspec against the process's working directory, and L11 hands it paths
	// that are relative to the repository root, so every pathspec matched
	// nothing and every frozen document passed. The whole freeze check was off,
	// silently, at exit 0.
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

	const nested = "docs"
	write(t, dir, nested+"/archdoc.json", `{"name":"Nested","branch":"main"}`)
	write(t, dir, nested+"/rfc/0001-frozen.md",
		rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The original design."))
	run("add", "-A")
	run("commit", "-m", "initial")

	write(t, dir, nested+"/rfc/0001-frozen.md",
		rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "An edit after freezing."))

	got := findingsFor(lintDir(t, filepath.Join(dir, nested)), "L11", "rfc/0001-frozen.md")
	if len(got) != 1 {
		t.Fatalf("want one L11 finding for the edit, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "changed") {
		t.Errorf("message = %q, want it to say the document changed", got[0].Message)
	}
}

func TestL11IsQuietForAnUnchangedNestedDocument(t *testing.T) {
	// The other side, so the fix cannot be "report everything".
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

	const nested = "docs"
	write(t, dir, nested+"/archdoc.json", `{"name":"Nested","branch":"main"}`)
	write(t, dir, nested+"/rfc/0001-frozen.md",
		rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The original design."))
	run("add", "-A")
	run("commit", "-m", "initial")

	if got := findingsFor(lintDir(t, filepath.Join(dir, nested)), "L11", "rfc/0001-frozen.md"); len(got) != 0 {
		t.Errorf("L11 fired on an unchanged nested document: %v", got)
	}
}

// resolveOnBranch runs link.Resolve against a real repository so that the
// frozen softening, which needs a branch snapshot, is actually exercised.
func TestResolveSoftensToAWarningOnceADocumentIsFrozen(t *testing.T) {
	// docs/ARCHDOC.md: a frozen document containing [[...]] produces a warning, not
	// an error, because L11 forbids the edit that would clear it. Only the
	// frozen branch of that choice was unpinned, and without it `archdoc link`
	// exits 2 forever on such a repository and the workflow can never go green.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "See [[RFC-9999]]."),
	})

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

	_, findings := link.Resolve(lint.NewContext(r, g, now))
	if len(findings) != 1 {
		t.Fatalf("want one finding, got %d: %v", len(findings), findings)
	}
	if findings[0].Severity != lint.Warning {
		t.Errorf("severity = %q, want %q: the document is frozen, so no permitted edit clears it",
			findings[0].Severity, lint.Warning)
	}
}

func TestL11IgnoresTheUsersDiffRelativeSetting(t *testing.T) {
	// git's diff.relative makes --name-only print paths relative to the working
	// directory. L11 hands git repository-root-relative pathspecs and matches
	// the output against the same, so a user with that set in ~/.gitconfig lost
	// the whole change check whenever archdoc.json sat below the git root, at
	// exit 0 and in silence. The :(top) prefix fixed the input side of that
	// assumption; this is the output side.
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
	run("config", "diff.relative", "true")

	const nested = "docs"
	write(t, dir, nested+"/archdoc.json", `{"name":"Nested","branch":"main"}`)
	write(t, dir, nested+"/rfc/0001-frozen.md",
		rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The original design."))
	run("add", "-A")
	run("commit", "-m", "initial")

	write(t, dir, nested+"/rfc/0001-frozen.md",
		rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "An edit after freezing."))

	if got := findingsFor(lintDir(t, filepath.Join(dir, nested)), "L11", "rfc/0001-frozen.md"); len(got) != 1 {
		t.Errorf("want one L11 finding despite diff.relative, got %d: %v", len(got), got)
	}
}

func TestL17SoftensToAWarningOnAFrozenDocument(t *testing.T) {
	// A broken link in a frozen document cannot be repaired: L11 forbids the
	// edit. Reported as an error it makes the workflow permanently red with no
	// remedy, which is the whole reason severityFor exists.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "See [the note](nowhere.md)."),
	})

	got := findingsFor(lintDir(t, dir), "L17", path)
	if len(got) != 1 {
		t.Fatalf("want one L17 finding, got %d: %v", len(got), got)
	}
	if got[0].Severity != lint.Warning {
		t.Errorf("severity = %q, want %q on a document frozen on the branch", got[0].Severity, lint.Warning)
	}
}

func TestL11DoesNotFreezeATypeWithNoLifecycle(t *testing.T) {
	// StatusIn reads a status key out of any committed blob, so a spec page or
	// a ref carrying one, which its schema does not allow and L01 reports
	// separately, was treated as frozen. Every later edit to it was then an
	// L11 error, on a document type that has no lifecycle at all.
	const path = "spec/page.md"
	dir := gitFixture(t, map[string]string{
		path: "---\ntitle: Page\nincludes: []\nstatus: accepted\n---\n\n# Page\n\nProse.\n",
	})
	write(t, dir, path, "---\ntitle: Page\nincludes: []\nstatus: accepted\n---\n\n# Page\n\nProse, edited.\n")

	if got := findingsFor(lintDir(t, dir), "L11", path); len(got) != 0 {
		t.Errorf("L11 froze a spec page, which has no lifecycle: %v", got)
	}
}

func TestL16SoftensToAWarningOnAFrozenDocument(t *testing.T) {
	// `archdoc withdraw` performs no content checks, so a document can freeze
	// still holding [[...]]. L11 then forbids the edit that would resolve it,
	// which is exactly the case the softening exists for. link.Resolve's
	// matching softening was pinned; L16's own call site was not.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "withdrawn", " 2026-02-01", "See [[RFC-9999]]."),
	})

	got := findingsFor(lintDir(t, dir), "L16", path)
	if len(got) != 1 {
		t.Fatalf("want one L16 finding, got %d: %v", len(got), got)
	}
	if got[0].Severity != lint.Warning {
		t.Errorf("severity = %q, want %q on a document frozen on the branch", got[0].Severity, lint.Warning)
	}
}
