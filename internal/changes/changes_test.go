package changes_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"archdoc.dev/internal/changes"
	"archdoc.dev/internal/config"
	"archdoc.dev/internal/git"
	"archdoc.dev/internal/repo"
)

func document(id, title, status string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: " + status +
		"\ncreated: 2026-01-01\ndecided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# " + id + ": " + title + "\n"
}

func page(title, body string) string {
	return "---\ntitle: " + title + "\nincludes: []\n---\n\n# " + title + "\n\n" + body + "\n"
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

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// committed builds a git repository whose main holds files, with the spec
// repository under spec/ so that every path is translated through a prefix.
func committed(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "test@example.invalid")
	gitIn(t, dir, "config", "user.name", "Fixture")
	write(t, dir, "spec/archdoc.json", `{"name":"Changes","branch":"main"}`)
	for path, contents := range files {
		write(t, dir, filepath.Join("spec", path), contents)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func changesIn(t *testing.T, dir string) []changes.Change {
	t.Helper()
	c, err := config.Find(filepath.Join(dir, "spec"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := repo.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	g, err := git.Open(c.Dir())
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

func TestChangesListsEachChangeInOrder(t *testing.T) {
	dir := committed(t, map[string]string{
		"rfc/0001-moves.md":   document("RFC-0001", "Moves", "proposed"),
		"rfc/0002-edited.md":  document("RFC-0002", "Edited", "accepted"),
		"rfc/0003-same.md":    document("RFC-0003", "Same", "proposed"),
		"adr/0001-removed.md": document("ADR-0001", "Removed", "proposed"),
		"spec/page.md":        page("Page", "Before."),
		"term/gone.md":        "---\ntitle: gone\nformerly: []\nnamed_by: RFC-0001\n---\n\nA term.\n",
	})
	write(t, dir, "spec/rfc/0001-moves.md", document("RFC-0001", "Moves", "accepted"))
	write(t, dir, "spec/rfc/0002-edited.md", document("RFC-0002", "Edited", "accepted")+"\nAn edit.\n")
	write(t, dir, "spec/rfc/0010-arrives.md", document("RFC-0010", "Arrives", "proposed"))
	write(t, dir, "spec/spec/page.md", page("Page", "After."))
	os.Remove(filepath.Join(dir, "spec/adr/0001-removed.md"))
	os.Remove(filepath.Join(dir, "spec/term/gone.md"))

	got := changesIn(t, dir)
	var summary []string
	for _, c := range got {
		summary = append(summary, c.Path+" "+c.Change+" "+string(c.From)+">"+string(c.To))
	}
	want := []string{
		"rfc/0001-moves.md status proposed>accepted",
		"rfc/0002-edited.md edited >",
		"rfc/0010-arrives.md arrives >proposed",
		"adr/0001-removed.md removed proposed>",
		"spec/page.md edited >",
		"term/gone.md removed >",
	}
	if !slices.Equal(summary, want) {
		t.Fatalf("changes:\n got %q\nwant %q", summary, want)
	}
	if got[1].Terminal == nil || !*got[1].Terminal {
		t.Errorf("an edit to a document accepted on the base is not marked terminal: %+v", got[1])
	}
	if got[4].Terminal != nil {
		t.Errorf("an edit to a spec page, which has no status, carries terminal: %+v", got[4])
	}
	if got[4].ID != "" || got[4].Title != "Page" {
		t.Errorf("a spec page is not named by its path alone: %+v", got[4])
	}
	if got[0].Title != "Moves" || got[0].ID != "RFC-0001" {
		t.Errorf("a numbered document is not named by identifier and title: %+v", got[0])
	}
}

func TestAStatusMissingOnEitherSideReadsAsAnEdit(t *testing.T) {
	dir := committed(t, map[string]string{"rfc/0001-broken.md": "---\nid: RFC-0001\ntitle: Broken\n---\n\n# RFC-0001: Broken\n"})
	write(t, dir, "spec/rfc/0001-broken.md", document("RFC-0001", "Broken", "proposed"))
	got := changesIn(t, dir)
	if len(got) != 1 || got[0].Change != changes.Edited || got[0].From != "" {
		t.Errorf("repairing a missing status lists %+v, want one edit", got)
	}
}

func TestTheBaseStrictIsReadFromTheConfigurationBesideRoot(t *testing.T) {
	dir := committed(t, nil)
	write(t, dir, "spec/archdoc.json", `{"name":"Changes","branch":"main","strict":false}`)
	gitIn(t, dir, "commit", "-q", "-am", "relax")
	write(t, dir, "spec/archdoc.json", `{"name":"Changes","branch":"main"}`)

	c, err := config.Find(filepath.Join(dir, "spec"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := repo.Open(c)
	if err != nil {
		t.Fatal(err)
	}
	g, err := git.Open(c.Dir())
	if err != nil {
		t.Fatal(err)
	}
	base, err := changes.ReadBase(r, g)
	if err != nil {
		t.Fatal(err)
	}
	if base.Strict {
		t.Error("strict was read from the working tree, not from spec/archdoc.json on main")
	}
}

func TestADocumentOutsideRootIsNotOnTheBase(t *testing.T) {
	dir := committed(t, nil)
	write(t, dir, "rfc/0001-outside.md", document("RFC-0001", "Outside", "proposed"))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "outside root")
	if got := changesIn(t, dir); len(got) != 0 {
		t.Errorf("a document outside root lists %+v", got)
	}
}

func TestChangesIsEmptyForAnUnchangedRecord(t *testing.T) {
	dir := committed(t, map[string]string{"rfc/0001-same.md": document("RFC-0001", "Same", "proposed")})
	if got := changesIn(t, dir); len(got) != 0 {
		t.Errorf("an unchanged record lists %+v", got)
	}
}

func TestChangesIgnoresWhatTheBranchMergedSinceTheBranchWasCut(t *testing.T) {
	dir := committed(t, map[string]string{"rfc/0001-edited-later.md": document("RFC-0001", "Edited later", "proposed")})
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	write(t, dir, "spec/rfc/0003-arrives.md", document("RFC-0003", "Arrives", "proposed"))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "feature")
	gitIn(t, dir, "checkout", "-q", "main")
	write(t, dir, "spec/rfc/0001-edited-later.md", document("RFC-0001", "Edited later", "proposed")+"\nAn edit on main.\n")
	write(t, dir, "spec/rfc/0002-later.md", document("RFC-0002", "Later", "proposed"))
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "later")
	gitIn(t, dir, "checkout", "-q", "feature")

	got := changesIn(t, dir)
	if len(got) != 1 || got[0].Path != "rfc/0003-arrives.md" || got[0].Change != changes.Arrives {
		t.Errorf("an unrebased branch lists %+v, want only its own arrival", got)
	}
}
