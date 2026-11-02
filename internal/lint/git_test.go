package lint_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/link"
	"archdoc.dev/internal/lint"
	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/repotest"
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

func (s stubGit) BranchExists(string) (bool, error) { return true, nil }

func (s stubGit) HasCommits() (bool, error) { return true, nil }

func (s stubGit) Changed(string, []string) (map[string]bool, error) { return nil, s.err }

func (s stubGit) FilesAt(string, []string) (map[string][]byte, error) { return nil, s.err }

func (s stubGit) Differs(string, map[string]string) (map[string]bool, error) { return nil, s.err }

// ListFiles succeeds, so the failure under test is the blob read, not the listing.
func (s stubGit) ListFiles(string) ([]string, error) { return nil, nil }

// MergeBase gives a base unless the stub fails, so that the stubs embedding
// this one reach the failure they stand for rather than stopping at a branch
// that shares no history.
func (s stubGit) MergeBase(string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "base", nil
}
func (s stubGit) CurrentBranch() (string, error) { return "main", nil }
func (s stubGit) RepoRoot() (string, error)      { return "/", nil }

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

func TestL11IsSilentForADeletedGroupCarryingAStatus(t *testing.T) {
	// A group cannot be frozen. One committed with a stray status key, which
	// L01 reports, was reported as frozen and deleted once removed.
	const path = "group/team.md"
	dir := gitFixture(t, map[string]string{
		path: "---\ntitle: Team\ndocuments: []\nstatus: accepted\n---\n\n# Team\n",
	})
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}

	if got := findingsFor(lintDir(t, dir), "L11", path); len(got) != 0 {
		t.Errorf("L11 fired on a deleted group: %v", got)
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

// Runs link.Resolve against a real repository so that the frozen softening,
// which needs a branch snapshot, is actually exercised.
func TestResolveSoftensToAWarningOnceADocumentIsFrozen(t *testing.T) {
	// spec/spec/links.md: a frozen document containing [[...]] produces a warning, not
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
	// exit 0 and in silence. The diff now runs from the repository root, where
	// the setting changes nothing.
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

func TestL20AndL21SoftenToWarningsOnAFrozenDocument(t *testing.T) {
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		path: rfc("RFC-0001", "Frozen", "rejected", " 2026-02-01",
			"Words.\n\n## Resolved questions\n\n### Was it?\n\nNo answer.\n\n## Changelog\n\n- Undated."),
	})
	findings := lintDir(t, dir)
	for _, code := range []string{"L20", "L21"} {
		got := findingsFor(findings, code, path)
		if len(got) == 0 {
			t.Fatalf("want %s findings on the frozen document, got none", code)
		}
		for _, f := range got {
			if f.Severity != lint.Warning {
				t.Errorf("%s at line %d is a %s, want a warning on a document frozen on the branch", code, f.Line, f.Severity)
			}
		}
	}
}

