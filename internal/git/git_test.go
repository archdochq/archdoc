package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"archdoc.dev/internal/git"
)

// newRepo makes a real repository in a temporary directory. Identity is set
// locally so the tests do not depend on the machine's git configuration.
func newRepo(t *testing.T) string {
	t.Helper()
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
	return dir
}

func commit(t *testing.T, dir, path, contents string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", "commit " + path}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestOpenRejectsADirectoryOutsideAnyRepository(t *testing.T) {
	if _, err := git.Open(t.TempDir()); !errors.Is(err, git.ErrNotARepository) {
		t.Errorf("error = %v, want it to match git.ErrNotARepository", err)
	}
}

func TestBranchExistsDistinguishesAMissingBranch(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.md", "a\n")

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if exists, err := g.BranchExists("main"); err != nil || !exists {
		t.Errorf("BranchExists(main) = %v, %v; want true, nil", exists, err)
	}
	if exists, err := g.BranchExists("trunk"); err != nil || exists {
		t.Errorf("BranchExists(trunk) = %v, %v; want false, nil", exists, err)
	}
}

func TestCurrentBranchNamesTheCheckedOutBranch(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.md", "a\n")

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	branch, err := g.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "main")
	}
}

func TestRepoRootIsTheToplevelEvenFromASubdirectory(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "docs/spec/x.md", "x\n")

	g, err := git.Open(filepath.Join(dir, "docs", "spec"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	root, err := g.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	// git resolves symlinks; the temporary directory on macOS is one.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != want {
		t.Errorf("RepoRoot = %q, want %q", root, want)
	}
}

func TestListFilesNamesEveryPathOnTheBranch(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "rfc/0001-x.md", "x\n")
	commit(t, dir, "spec/http/y.md", "y\n")
	// A working tree addition is not on the branch and must not be listed.
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.md"), []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	files, err := g.ListFiles("main")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	want := []string{"rfc/0001-x.md", "spec/http/y.md"}
	if !slices.Equal(files, want) {
		t.Errorf("ListFiles = %q, want %q", files, want)
	}
}

func TestOpenDistinguishesAMissingGitFromAMissingRepository(t *testing.T) {
	// With no PATH the git binary cannot be found at all. Reporting that as
	// "not a repository" would let lint skip every frozen-document check and
	// still exit 0.
	t.Setenv("PATH", "")

	_, err := git.Open(t.TempDir())
	if err == nil {
		t.Fatal("Open succeeded with no git binary available")
	}
	if errors.Is(err, git.ErrNotARepository) {
		t.Errorf("error = %v, want one that does not claim the directory is not a repository", err)
	}
	if !strings.Contains(err.Error(), "install git") {
		t.Errorf("error = %v, want it to say git must be installed", err)
	}
}

func TestBranchResolvesThroughTheRemoteTrackingRef(t *testing.T) {
	// A CI checkout usually has the comparison branch only as origin/<branch>,
	// and rev-parse does not resolve a bare name to a remote-tracking ref. The
	// frozen-document rule is worth nothing if it cannot find the branch.
	origin := newRepo(t)
	commit(t, origin, "rfc/0001-x.md", "committed\n")

	clone := t.TempDir()
	cmd := exec.Command("git", "clone", "--no-checkout", origin, clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	// Check out a detached head, as actions/checkout does for a pull request,
	// leaving no local main.
	for _, args := range [][]string{{"checkout", "--detach", "origin/main"}, {"branch", "-D", "main"}} {
		c := exec.Command("git", args...)
		c.Dir = clone
		c.CombinedOutput() // a missing local main is fine
	}

	g, err := git.Open(clone)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if exists, err := g.BranchExists("main"); err != nil || !exists {
		t.Errorf("BranchExists(main) = %v, %v; want true: it exists as origin/main", exists, err)
	}
	blobs, err := g.FilesAt("main", []string{"rfc/0001-x.md"})
	if err != nil {
		t.Fatalf("FilesAt: %v", err)
	}
	if string(blobs["rfc/0001-x.md"]) != "committed\n" {
		t.Errorf("FilesAt = %q, want the contents reached through origin/main", blobs["rfc/0001-x.md"])
	}
	if files, err := g.ListFiles("main"); err != nil || len(files) == 0 {
		t.Errorf("ListFiles = %v, %v; want the committed paths", files, err)
	}
}

func TestFilesAtReadsManyBlobsInOneOperation(t *testing.T) {
	dir := newRepo(t)
	// A path with a space, because the batch protocol echoes the request only
	// for a missing object and responses are otherwise paired by position.
	// Content with an embedded newline and a trailing one, because sizes are
	// what separate the entries.
	commit(t, dir, "rfc/0001-x.md", "first\ncontents\n")
	commit(t, dir, "rfc/0002 spaced.md", "second contents, no trailing newline")
	commit(t, dir, "rfc/0003-empty.md", "")
	// The working tree diverging must not change what is read. L11 compares the
	// branch against the tree, so a read that picked up the file on disk would
	// be comparing the tree against itself and could never report a change.
	if err := os.WriteFile(filepath.Join(dir, "rfc", "0001-x.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := g.FilesAt("main", []string{
		"rfc/0001-x.md", "rfc/0002 spaced.md", "rfc/0003-empty.md", "rfc/0004-absent.md",
	})
	if err != nil {
		t.Fatalf("FilesAt: %v", err)
	}

	want := map[string]string{
		"rfc/0001-x.md":      "first\ncontents\n",
		"rfc/0002 spaced.md": "second contents, no trailing newline",
		"rfc/0003-empty.md":  "",
	}
	if len(got) != len(want) {
		t.Fatalf("read %d files, want %d: %v", len(got), len(want), got)
	}
	for path, contents := range want {
		if string(got[path]) != contents {
			t.Errorf("%s = %q, want %q", path, got[path], contents)
		}
	}
	if _, present := got["rfc/0004-absent.md"]; present {
		t.Error("a path absent from the branch came back with contents")
	}
}

func TestFilesAtReturnsNothingForNoPaths(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "rfc/0001-x.md", "x\n")

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := g.FilesAt("main", nil)
	if err != nil || len(got) != 0 {
		t.Errorf("FilesAt(nil) = %v, %v, want empty and no error", got, err)
	}
}

