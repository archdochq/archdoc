package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// onMain commits the repository to main, so that what follows is judged as a
// pull request against it.
func onMain(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.invalid")
	runGit(t, dir, "config", "user.name", "Fixture")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
}

func TestLintMergeReportsADraftArriving(t *testing.T) {
	dir := repository(t)
	onMain(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "rfc/0003-draft.md"), []byte(rfc("RFC-0003", "Draft", "draft", "")), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, code := run(t, dir, "lint"); code != exitOK {
		t.Fatalf("lint without --merge: exit %d: %s", code, out)
	}
	out, code := run(t, dir, "lint", "--merge")
	if code != exitFindings || !hasLine(out, "rfc/0003-draft.md", "arrives on main as a draft") {
		t.Errorf("lint --merge: exit %d, output %q; want the draft reported", code, out)
	}
}

func TestLintMergeAndChangesNeedTheBranch(t *testing.T) {
	dir := repository(t)
	for _, args := range [][]string{{"lint", "--merge"}, {"changes"}} {
		out, code := run(t, dir, args...)
		if code != exitUsage || !strings.Contains(out, "no branch to compare against") || !strings.Contains(out, "main") {
			t.Errorf("%v outside git: exit %d, output %q; want a usage error naming main", args, code, out)
		}
	}
}

func TestChangesSummarisesThePullRequest(t *testing.T) {
	dir := repository(t)
	onMain(t, dir)
	editDocument(t, dir, "rfc/0002-cache.md", "status: proposed", "status: accepted")
	editDocument(t, dir, "rfc/0002-cache.md", "title: Caching", "title: Caching | the edge")
	editDocument(t, dir, "rfc/0001-queues.md", "The design.", "An edit after freezing.")
	if err := os.WriteFile(filepath.Join(dir, "rfc/0003-new.md"), []byte(rfc("RFC-0003", "New", "proposed", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "term/wings.md")); err != nil {
		t.Fatal(err)
	}

	out, code := run(t, dir, "changes")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, row := range []string{
		"| RFC-0001: Job queues | Edited, terminal on `main` |",
		"| RFC-0002: Caching \\| the edge | proposed → accepted |",
		"| RFC-0003: New | Arrives, proposed |",
		"| `term/wings.md` | Removed |",
	} {
		if !strings.Contains(out, row+"\n") {
			t.Errorf("Markdown lacks the row %q:\n%s", row, out)
		}
	}

	out, _ = run(t, dir, "changes", "--json")
	var got struct {
		Base    string           `json:"base"`
		Changes []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Base != "main" || len(got.Changes) != 4 {
		t.Fatalf("JSON = %+v", got)
	}
	if c := got.Changes[1]; c["change"] != "status" || c["from"] != "proposed" || c["to"] != "accepted" {
		t.Errorf("the change of status = %v", c)
	}
	if c := got.Changes[3]; c["change"] != "removed" || c["id"] != nil || c["from"] != nil || c["terminal"] != nil {
		t.Errorf("a removed term carries an identifier, a status or terminal: %v", c)
	}
}

func TestChangesSaysWhenTheRecordIsUnchanged(t *testing.T) {
	dir := repository(t)
	onMain(t, dir)
	out, _ := run(t, dir, "changes")
	if !strings.Contains(out, "The record is unchanged against `main`.") {
		t.Errorf("Markdown = %q", out)
	}
	out, _ = run(t, dir, "changes", "--json")
	if !strings.Contains(out, `"changes": []`) {
		t.Errorf("JSON = %q, want an empty array", out)
	}
}