func TestFaultsNoPermittedEditCanClearAreWarningsOnAFrozenDocument(t *testing.T) {
	// L01 to L05 and L07 to L10 report faults that only an edit to the
	// document could clear, so on a frozen document they are warnings, as the
	// body rules are. Each case is one fault site in its own document, so a
	// site left reporting an error is named. A fault only a status changed
	// after freezing can produce is written to the working tree as an edit,
	// which L11 reports. A case with no rule is only a target.
	base := func(id string) string {
		return rfc(id, "Frozen", "accepted", " 2026-02-01", "Words.")
	}
	with := func(id, field, ids string) string {
		return strings.Replace(base(id), field+": []", field+": ["+ids+"]", 1)
	}
	cases := []struct {
		path, rule, message, contents string
	}{
		{"rfc/0001-unknown.md", "L01", `unknown front matter field "extra"`,
			strings.Replace(base("RFC-0001"), "depends: []", "depends: []\nextra: x", 1)},
		{"rfc/0002-duplicate.md", "L01", "obsoletes",
			strings.Replace(base("RFC-0002"), "obsoletes: []", "obsoletes: []\nobsoletes: []", 1)},
		{"rfc/0003-missing.md", "L01", `missing required front matter key "obsoletes"`,
			strings.Replace(base("RFC-0003"), "obsoletes: []\n", "", 1)},
		{"rfc/0004-empty.md", "L01", `front matter key "created" is present but empty`,
			strings.Replace(base("RFC-0004"), "created: 2026-01-01", "created:", 1)},
		{"rfc/0005-id.md", "L02", `id is "RFC-0099" but the filename gives "RFC-0005"`,
			strings.Replace(base("RFC-0005"), "id: RFC-0005", "id: RFC-0099", 1)},
		{"rfc/unnumbered.md", "L02", "filename does not match", base("RFC-0006")},
		{"rfc/0000-zero.md", "L02", "filename number is 0000", base("RFC-0000")},
		{"rfc/0008-undecided.md", "L07", "status is accepted but decided is not set",
			rfc("RFC-0008", "Frozen", "accepted", "", "Words.")},
		{"rfc/0009-backfilled.md", "L07", "backfilled 2026-01-15 is before decided 2026-02-01",
			strings.Replace(base("RFC-0009"), "depends: []", "backfilled: 2026-01-15\ndepends: []", 1)},
		{"rfc/0010-early.md", "L07", "decided 2025-12-01 is before created 2026-01-01",
			rfc("RFC-0010", "Frozen", "accepted", " 2025-12-01", "Words.")},
		{"rfc/0011-section.md", "L08", `missing required section "Motivation"`,
			strings.Replace(base("RFC-0011"), "## Motivation\n\nTo exercise L11 and L14 together.\n\n", "", 1)},
		{"rfc/0012-order.md", "L08", `section "Motivation" appears before "Abstract"`,
			strings.Replace(base("RFC-0012"), "## Abstract", "## Motivation\n\nFirst.\n\n## Abstract", 1)},
		{"rfc/0013-rationale.md", "L08", `"Rejection rationale" is present but the status is accepted`,
			strings.Replace(base("RFC-0013"), "## Open questions", "## Rejection rationale\n\nWords.\n\n## Open questions", 1)},
		{"rfc/0014-open.md", "L09", `must have an empty "Open questions" section`,
			strings.Replace(base("RFC-0014"), "## Open questions\n", "## Open questions\n\n### Is it?\n", 1)},
		{"rfc/0015-heading.md", "L10", `H1 is "RFC-0015: Other"`,
			strings.Replace(base("RFC-0015"), "# RFC-0015: Frozen", "# RFC-0015: Other", 1)},
		{"rfc/0016-no-heading.md", "L10", "the first heading must be an H1",
			strings.Replace(base("RFC-0016"), "# RFC-0016: Frozen\n", "", 1)},
		{"rfc/0017-closing.md", "L08", `"Rejection rationale" must be the last H2`,
			strings.Replace(rfc("RFC-0017", "Frozen", "rejected", " 2026-02-01", "Words."),
				"## Open questions", "## Rejection rationale\n\nWords.\n\n## Open questions", 1)},
		{"rfc/0018-status.md", "L01", `status "final" is not one of`, base("RFC-0018")},
		{"rfc/0019-reopened.md", "L07", "decided is set but status is proposed", base("RFC-0019")},
		{"rfc/0020-backfilled.md", "L07", "backfilled is set but status is proposed",
			strings.Replace(base("RFC-0020"), "depends: []", "backfilled: 2026-03-01\ndepends: []", 1)},
		{"ref/0001-notes.md", "", "", "---\nid: REF-0001\ntitle: Notes\nverified: 2026-08-01\n---\n\n# REF-0001: Notes\n\nNotes.\n\n## Sources\n\n- None.\n"},
		{"rfc/0021-ref.md", "L04", "depends references REF-0001, but only RFCs and ADRs are permitted there",
			with("RFC-0021", "depends", "REF-0001")},
		{"rfc/0022-missing.md", "L04", "depends references RFC-0999, which does not exist",
			with("RFC-0022", "depends", "RFC-0999")},
		{"rfc/0023-itself.md", "L05", "RFC-0023 names itself in updates",
			with("RFC-0023", "updates", "RFC-0023")},
		{"rfc/0024-both.md", "L05", "RFC-0001 appears in both updates and obsoletes",
			strings.Replace(with("RFC-0024", "updates", "RFC-0001"), "obsoletes: []", "obsoletes: [RFC-0001]", 1)},
		{"rfc/0025-type.md", "L05", "updates references REF-0001, which is a ref",
			with("RFC-0025", "updates", "REF-0001")},
		{"rfc/0026-rejected.md", "L05", "updates references RFC-0017, which is rejected",
			with("RFC-0026", "updates", "RFC-0017")},
		{"rfc/0027-fresh-target.md", "L05", "updates references RFC-0028, which is rejected",
			with("RFC-0027", "updates", "RFC-0028")},
		{"rfc/0028-rejected.md", "", "",
			strings.Replace(rfc("RFC-0028", "Fresh", "rejected", " 2026-02-01", "Words."),
				"## Open questions\n\n## Changelog", "## Open questions\n\n## Rejection rationale\n\nWords.\n\n## Changelog", 1)},
		{"rfc/0029-fresh-updates.md", "L05", "updates references RFC-0017, which is rejected",
			with("RFC-0029", "updates", "RFC-0017")},
		{"rfc/0030-one.md", "L03", "identifier RFC-0030 is also used by rfc/0030-two.md",
			base("RFC-0030")},
		{"rfc/0030-two.md", "", "", base("RFC-0030")},
		{"rfc/0031-held.md", "L03", "identifier RFC-0031 is also used by rfc/0031-new.md",
			base("RFC-0031")},
		{"rfc/0031-new.md", "", "", base("RFC-0031")},
	}
	// Fresh documents are never committed, and a fault in a case that stays
	// an error is one some permitted edit can clear.
	isFresh := map[string]bool{"rfc/0028-rejected.md": true, "rfc/0029-fresh-updates.md": true, "rfc/0031-new.md": true}
	stays := map[string]bool{"rfc/0022-missing.md": true, "rfc/0027-fresh-target.md": true, "rfc/0031-held.md": true}
	files, fresh := map[string]string{}, map[string]string{}
	for _, c := range cases {
		if isFresh[c.path] {
			fresh[c.path] = c.contents
		} else {
			files[c.path] = c.contents
		}
	}
	// The status each of the last three is given after freezing.
	edits := map[string]string{
		"rfc/0018-status.md":     "final",
		"rfc/0019-reopened.md":   "proposed",
		"rfc/0020-backfilled.md": "proposed",
	}

	for _, frozen := range []bool{true, false} {
		var dir string
		want := lint.Warning
		if frozen {
			dir = gitFixture(t, files)
		} else {
			// Committed with nothing, so every document is new and editable.
			dir = gitFixture(t, nil)
			for path, contents := range files {
				write(t, dir, path, contents)
			}
			want = lint.Error
		}
		for path, contents := range fresh {
			write(t, dir, path, contents)
		}
		for path, status := range edits {
			write(t, dir, path, strings.Replace(files[path], "status: accepted", "status: "+status, 1))
		}
		findings := lintDir(t, dir)
		for _, c := range cases {
			if c.rule == "" {
				continue
			}
			want := want
			if isFresh[c.path] || stays[c.path] {
				want = lint.Error
			}
			var found bool
			for _, f := range findingsFor(findings, c.rule, c.path) {
				if !strings.Contains(f.Message, c.message) {
					continue
				}
				found = true
				if f.Severity != want {
					t.Errorf("%s on %s (frozen %t) is a %s, want a %s: %s", c.rule, c.path, frozen, f.Severity, want, f.Message)
				}
			}
			if !found {
				t.Errorf("%s reported nothing on %s (frozen %t) matching %q", c.rule, c.path, frozen, c.message)
			}
		}
	}
}