func TestFilesAtSurvivesAPathContainingANewline(t *testing.T) {
	// The batch protocol pairs responses to requests by position. With requests
	// separated by newlines, a path containing one became two requests and two
	// responses, so every later path was paired with the previous path's
	// content: L11 then reported a frozen document as changed when it was not,
	// and missed one that was. -z separates with NUL, as ListFiles already does.
	dir := newRepo(t)
	commit(t, dir, "adr/0000-a\nb.md", "newline path\n")
	commit(t, dir, "adr/0001-frozen.md", "frozen contents\n")

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := g.FilesAt("main", []string{"adr/0000-a\nb.md", "adr/0001-frozen.md"})
	if err != nil {
		t.Fatalf("FilesAt: %v", err)
	}

	for path, want := range map[string]string{
		"adr/0000-a\nb.md":   "newline path\n",
		"adr/0001-frozen.md": "frozen contents\n",
	} {
		if string(got[path]) != want {
			t.Errorf("%q = %q, want %q", path, got[path], want)
		}
	}
}

func TestCurrentBranchAnswersBeforeTheFirstCommit(t *testing.T) {
	// rev-parse --abbrev-ref HEAD cannot resolve an unborn HEAD, so it returned
	// the literal "HEAD" and init fell back to writing branch: main into the
	// configuration of a repository whose branch was something else. The first
	// commit then made L11 fail with "branch main does not exist" on a
	// repository the user had done nothing wrong in. symbolic-ref answers
	// without needing a commit.
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "trunk")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	branch, err := g.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "trunk" {
		t.Errorf("CurrentBranch = %q, want %q", branch, "trunk")
	}
}

