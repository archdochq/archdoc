package main

import (
	"github.com/ollieread/archdoc/internal/config"
	"github.com/ollieread/archdoc/internal/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The acceptance test for the whole tool: a scaffolded repository passes the
// checks its own generated workflow runs.
func TestInitProducesARepositoryThatPassesItsOwnChecks(t *testing.T) {
	dir := t.TempDir()

	out, code := run(t, dir, "init", "--name", "TheGamePanel")
	if code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	if _, code := run(t, dir, "lint"); code != exitOK {
		got, _ := run(t, dir, "lint")
		t.Errorf("lint on a fresh repository exited %d:\n%s", code, got)
	}
	if got, code := run(t, dir, "index", "--check"); code != exitOK {
		t.Errorf("index --check on a fresh repository exited %d:\n%s", code, got)
	}
}

func TestInitWritesEverythingItPromises(t *testing.T) {
	dir := t.TempDir()

	out, code := run(t, dir, "init", "--name", "TheGamePanel", "--license", "mit")
	if code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	for _, want := range []string{
		"archdoc.json", "README.md", "PROCESS.md", "INDEX.md", "LICENSE",
		"spec/glossary.md", "rfc/.gitkeep", "adr/.gitkeep", "ref/.gitkeep",
		".github/workflows/archdoc-lint.yml",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(want))); err != nil {
			t.Errorf("%s was not written: %v", want, err)
		}
		if !strings.Contains(out, want) {
			t.Errorf("%s was written but not reported:\n%s", want, out)
		}
	}

	settings, err := os.ReadFile(filepath.Join(dir, "archdoc.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name": "TheGamePanel"`, `"branch": "main"`, `"strict": true`, `"ref_stale_days": 180`} {
		if !strings.Contains(string(settings), want) {
			t.Errorf("archdoc.json is missing %q:\n%s", want, settings)
		}
	}
	licence, _ := os.ReadFile(filepath.Join(dir, "LICENSE"))
	if !strings.Contains(string(licence), "TheGamePanel") {
		t.Errorf("the licence does not name the project:\n%s", licence)
	}
}

func TestInitDefaultsTheNameFromTheDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "thegamepanel-spec")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if out, code := run(t, dir, "init"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	settings, _ := os.ReadFile(filepath.Join(dir, "archdoc.json"))
	if !strings.Contains(string(settings), `"name": "thegamepanel"`) {
		t.Errorf("the trailing -spec was not removed:\n%s", settings)
	}
}

func TestInitRefusesToRunTwiceAndWritesNothingOnACollision(t *testing.T) {
	dir := t.TempDir()
	if _, code := run(t, dir, "init", "--name", "X"); code != exitOK {
		t.Fatal("first init failed")
	}
	if out, code := run(t, dir, "init", "--name", "X"); code != exitUsage || !strings.Contains(out, "already exists") {
		t.Errorf("second init exited %d with %q", code, out)
	}

	// A collision on any other file must leave the directory untouched.
	fresh := t.TempDir()
	if err := os.WriteFile(filepath.Join(fresh, "README.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := run(t, fresh, "init", "--name", "X")
	if code != exitUsage || !strings.Contains(out, "nothing has been written") {
		t.Errorf("exited %d with %q, want a refusal", code, out)
	}
	if _, err := os.Stat(filepath.Join(fresh, "archdoc.json")); err == nil {
		t.Error("archdoc.json was written despite the refusal")
	}
	if kept, _ := os.ReadFile(filepath.Join(fresh, "README.md")); string(kept) != "mine\n" {
		t.Errorf("the existing README was overwritten: %q", kept)
	}
}

func TestInitPutsTheWorkflowAtTheGitRootWithAWorkingDirectory(t *testing.T) {
	top := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.email", "t@e.invalid"}, {"config", "user.name", "T"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = top
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	docs := filepath.Join(top, "docs")
	if err := os.Mkdir(docs, 0o755); err != nil {
		t.Fatal(err)
	}

	if out, code := run(t, docs, "init", "--name", "Nested"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	// GitHub only runs workflows from the top of the repository.
	workflow, err := os.ReadFile(filepath.Join(top, ".github", "workflows", "archdoc-lint.yml"))
	if err != nil {
		t.Fatalf("the workflow is not at the git root: %v", err)
	}
	if !strings.Contains(string(workflow), "working-directory: docs") {
		t.Errorf("the workflow does not tell archdoc where to run:\n%s", workflow)
	}
	if _, err := os.Stat(filepath.Join(docs, ".github")); err == nil {
		t.Error("a .github directory was also created under the spec directory")
	}
}

func TestInitPinsTheWorkflowToAVersion(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "X"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	workflow, _ := os.ReadFile(filepath.Join(dir, ".github", "workflows", "archdoc-lint.yml"))
	// A dev build has no release to pin to, so it writes latest rather than a
	// tag that does not exist.
	if !strings.Contains(string(workflow), "ARCHDOC_VERSION: latest") {
		t.Errorf("a dev build should pin latest:\n%s", workflow)
	}
	for _, want := range []string{"archdoc lint", "archdoc index --check", "checksums.txt"} {
		if !strings.Contains(string(workflow), want) {
			t.Errorf("the workflow is missing %q:\n%s", want, workflow)
		}
	}
}

func TestInitRefusesAnExistingIndex(t *testing.T) {
	// INDEX.md is written after the collision loop, so it was the one
	// scaffolded file init would replace without asking.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, dir, "init", "--name", "X")
	if code != exitUsage || !strings.Contains(out, "already exists") {
		t.Errorf("exited %d with %q, want a refusal", code, out)
	}
	if kept, _ := os.ReadFile(filepath.Join(dir, "INDEX.md")); string(kept) != "mine\n" {
		t.Errorf("the existing index was overwritten: %q", kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "archdoc.json")); err == nil {
		t.Error("archdoc.json was written despite the refusal")
	}
}

func TestInitOmitsTheWorkingDirectoryAtTheRoot(t *testing.T) {
	// docs/ARCHDOC.md: working-directory is "omitted when they are the same".
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "X"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	workflow, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "archdoc-lint.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(workflow), "working-directory: .") {
		t.Errorf("the key should be omitted when archdoc.json is at the root:\n%s", workflow)
	}
	if !strings.Contains(string(workflow), "run: archdoc lint") {
		t.Errorf("the workflow lost its steps:\n%s", workflow)
	}
}

func TestInitCanBeRetriedAfterAFailedWrite(t *testing.T) {
	// A write that fails part-way leaves what came before it. If archdoc.json is
	// among those, every later attempt refuses and the user has to clean up by
	// hand, which is the end state the plan-then-write design exists to avoid.
	dir := t.TempDir()
	blocked := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o755) })

	if _, code := run(t, dir, "init", "--name", "X"); code == exitOK {
		t.Skip("the write was not blocked on this filesystem")
	}
	if _, err := os.Stat(filepath.Join(dir, "archdoc.json")); err == nil {
		t.Error("archdoc.json survives a failed init, so it can never be retried")
	}
}

func TestInitAcceptsNoStrict(t *testing.T) {
	// docs/ARCHDOC.md documents "--strict / --no-strict". pflag does not synthesise
	// the negative spelling, so it has to be registered.
	dir := t.TempDir()
	out, code := run(t, dir, "init", "--name", "Negative", "--no-strict")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d:\n%s", code, exitOK, out)
	}

	settings, err := os.ReadFile(filepath.Join(dir, "archdoc.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), `"strict": false`) {
		t.Errorf("--no-strict did not turn strict off:\n%s", settings)
	}
}

func TestInitRefusesBothSpellingsAtOnce(t *testing.T) {
	// Whichever won would depend on the order they are read in, which is
	// invisible from the command line.
	dir := t.TempDir()
	out, code := run(t, dir, "init", "--name", "Both", "--strict", "--no-strict")
	if code == exitOK {
		t.Fatalf("exit = %d, want a failure:\n%s", code, out)
	}
	if !strings.Contains(out, "--strict") || !strings.Contains(out, "--no-strict") {
		t.Errorf("the message does not name both flags:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "archdoc.json")); err == nil {
		t.Error("a contradictory invocation still scaffolded the repository")
	}
}

func TestInitSaysWhenItCannotPinTheWorkflow(t *testing.T) {
	// The generated file carries "Pinned to the version that scaffolded this
	// repository... Raise it deliberately" directly above ARCHDOC_VERSION, so
	// when a dev build writes "latest" the artefact asserts something untrue and
	// nothing said so. Both docs/ARCHDOC.md and docs/DECISIONS.md claimed a warning that
	// did not exist.
	dir := t.TempDir()

	out, code := run(t, dir, "init", "--name", "Unpinned")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	workflow, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(workflowPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "ARCHDOC_VERSION: latest") {
		t.Skip("this build reports a real version, so there is nothing to warn about")
	}
	if !strings.Contains(out, "latest") {
		t.Errorf("init wrote an unpinned workflow and said nothing:\n%s", out)
	}
}

func TestOnlyAPublishedReleaseTagCountsAsPinnable(t *testing.T) {
	// The test was "is the version the string dev", so every other value was
	// taken for a release tag. go install from a branch or a commit reports a
	// pseudo-version, and pinning to one wrote a workflow whose first CI run
	// could only fail, silently.
	for _, tc := range []struct {
		version  string
		pinnable bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true},
		{"v1.2.3-rc.1", true},
		{"dev", false},
		{"v0.0.0-20260912120000-aaaaaaaaaaaa", false},
		{"", false},
		{"main", false},
	} {
		if got := pinnable(tc.version); got != tc.pinnable {
			t.Errorf("pinnable(%q) = %v, want %v", tc.version, got, tc.pinnable)
		}
	}
}

func TestInitDoesNotRecordHEADAsABranch(t *testing.T) {
	// A detached head is what actions/checkout produces for a pull request, and
	// git reports the literal string "HEAD" for it. Recording that is worse than
	// useless: HEAD always resolves, so L11 would compare every frozen document
	// against the current commit rather than against the base branch, and never
	// report anything.
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
	if err := os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "seed"}, {"checkout", "--detach", "HEAD"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if out, code := run(t, dir, "init", "--name", "Detached"); code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	settings, err := os.ReadFile(filepath.Join(dir, config.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(settings), `"branch": "HEAD"`) {
		t.Errorf("init recorded HEAD as the branch:\n%s", settings)
	}
}

// TestChdirRunsInAnotherDirectory pins the flag that makes the two-repository
// workflow usable: a spec repository is commonly a sibling of the code it
// documents, so config.Find walking up from the working directory never reaches
// it, and an agent working in the code has no way to run archdoc without it.
func TestChdirRunsInAnotherDirectory(t *testing.T) {
	parent := t.TempDir()
	spec := filepath.Join(parent, "spec")
	code := filepath.Join(parent, "code")
	for _, d := range []string{spec, code} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, code := run(t, spec, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	// From the sibling, with no archdoc.json above it, lint is only reachable
	// through the flag.
	out, exit := run(t, code, "lint")
	if exit == exitOK {
		t.Fatalf("lint succeeded from a directory with no repository above it:\n%s", out)
	}
	out, exit = run(t, code, "--chdir", spec, "lint")
	if exit != exitOK {
		t.Errorf("lint with --chdir exited %d:\n%s", exit, out)
	}

	out, exit = run(t, code, "-C", "/does/not/exist", "lint")
	if exit == exitOK {
		t.Error("--chdir accepted a directory that does not exist")
	}
	if !strings.Contains(out, "chdir") {
		t.Errorf("the error does not name the flag:\n%s", out)
	}
}

// TestAgentsIsOptAndWritesTheGuides pins both halves of the flag: absent by
// default, so a repository whose owner does not use agents carries nothing it
// ignores, and complete when asked, with AGENTS.md at the top of the tree where
// the convention puts it rather than beside the guides it indexes.
func TestAgentsIsOptAndWritesTheGuides(t *testing.T) {
	t.Run("absent by default", func(t *testing.T) {
		dir := t.TempDir()
		out, code := run(t, dir, "init", "--name", "T")
		if code != exitOK {
			t.Fatalf("init exited %d: %s", code, out)
		}
		for _, unwanted := range []string{"AGENTS.md", "agents"} {
			if _, err := os.Stat(filepath.Join(dir, unwanted)); err == nil {
				t.Errorf("%s was written without --agents", unwanted)
			}
		}
		if strings.Contains(out, "AGENTS.md") {
			t.Errorf("init reported AGENTS.md without --agents:\n%s", out)
		}
	})

	t.Run("written when asked", func(t *testing.T) {
		dir := t.TempDir()
		out, code := run(t, dir, "init", "--name", "T", "--agents")
		if code != exitOK {
			t.Fatalf("init exited %d: %s", code, out)
		}
		// Every embedded guide reaches the repository, so adding one to the
		// templates cannot silently fail to ship.
		entries, err := template.Files.ReadDir("agents")
		if err != nil {
			t.Fatalf("ReadDir(agents): %v", err)
		}
		if len(entries) < 2 {
			t.Fatalf("expected several guides, found %d", len(entries))
		}
		for _, e := range entries {
			want := "agents/" + e.Name()
			if e.Name() == "AGENTS.md" {
				want = "AGENTS.md"
			}
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(want))); err != nil {
				t.Errorf("%s was not written: %v", want, err)
			}
			if !strings.Contains(out, want) {
				t.Errorf("%s was written but not reported:\n%s", want, out)
			}
		}
		// The index is what an agent reads first, so it has to be at the top.
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
			t.Errorf("AGENTS.md is not at the root of the repository: %v", err)
		}
	})
}

// TestAgentsRefreshesGuidesButNeverTheIndex pins the ownership split. The
// guides ship with the binary and change as ArchDoc does, so re-running must
// replace them; AGENTS.md is where a project writes its own instructions, so
// re-running must not touch it.
func TestAgentsRefreshesGuidesButNeverTheIndex(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T", "--agents"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}

	guide := filepath.Join(dir, "agents", "triage.md")
	index := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(guide, []byte("stale, from an older archdoc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const mine = "\nProject rule: every RFC links its discussion thread.\n"
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, append(before, mine...), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, dir, "agents")
	if code != exitOK {
		t.Fatalf("agents exited %d: %s", code, out)
	}

	got, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "stale, from an older archdoc") {
		t.Error("agents left a stale guide in place")
	}
	if !strings.Contains(string(got), "archdoc-triage") {
		t.Errorf("the refreshed guide is not the shipped one:\n%s", got)
	}

	kept, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kept), mine) {
		t.Error("agents overwrote AGENTS.md, discarding the project's own instructions")
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("agents did not report keeping AGENTS.md:\n%s", out)
	}
}

// TestAgentsCreatesTheIndexWhenItIsMissing covers the other half: a repository
// scaffolded before --agents existed, or one whose AGENTS.md was deleted, gets
// one rather than being left with guides nothing points at.
func TestAgentsCreatesTheIndexWhenItIsMissing(t *testing.T) {
	dir := t.TempDir()
	if out, code := run(t, dir, "init", "--name", "T"); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	out, code := run(t, dir, "agents")
	if code != exitOK {
		t.Fatalf("agents exited %d: %s", code, out)
	}
	for _, want := range []string{"AGENTS.md", filepath.Join("agents", "working.md")} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s was not created: %v", want, err)
		}
	}
	if !strings.Contains(out, "written") {
		t.Errorf("agents did not report writing AGENTS.md:\n%s", out)
	}
}