func TestL11ReadsARootBeginningWithAColon(t *testing.T) {
	// Read as a pathspec, ":spec/..." is magic and matches nothing, so no
	// frozen document read as changed or deleted. An inherited pathspec
	// setting must change nothing either.
	for _, inherited := range []string{"", "GIT_GLOB_PATHSPECS", "GIT_ICASE_PATHSPECS"} {
		t.Run("inheriting "+inherited, func(t *testing.T) {
			const edited, deleted = ":spec/rfc/0001-edited.md", ":spec/rfc/0002-deleted.md"
			dir := gitFixture(t, map[string]string{
				"archdoc.json": `{"name":"Frozen","branch":"main","root":":spec"}`,
				edited:         rfc("RFC-0001", "Edited", "accepted", " 2026-02-01", "The design."),
				deleted:        rfc("RFC-0002", "Deleted", "accepted", " 2026-02-01", "The design."),
			})
			write(t, dir, edited, rfc("RFC-0001", "Edited", "accepted", " 2026-02-01", "An edit after freezing."))
			remove(t, dir, deleted)
			if inherited != "" {
				t.Setenv(inherited, "1")
			}

			findings := lintDir(t, dir)
			for _, path := range []string{"rfc/0001-edited.md", "rfc/0002-deleted.md"} {
				if got := findingsFor(findings, "L11", path); len(got) != 1 {
					t.Errorf("want one L11 finding for %s, got %v", path, got)
				}
			}
		})
	}
}