func TestCurrentBranchStillReportsADetachedHead(t *testing.T) {
	// A detached head has no branch, and init must not record one.
	dir := newRepo(t)
	commit(t, dir, "rfc/0001-x.md", "x\n")
	for _, args := range [][]string{{"checkout", "--detach", "HEAD"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if branch, err := g.CurrentBranch(); err == nil && branch != "HEAD" {
		t.Errorf("CurrentBranch = %q on a detached head, want %q or an error", branch, "HEAD")
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

func TestOpenRecognisesARepositoryOutsideAnyLocale(t *testing.T) {
	// Open matches git's message, which git translates. This passes trivially
	// where git ships no translations, as Apple's does.
	t.Setenv("LANGUAGE", "de")
	t.Setenv("LC_ALL", "C.UTF-8")
	if _, err := git.Open(t.TempDir()); !errors.Is(err, git.ErrNotARepository) {
		t.Errorf("error = %v, want it to match git.ErrNotARepository", err)
	}
}

func TestOpenReportsAnyOtherGitFailureAsAnError(t *testing.T) {
	// git exits 128 both for a directory outside any repository and for one it
	// refuses to read. Mapping every 128 to ErrNotARepository skipped L11 at
	// exit 0 in a container whose checkout belonged to another user.
	for name, env := range map[string][2]string{
		"dubious ownership": {"GIT_TEST_ASSUME_DIFFERENT_OWNER", "1"},
		"broken config":     {"GIT_CONFIG_GLOBAL", ""},
	} {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t)
			commit(t, dir, "a.md", "a\n")
			value := env[1]
			if value == "" {
				value = filepath.Join(t.TempDir(), "gitconfig")
				if err := os.WriteFile(value, []byte("[core\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv(env[0], value)

			_, err := git.Open(dir)
			if err == nil || errors.Is(err, git.ErrNotARepository) {
				t.Errorf("error = %v, want an error that is not git.ErrNotARepository", err)
			}
		})
	}
}

func TestOpenFindsTheRepositoryDespiteInheritedLocation(t *testing.T) {
	// git exports GIT_DIR without GIT_WORK_TREE to hooks and to rebase -x in a
	// linked worktree, and then takes the working directory as the top level.
	// Each variable here points git at another repository or another top level.
	for name, value := range map[string]func(dir, other string) string{
		"GIT_DIR": func(dir, _ string) string {
			wt := filepath.Join(filepath.Dir(dir), "wt")
			gitIn(t, dir, "worktree", "add", "-q", "-b", "side", wt)
			return filepath.Join(dir, ".git", "worktrees", "wt")
		},
		"GIT_WORK_TREE":  func(_, other string) string { return other },
		"GIT_COMMON_DIR": func(_, other string) string { return filepath.Join(other, ".git") },
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "repo")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			gitIn(t, dir, "init", "-q", "-b", "main")
			gitIn(t, dir, "config", "user.email", "test@example.invalid")
			gitIn(t, dir, "config", "user.name", "Fixture")
			commit(t, dir, "docs/a.md", "a\n")
			other := newRepo(t)
			commit(t, other, "elsewhere.md", "b\n")
			t.Setenv(name, value(dir, other))

			g, err := git.Open(filepath.Join(dir, "docs"))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if prefix, err := g.Prefix(); err != nil || prefix != "docs" {
				t.Errorf("Prefix() = %q, %v; want docs", prefix, err)
			}
			if files, err := g.ListFiles("main"); err != nil || !slices.Equal(files, []string{"docs/a.md"}) {
				t.Errorf("ListFiles(main) = %q, %v; want docs/a.md", files, err)
			}
		})
	}
}

func TestChangedIgnoresAnInheritedIndex(t *testing.T) {
	// A pre-commit hook inherits GIT_INDEX_FILE, which may be relative to the
	// top level; read from elsewhere it names no index, and every tracked file
	// then read as changed.
	dir := newRepo(t)
	commit(t, dir, "docs/adr/0001-a.md", "a\n")
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "none"))

	g, err := git.Open(filepath.Join(dir, "docs"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got, err := g.Changed("main", []string{"docs/adr/0001-a.md"}); err != nil || len(got) != 0 {
		t.Errorf("Changed = %v, %v; want nothing changed", got, err)
	}
}

func TestChangedIgnoresInheritedPathspecSettings(t *testing.T) {
	for _, variable := range []string{"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS"} {
		t.Run(variable, func(t *testing.T) {
			dir := newRepo(t)
			commit(t, dir, "docs/adr/0001-a.md", "a\n")
			if err := os.WriteFile(filepath.Join(dir, "docs", "adr", "0001-a.md"), []byte("edited\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv(variable, "1")

			g, err := git.Open(filepath.Join(dir, "docs"))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			got, err := g.Changed("main", []string{"docs/adr/0001-a.md"})
			if err != nil || !got["docs/adr/0001-a.md"] {
				t.Errorf("Changed = %v, %v; want docs/adr/0001-a.md changed", got, err)
			}
		})
	}
}

func TestFilesAtSurvivesAMissingPathContainingANewline(t *testing.T) {
	// A missing request is answered "<request> missing", echoing the path with
	// its newline, while the request itself was NUL-terminated.
	dir := newRepo(t)
	commit(t, dir, "adr/0001-frozen.md", "frozen contents\n")

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := g.FilesAt("main", []string{"my\ndir/archdoc.json", "adr/0001-frozen.md"})
	if err != nil {
		t.Fatalf("FilesAt: %v", err)
	}
	if _, ok := got["my\ndir/archdoc.json"]; ok || string(got["adr/0001-frozen.md"]) != "frozen contents\n" {
		t.Errorf("FilesAt = %q, want only adr/0001-frozen.md", got)
	}
}

func TestMergeBaseIsEmptyOnAnUnbornBranch(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.md", "a\n")
	gitIn(t, dir, "checkout", "-q", "--orphan", "fresh")

	g, err := git.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if base, err := g.MergeBase("main"); err != nil || base != "" {
		t.Errorf("MergeBase(main) = %q, %v; want empty, nil", base, err)
	}
}
