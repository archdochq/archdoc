package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scaffolded builds a repository the way a user would have, in a real git
// repository so the workflow lands where it belongs.
func scaffolded(t *testing.T, args ...string) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if out, code := run(t, dir, append([]string{"init", "--name", "T"}, args...)...); code != exitOK {
		t.Fatalf("init exited %d: %s", code, out)
	}
	return dir
}

// TestUpdateRefreshesWhatArchdocGenerates covers the reason the command exists:
// a defect in archdoc puts a wrong file into every repository it scaffolds, and
// without this the only remedy is a hand edit in each one.
func TestUpdateRefreshesWhatArchdocGenerates(t *testing.T) {
	dir := scaffolded(t)
	process := filepath.Join(dir, "PROCESS.md")
	workflow := filepath.Join(dir, ".github", "workflows", "archdoc-lint.yml")

	if err := os.WriteFile(process, []byte("stale, from an older archdoc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The shape the case bug produced: a path no checkout could contain.
	broken, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflow, []byte(strings.Replace(string(broken),
		"run: archdoc lint", "working-directory: ../../../somewhere/local\n        run: archdoc lint", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, dir, "update", "--yes")
	if code != exitOK {
		t.Fatalf("update exited %d: %s", code, out)
	}

	got, err := os.ReadFile(process)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "stale, from an older archdoc") {
		t.Error("update left a stale PROCESS.md in place")
	}
	if !strings.Contains(string(got), "## Lifecycle") {
		t.Errorf("PROCESS.md was not replaced with the shipped one:\n%s", got)
	}
	after, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "somewhere/local") {
		t.Error("update left the broken working-directory in the workflow")
	}
}

// TestUpdateNeverTouchesTheFilesTheRepositoryOwns is the guard against the
// command being destructive. Everything here is written once by init and then
// belongs to the project.
func TestUpdateNeverTouchesTheFilesTheRepositoryOwns(t *testing.T) {
	dir := scaffolded(t, "--agents")

	owned := map[string]string{
		"AGENTS.md":    "\nProject rule: every RFC links its discussion thread.\n",
		"README.md":    "\nOur own words.\n",
		"archdoc.json": "",
	}
	before := map[string][]byte{}
	for name, addition := range owned {
		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if addition != "" {
			body = append(body, addition...)
			if err := os.WriteFile(path, body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		before[name] = body
	}

	if out, code := run(t, dir, "update", "--yes"); code != exitOK {
		t.Fatalf("update exited %d: %s", code, out)
	}
	for name, want := range before {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("update modified %s, which the repository owns", name)
		}
	}
}

// TestUpdateRefreshesAgentsOnlyWhenPresent keeps update and agents distinct:
// agents installs, update refreshes what is already there.
func TestUpdateRefreshesAgentsOnlyWhenPresent(t *testing.T) {
	t.Run("absent, and said so", func(t *testing.T) {
		dir := scaffolded(t)
		out, code := run(t, dir, "update", "--yes")
		if code != exitOK {
			t.Fatalf("update exited %d: %s", code, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "agents")); err == nil {
			t.Error("update created agents/, which is the agents command's job")
		}
		if !strings.Contains(out, "archdoc agents") {
			t.Errorf("update did not point at the command that installs them:\n%s", out)
		}
	})

	t.Run("present, and refreshed", func(t *testing.T) {
		dir := scaffolded(t, "--agents")
		guide := filepath.Join(dir, "agents", "triage.md")
		// A marker that cannot occur in the real guide. "stale" does: the guide
		// discusses ref_stale_days and spec page staleness, so asserting on it
		// passes against correct content.
		const marker = "WRITTEN-BY-AN-OLDER-ARCHDOC"
		if err := os.WriteFile(guide, []byte(marker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, code := run(t, dir, "update", "--yes"); code != exitOK {
			t.Fatalf("update exited %d: %s", code, out)
		}
		got, err := os.ReadFile(guide)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), marker) {
			t.Error("update left the old guide in place")
		}
		if !strings.Contains(string(got), "archdoc-triage") {
			t.Errorf("the guide was not replaced with the shipped one:\n%s", got)
		}
	})
}

// TestUpdateCheckShowsWithoutWriting gives a user a way to see what would
// change before agreeing to it, which is the question "are you sure" cannot
// answer on its own.
func TestUpdateCheckShowsWithoutWriting(t *testing.T) {
	dir := scaffolded(t)
	process := filepath.Join(dir, "PROCESS.md")
	if err := os.WriteFile(process, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := run(t, dir, "update", "--check")
	// exitFindings specifically, not merely non-zero: the unattended refusal
	// also exits non-zero and prints no diff, so "not 0" cannot tell them apart.
	if code != exitFindings {
		t.Errorf("update --check exited %d, want %d:\n%s", code, exitFindings, out)
	}
	if strings.Contains(out, "--yes") {
		t.Errorf("--check took the unattended refusal path instead of reporting:\n%s", out)
	}
	if !strings.Contains(out, "+++") {
		t.Errorf("--check printed no diff:\n%s", out)
	}
	if !strings.Contains(out, "PROCESS.md") {
		t.Errorf("--check did not name the file that would change:\n%s", out)
	}
	got, err := os.ReadFile(process)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "stale\n" {
		t.Error("--check wrote to the file")
	}
}

// TestUpdateRefusesToRunUnattendedWithoutConsent stops a script or CI job from
// silently overwriting a customised workflow, which is the realistic way this
// command loses someone's work.
func TestUpdateRefusesToRunUnattendedWithoutConsent(t *testing.T) {
	dir := scaffolded(t)
	if err := os.WriteFile(filepath.Join(dir, "PROCESS.md"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// run() always passes --no-interaction, so this is the unattended case.
	out, code := run(t, dir, "update")
	if code == exitOK {
		t.Errorf("update applied changes unattended without --yes:\n%s", out)
	}
	// The refusal has to be deliberate. Without this, deleting the guard still
	// fails the command, because the prompt errors with no terminal attached,
	// and the test cannot tell a considered refusal from a broken prompt.
	if !strings.Contains(out, "--yes") {
		t.Errorf("update did not refuse deliberately, naming the flag that consents:\n%s", out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "PROCESS.md"))
	if string(got) != "stale\n" {
		t.Error("update wrote despite refusing")
	}
}

// TestUpdateKeepsTheExistingVersionPin stops the command from changing which
// ArchDoc a repository's CI runs as a side effect of fixing something else.
//
// The template's own comment says to raise the pin deliberately. Regenerating
// the workflow is about the generated content; moving the pin is a separate
// decision. It matters most when the binary doing the update is not itself a
// published release, because then the value it would write is "latest", and a
// repository pinned to a real version would be silently unpinned.
func TestUpdateKeepsTheExistingVersionPin(t *testing.T) {
	dir := scaffolded(t)
	workflow := filepath.Join(dir, ".github", "workflows", "archdoc-lint.yml")
	body, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatal(err)
	}
	// A repository pinned to a real release, with something else about the
	// workflow out of date so there is a reason to regenerate it.
	pinned := strings.ReplaceAll(string(body), "ARCHDOC_VERSION: latest", "ARCHDOC_VERSION: v9.9.9")
	pinned = strings.Replace(pinned, "run: archdoc lint", "working-directory: ../../../local/path\n        run: archdoc lint", 1)
	if err := os.WriteFile(workflow, []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, code := run(t, dir, "update", "--yes"); code != exitOK {
		t.Fatalf("update exited %d: %s", code, out)
	}

	got, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "ARCHDOC_VERSION: v9.9.9") {
		t.Errorf("update changed the version pin:\n%s", got)
	}
	if strings.Contains(string(got), "local/path") {
		t.Error("update did not regenerate the workflow, so the pin was preserved by doing nothing")
	}
}