func TestL11ReportsAnEditFromAHookInALinkedWorktree(t *testing.T) {
	// git runs a hook in a linked worktree with GIT_DIR set and no
	// GIT_WORK_TREE, so it takes the working directory as the top level.
	// archdoc.json sits below the top level: at the top, the working
	// directory git falls back to is the right one anyway.
	const path = "rfc/0001-frozen.md"
	dir := gitFixture(t, map[string]string{
		"spec/archdoc.json": `{"name":"Frozen","branch":"main"}`,
		"spec/" + path:      rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "The design."),
	})
	wt := filepath.Join(t.TempDir(), "wt")
	run(t, dir, "worktree", "add", "-q", "-b", "side", wt)
	write(t, wt, "spec/"+path, rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "An edit after freezing."))
	t.Setenv("GIT_DIR", filepath.Join(dir, ".git", "worktrees", "wt"))
	t.Chdir(filepath.Join(wt, "spec", "rfc"))

	if got := findingsFor(lintDir(t, filepath.Join(wt, "spec")), "L11", path); len(got) != 1 {
		t.Errorf("want one L11 finding from the hook, got %v", got)
	}
}

// cutBranch commits files on main, cuts feature from it, commits onFeature
// there, then commits onMain on main after the cut and checks feature out.
// Empty contents delete the path.
func cutBranch(t *testing.T, files, onFeature, onMain map[string]string) string {
	t.Helper()
	dir := gitFixture(t, files)
	apply := func(changes map[string]string, message string) {
		for path, contents := range changes {
			if contents == "" {
				remove(t, dir, path)
				continue
			}
			write(t, dir, path, contents)
		}
		commit(t, dir, message)
	}
	run(t, dir, "checkout", "-q", "-b", "feature")
	apply(onFeature, "feature")
	run(t, dir, "checkout", "-q", "main")
	apply(onMain, "after the cut")
	run(t, dir, "checkout", "-q", "feature")
	return dir
}

func TestL11IsSilentForWhatTheBranchFrozeAfterItWasCut(t *testing.T) {
	const older, newer = "rfc/0001-older.md", "rfc/0002-newer.md"
	dir := cutBranch(t,
		map[string]string{older: rfc("RFC-0001", "Older", "proposed", "", "Proposed at the cut.")},
		map[string]string{"rfc/0003-feature.md": rfc("RFC-0003", "Feature", "proposed", "", "The branch's own.")},
		map[string]string{
			older: rfc("RFC-0001", "Older", "accepted", " 2026-02-01", "Proposed at the cut."),
			newer: rfc("RFC-0002", "Newer", "accepted", " 2026-02-01", "Added after the cut."),
		},
	)

	got := lintDir(t, dir)
	for _, path := range []string{older, newer} {
		if f := findingsFor(got, "L11", path); len(f) != 0 {
			t.Errorf("L11 reported %s, which the branch never touched: %v", path, f)
		}
	}
}

