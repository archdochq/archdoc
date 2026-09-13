package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTheDocumentedWorkflowWorksEndToEnd walks the whole journey docs/ARCHDOC.md
// describes, in one real git repository, and fails if any step stops working.
//
// Every other test in this module pins one rule, one function or one defect.
// This one pins the only thing a user actually cares about: that the sequence
// of commands the documentation tells them to run does what it says. Four
// rounds of review repaired a great many edge cases, and the thing none of them
// measured was whether the ordinary path still ran.
func TestTheDocumentedWorkflowWorksEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	dir := filepath.Join(t.TempDir(), "myproject-spec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.invalid")
	runGit(t, dir, "config", "user.name", "Fixture")

	step := func(name string, want int, args ...string) string {
		t.Helper()
		out, code := run(t, dir, args...)
		if code != want {
			t.Fatalf("%s: exit = %d, want %d\n%s", name, code, want, out)
		}
		return out
	}

	step("init", exitOK, "init", "--name", "MyProject", "--license", "mit")
	// Straight after init, before any commit: this is the first thing anyone
	// does and it must not fail.
	step("lint a fresh scaffold", exitOK, "lint")
	step("index --check on a fresh scaffold", exitOK, "index", "--check")

	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "scaffold")
	step("lint a committed scaffold", exitOK, "lint")

	// A decision, written and frozen.
	step("new adr", exitOK, "new", "adr", "Storage engine")
	fill(t, filepath.Join(dir, "adr", "0001-storage-engine.md"))
	step("propose", exitOK, "propose", "ADR-0001")
	step("accept", exitOK, "accept", "ADR-0001")

	// A glossary, through all three writers.
	step("term add", exitOK, "term", "add", "Wings", "The node daemon.")
	step("term rename", exitOK, "term", "rename", "Wings", "Daemon")
	step("term remove", exitOK, "term", "remove", "Daemon")
	step("term add again", exitOK, "term", "add", "Wings", "The node daemon.")

	// A design that links to the decision and to the glossary.
	step("new rfc", exitOK, "new", "rfc", "Job scheduling")
	rfc := filepath.Join(dir, "rfc", "0001-job-scheduling.md")
	fill(t, rfc)
	appendTo(t, rfc, "\nIt depends on [[ADR-0001]] and uses [[Wings]].\n")
	step("link", exitOK, "link")
	if body := readFile(t, rfc); strings.Contains(body, "[[") {
		t.Errorf("link left a wiki link unresolved:\n%s", body)
	}

	step("index", exitOK, "index")
	step("index --check after index", exitOK, "index", "--check")
	step("lint after all of it", exitOK, "lint")

	// Everything committed, so the frozen ADR is genuinely frozen.
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "work")
	step("lint with a frozen document on the branch", exitOK, "lint")

	// And the freeze actually holds.
	appendTo(t, filepath.Join(dir, "adr", "0001-storage-engine.md"), "\nAn edit after freezing.\n")
	if out := step("lint after editing a frozen document", exitFindings, "lint"); !strings.Contains(out, "frozen") {
		t.Errorf("lint did not report the edit to a frozen document:\n%s", out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// fill replaces the template's prompts with content, as an author would.
func fill(t *testing.T, path string) {
	t.Helper()
	body := regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(readFile(t, path), "Recorded content.")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(readFile(t, path)+text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