func TestL11StillReportsWhatABranchChangesOrDeletes(t *testing.T) {
	const edited, deleted, since = "rfc/0001-edited.md", "rfc/0002-deleted.md", "rfc/0003-since.md"
	dir := cutBranch(t,
		map[string]string{
			edited:  rfc("RFC-0001", "Edited", "accepted", " 2026-02-01", "Frozen at the cut."),
			deleted: rfc("RFC-0002", "Deleted", "accepted", " 2026-02-01", "Frozen at the cut."),
			since:   rfc("RFC-0003", "Since", "proposed", "", "Proposed at the cut."),
		},
		map[string]string{
			edited:  rfc("RFC-0001", "Edited", "accepted", " 2026-02-01", "Edited on the branch."),
			deleted: "",
			since:   rfc("RFC-0003", "Since", "proposed", "", "Edited on the branch."),
		},
		// Accepted on main after the cut, so merging the branch's edit would
		// change a frozen document.
		map[string]string{since: rfc("RFC-0003", "Since", "accepted", " 2026-02-01", "Proposed at the cut.")},
	)

	got := lintDir(t, dir)
	expect(t, got, "L11", edited, 1, "changed since it was frozen on main")
	expect(t, got, "L11", deleted, 1, "frozen on main but deleted from the working tree")
	expect(t, got, "L11", since, 1, "changed since it was frozen on main")
}

func TestL11NamesAShallowCloneWhereHEADSharesNoHistoryWithTheBranch(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"rfc/0001-frozen.md": rfc("RFC-0001", "Frozen", "accepted", " 2026-02-01", "Committed."),
	})
	run(t, dir, "checkout", "-q", "--orphan", "unrelated")
	run(t, dir, "commit", "-q", "-m", "no shared history")

	expect(t, lintDir(t, dir), "L11", ".", 1, "a shallow clone needs its full history (fetch-depth: 0)")
}

// failingMergeBase reads the branch fine but cannot find where HEAD left it.
type failingMergeBase struct{ stubGit }

func (failingMergeBase) MergeBase(string) (string, error) {
	return "", errors.New("merge-base exploded")
}

func TestL11ReportsAFailureToFindTheMergeBase(t *testing.T) {
	findings := lint.Run(lint.NewContext(repotest.Fixture(t, "repo"), failingMergeBase{}, now))

	expect(t, findings, "L11", ".", 1, "merge-base exploded")
}

// failingDeletion lists a frozen document the working tree does not have on the
// branch, and cannot list the base to ask whether it had it.
type failingDeletion struct{ stubGit }

const goneDocument = "rfc/0099-gone.md"

func (failingDeletion) ListFiles(rev string) ([]string, error) {
	if rev == "base" {
		return nil, errors.New("listing the base exploded")
	}
	return []string{goneDocument}, nil
}
func (failingDeletion) FilesAt(_ string, paths []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, p := range paths {
		out[p] = []byte("---\nstatus: accepted\n---\n")
	}
	return out, nil
}
func (failingDeletion) Changed(string, []string) (map[string]bool, error) { return nil, nil }

func TestL11ReportsAFailureToAskWhetherADocumentWasDeleted(t *testing.T) {
	findings := lint.Run(lint.NewContext(repotest.Fixture(t, "repo"), failingDeletion{}, now))

	expect(t, findings, "L11", ".", 1, "listing the base exploded")
}

// TestL17FollowsALinkOutOfRootIntoTheGitRepository: init writes a root below
// the git top level inside a code repository, and a link to the code was
// reported as leaving the repository.
func TestL17FollowsALinkOutOfRootIntoTheGitRepository(t *testing.T) {
	dir := gitFixture(t, map[string]string{
		"src/main.go":       "package main\n",
		"docs/archdoc.json": `{"name":"Nested","branch":"main"}`,
		"docs/spec/p.md": "---\ntitle: P\nincludes: []\n---\n\n# P\n\n" +
			"See [main](../../src/main.go), [gone](../../src/gone.go) and [out](../../../outside.md).\n\n" +
			"The [top level](../../) is the repository's, and [above it](../../../) is not.\n",
	})
	var got []string
	for _, f := range findingsFor(lintDir(t, filepath.Join(dir, "docs")), "L17", "spec/p.md") {
		got = append(got, f.Message)
	}
	want := []string{
		`link target "../../src/gone.go" does not exist`,
		`link target "../../../outside.md" leaves the repository`,
		`link target "../../../" leaves the repository`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}
