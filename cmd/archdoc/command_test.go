package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"archdoc.dev/internal/config"
	"archdoc.dev/internal/link"
	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/repotest"
	"github.com/spf13/cobra"
)

// run executes archdoc inside dir and returns what it wrote and the exit code
// main would have used.
func run(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	t.Chdir(dir)

	// Both streams into one buffer: a test asserting on a notice cannot see it
	// otherwise, and sending the real stderr through made the suite's own
	// output noisy.
	var out bytes.Buffer
	root := newRoot(&out, &out)
	// Every test is non-interactive: prompting is the one thing this package
	// does that a test cannot drive.
	root.SetArgs(append([]string{"--no-interaction"}, args...))

	err := root.Execute()
	switch {
	case err == nil:
		return out.String(), exitOK
	default:
		var quiet silentExit
		if errors.As(err, &quiet) {
			return out.String(), int(quiet)
		}
		out.WriteString("archdoc: " + err.Error() + "\n")
		return out.String(), exitUsage
	}
}

// rfc renders a complete RFC so that only the field under test varies.
func rfc(id, title, status, decided string) string {
	return "---\nid: " + id + "\ntitle: " + title + "\nstatus: " + status +
		"\ncreated: 2026-01-01\ndecided:" + decided + "\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n" +
		"# " + id + ": " + title + "\n\n" +
		"## Abstract\n\nWhat this proposes.\n\n## Motivation\n\nWhy it matters.\n\n" +
		"## Proposal\n\nThe design.\n\n## Alternatives considered\n\nWhat lost.\n\n" +
		"## Backwards compatibility\n\nNothing breaks.\n\n## Open questions\n\n## Changelog\n\n- 2026-01-01: written.\n"
}

// repository is a small, valid spec repository on disk.
func repository(t *testing.T) string {
	t.Helper()
	r := repotest.New(t, map[string]string{
		"rfc/0001-queues.md": rfc("RFC-0001", "Job queues", "accepted", " 2026-02-01"),
		"rfc/0002-cache.md":  rfc("RFC-0002", "Caching", "proposed", ""),
		"spec/glossary.md":   "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Wings\n\nThe node daemon.\n",
	})
	return r.Config().Dir()
}

func TestVersionPrintsTheTagAlone(t *testing.T) {
	out, code := run(t, t.TempDir(), "--version")

	if code != exitOK {
		t.Errorf("exit = %d, want %d", code, exitOK)
	}
	if strings.TrimSpace(out) == "" || strings.Contains(out, " ") {
		t.Errorf("version output = %q, want the tag alone", out)
	}
}

func TestCommandsOutsideARepositorySaySo(t *testing.T) {
	out, code := run(t, t.TempDir(), "lint")

	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(out, "archdoc init") {
		t.Errorf("output = %q, want it to suggest archdoc init", out)
	}
}

func TestLintExitsCleanOnAValidRepository(t *testing.T) {
	out, code := run(t, repository(t), "lint")

	if strings.Contains(out, "error:") {
		t.Errorf("unexpected errors in:\n%s", out)
	}
	if code != exitOK {
		t.Errorf("exit = %d, want %d; output:\n%s", code, exitOK, out)
	}
}

func TestLintExitsTwoWhenItFindsErrors(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "rfc/0003-broken.md", "---\nid: RFC-9999\ntitle: Broken\nstatus: draft\ncreated: 2026-01-01\ndecided:\ndepends: []\nupdates: []\nobsoletes: []\n---\n\n# RFC-9999: Broken\n")

	out, code := run(t, dir, "lint")

	if code != exitFindings {
		t.Errorf("exit = %d, want %d; output:\n%s", code, exitFindings, out)
	}
	if !strings.Contains(out, "error:") {
		t.Errorf("output = %q, want at least one error", out)
	}
}

func TestLintJSONIsParseable(t *testing.T) {
	out, _ := run(t, repository(t), "lint", "--json")

	// Unmarshal on the whole buffer, not a Decoder: a Decoder stops after the
	// first value and would accept trailing text that is not JSON.
	var findings []jsonFinding
	if err := json.Unmarshal([]byte(out), &findings); err != nil {
		t.Fatalf("lint --json did not emit a JSON document: %v\n%s", err, out)
	}
}

func TestIndexWritesTheFileAndCheckAgrees(t *testing.T) {
	dir := repository(t)

	if out, code := run(t, dir, "index"); code != exitOK {
		t.Fatalf("index exited %d: %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "INDEX.md")); err != nil {
		t.Fatalf("INDEX.md was not written: %v", err)
	}
	if out, code := run(t, dir, "index", "--check"); code != exitOK {
		t.Errorf("check failed straight after generating: %d\n%s", code, out)
	}
}

func TestIndexCheckReportsDriftWithADiff(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "INDEX.md", "# Stale index\n")

	out, code := run(t, dir, "index", "--check")

	if code != exitFindings {
		t.Errorf("exit = %d, want %d", code, exitFindings)
	}
	// "+" alone is satisfied by the +++ header, so assert a real hunk.
	if !strings.Contains(out, "--- INDEX.md") || !strings.Contains(out, "@@ -") {
		t.Errorf("output is not a diff:\n%s", out)
	}
	if !strings.Contains(out, "-# Stale index") {
		t.Errorf("the replaced line is not shown as removed:\n%s", out)
	}
}

func TestIndexCheckJSONReportsOneFinding(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "INDEX.md", "# Stale index\n")

	out, code := run(t, dir, "index", "--check", "--json")

	if code != exitFindings {
		t.Errorf("exit = %d, want %d", code, exitFindings)
	}
	if !strings.Contains(out, `"rule": "index"`) || strings.Contains(out, "---") {
		t.Errorf("want one JSON finding and no diff:\n%s", out)
	}
}

func TestNewCreatesTheNextNumberedDocument(t *testing.T) {
	dir := repository(t)

	out, code := run(t, dir, "new", "rfc", "Scheduling jobs")

	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if want := "rfc/0003-scheduling-jobs.md"; !strings.Contains(out, want) {
		t.Errorf("output = %q, want the path %q", out, want)
	}
	written, err := os.ReadFile(filepath.Join(dir, "rfc", "0003-scheduling-jobs.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"id: RFC-0003", "title: Scheduling jobs", "status: draft"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the document is missing %q:\n%s", want, written)
		}
	}
}

func TestNewBackfillRecordsTheHistoricalDates(t *testing.T) {
	dir := repository(t)

	out, code := run(t, dir, "new", "adr", "Storage engine",
		"--backfill", "--created", "2023-01-05", "--decided", "2023-02-10")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	written, err := os.ReadFile(filepath.Join(dir, "adr", "0001-storage-engine.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"status: accepted", "created: 2023-01-05", "decided: 2023-02-10", "backfilled: ",
		"## Sources", "Not recorded.",
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the backfilled document is missing %q:\n%s", want, written)
		}
	}
}

func TestNewRefusesABackfilledRef(t *testing.T) {
	out, code := run(t, repository(t), "new", "ref", "A survey", "--backfill")

	if code != exitUsage || !strings.Contains(out, "no status") {
		t.Errorf("exit = %d, output = %q; want a usage error about refs having no status", code, out)
	}
}

func TestTransitionRewritesAndReports(t *testing.T) {
	dir := repository(t)

	out, code := run(t, dir, "accept", "RFC-0002")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	if !strings.Contains(out, "RFC-0002 is now accepted") {
		t.Errorf("output = %q", out)
	}

	written, err := os.ReadFile(filepath.Join(dir, "rfc", "0002-cache.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "status: accepted") {
		t.Errorf("the document was not rewritten:\n%s", written)
	}
}

func TestTransitionRefusesOneThatIsNotPermitted(t *testing.T) {
	out, code := run(t, repository(t), "propose", "RFC-0001")

	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(out, "terminal") {
		t.Errorf("output = %q, want it to explain that accepted is terminal", out)
	}
}

func TestTermListAndShow(t *testing.T) {
	dir := repository(t)

	if out, _ := run(t, dir, "term", "list"); strings.TrimSpace(out) != "Wings" {
		t.Errorf("list = %q, want %q", out, "Wings")
	}
	out, code := run(t, dir, "term", "show", "wings")
	if code != exitOK || !strings.Contains(out, "The node daemon.") {
		t.Errorf("show exited %d with %q", code, out)
	}
}

func TestTermAddInsertsAlphabetically(t *testing.T) {
	dir := repository(t)

	if out, code := run(t, dir, "term", "add", "Anchor", "A fragment identifier."); code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	out, _ := run(t, dir, "term", "list")
	if want := "Anchor\nWings"; strings.TrimSpace(out) != want {
		t.Errorf("list = %q, want %q", out, want)
	}
}

func TestMissingArgumentIsAUsageErrorWithoutATerminal(t *testing.T) {
	for _, args := range [][]string{{"new"}, {"accept"}, {"term", "add"}} {
		out, code := run(t, repository(t), args...)
		if code != exitUsage {
			t.Errorf("%v exited %d, want %d: %s", args, code, exitUsage, out)
		}
	}
}

func TestLintStrictWarningsTurnsWarningsIntoFailure(t *testing.T) {
	dir := repository(t)
	// A ref verified long ago is an L13 warning and nothing more.
	repotest.Write(t, dir, "ref/0001-old.md",
		"---\nid: REF-0001\ntitle: Old\nverified: 2019-01-01\n---\n\n# REF-0001: Old\n\n## Sources\n\n- Somewhere.\n")

	if _, code := run(t, dir, "lint"); code != exitOK {
		t.Errorf("a warning alone exited %d, want %d", code, exitOK)
	}
	out, code := run(t, dir, "lint", "--strict-warnings")
	if code != exitFindings {
		t.Errorf("exit = %d, want %d under --strict-warnings; output:\n%s", code, exitFindings, out)
	}
}

func TestLinkResolvesWikiLinksAndWritesThem(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "spec/queues.md",
		"---\ntitle: Queues\nincludes: [RFC-0001]\n---\n\n# Queues\n\nAs [[RFC-0001]] describes, a [[Wings]] node consumes them.\n")

	out, code := run(t, dir, "link")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	written, err := os.ReadFile(filepath.Join(dir, "spec", "queues.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[RFC-0001: Job queues](../rfc/0001-queues.md)",
		"[Wings](glossary.md#wings)",
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the document is missing %q:\n%s", want, written)
		}
	}
}

func TestLinkReportsAnUnresolvableLinkAndWritesNothing(t *testing.T) {
	dir := repository(t)
	before := "---\ntitle: Queues\nincludes: []\n---\n\n# Queues\n\nSee [[Nothing At All]].\n"
	repotest.Write(t, dir, "spec/queues.md", before)

	out, code := run(t, dir, "link")

	if code != exitFindings {
		t.Errorf("exit = %d, want %d", code, exitFindings)
	}
	if !strings.Contains(out, "Nothing At All") {
		t.Errorf("output = %q, want it to name the link", out)
	}
	written, _ := os.ReadFile(filepath.Join(dir, "spec", "queues.md"))
	if string(written) != before {
		t.Errorf("the document was rewritten despite the failure:\n%s", written)
	}
}

func TestDevNullIsNotATerminal(t *testing.T) {
	// A character device is not the same thing as a terminal. /dev/null is one,
	// and it is how a shell, Makefile or job runner says "no input": prompting
	// there hangs or fails with an error naming a library the user never
	// installed.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	root.SetIn(devNull)
	if interactive(root) {
		t.Error("interactive() called /dev/null a terminal")
	}
}

func TestAPipeIsNotATerminal(t *testing.T) {
	root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	root.SetIn(strings.NewReader(""))
	if interactive(root) {
		t.Error("interactive() called a pipe a terminal")
	}
}

func TestLinkSuggestDoesNotRewriteThePreResolutionSnapshot(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "spec/queues.md",
		"---\ntitle: Queues\nincludes: []\n---\n\n# Queues\n\nA link to [[Wings]] here, and a bare RFC-0001 mention.\n")

	out, code := run(t, dir, "link", "--suggest", "--apply")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	written, err := os.ReadFile(filepath.Join(dir, "spec", "queues.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "[[") {
		t.Errorf("resolution was reverted by the suggestion pass:\n%s", written)
	}
	if strings.Contains(string(written), "]](") {
		t.Errorf("a link was nested inside another:\n%s", written)
	}
}

func TestLinkResolvesEveryDocumentItCanEvenWhenOneFails(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "spec/good.md",
		"---\ntitle: Good\nincludes: []\n---\n\n# Good\n\nSee [[Wings]].\n")
	repotest.Write(t, dir, "spec/bad.md",
		"---\ntitle: Bad\nincludes: []\n---\n\n# Bad\n\nSee [[No Such Thing]].\n")

	out, code := run(t, dir, "link")
	if code != exitFindings {
		t.Errorf("exit = %d, want %d", code, exitFindings)
	}

	good, _ := os.ReadFile(filepath.Join(dir, "spec", "good.md"))
	if strings.Contains(string(good), "[[Wings]]") {
		t.Errorf("a resolvable document was left alone because another failed:\n%s\noutput: %s", good, out)
	}
	bad, _ := os.ReadFile(filepath.Join(dir, "spec", "bad.md"))
	if !strings.Contains(string(bad), "[[No Such Thing]]") {
		t.Errorf("the failing document was rewritten:\n%s", bad)
	}
}

func TestLinkJSONEmitsExactlyOneArray(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "spec/queues.md",
		"---\ntitle: Queues\nincludes: []\n---\n\n# Queues\n\nSee [[Wings]] and a bare RFC-0001.\n")

	// Separate streams here, unlike run: the whole claim is that --json puts
	// the array and nothing else on stdout, and progress notices go to stderr
	// precisely so this holds.
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	root := newRoot(&stdout, &stderr)
	root.SetArgs([]string{"--no-interaction", "link", "--suggest", "--json"})
	_ = root.Execute()

	var findings []jsonFinding
	if err := json.Unmarshal(stdout.Bytes(), &findings); err != nil {
		t.Fatalf("link --json did not emit one JSON array: %v\nstdout:\n%s", err, stdout.String())
	}
	if stderr.Len() == 0 {
		t.Error("nothing reached stderr, so this no longer distinguishes the two streams")
	}
}

func TestLinkOutputOrderIsStable(t *testing.T) {
	dir := repository(t)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		repotest.Write(t, dir, "spec/"+name+".md",
			"---\ntitle: "+name+"\nincludes: []\n---\n\n# "+name+"\n\nSee [[Wings]].\n")
	}

	first, _ := run(t, dir, "link")
	for range 8 {
		// Reset and run again: the order of the notices must not vary.
		for _, name := range []string{"a", "b", "c", "d", "e"} {
			repotest.Write(t, dir, "spec/"+name+".md",
				"---\ntitle: "+name+"\nincludes: []\n---\n\n# "+name+"\n\nSee [[Wings]].\n")
		}
		if got, _ := run(t, dir, "link"); got != first {
			t.Fatalf("output varies between runs:\n first %q\n then  %q", first, got)
		}
	}
}

func TestIndexCheckSaysSomethingWhenOnlyTheNewlineDiffers(t *testing.T) {
	dir := repository(t)
	if _, code := run(t, dir, "index"); code != exitOK {
		t.Fatal("index failed")
	}
	// An editor or a script can strip the final newline. Exiting 2 in silence
	// is what the spec reserves for the index being current.
	generated, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	repotest.Write(t, dir, "INDEX.md", strings.TrimRight(string(generated), "\n"))

	out, code := run(t, dir, "index", "--check")
	if code != exitFindings {
		t.Errorf("exit = %d, want %d", code, exitFindings)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("exited 2 having printed nothing, which is indistinguishable from success")
	}
}

func TestTermAddCreatesTheGlossaryWhenThereIsNone(t *testing.T) {
	r := repotest.New(t, map[string]string{
		"rfc/0001-queues.md": rfc("RFC-0001", "Job queues", "accepted", " 2026-02-01"),
	})
	dir := r.Config().Dir()

	out, code := run(t, dir, "term", "add", "Widget", "A thing.")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	written, err := os.ReadFile(filepath.Join(dir, "spec", "glossary.md"))
	if err != nil {
		t.Fatalf("the glossary was not created: %v", err)
	}
	for _, want := range []string{"title: Glossary", "## Widget", "A thing."} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the created glossary is missing %q:\n%s", want, written)
		}
	}
	if _, code := run(t, dir, "lint"); code != exitOK {
		t.Error("the created glossary does not pass lint")
	}
}

func TestBackfillRefusesDatesThatWouldFailLint(t *testing.T) {
	for _, tc := range []struct{ name, created, decided, want string }{
		{"decided before created", "2020-06-01", "2019-01-01", "before"},
		{"decided in the future", "2099-01-01", "2099-06-01", "future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := run(t, repository(t), "new", "adr", "Bad dates",
				"--backfill", "--created", tc.created, "--decided", tc.decided)
			if code != exitUsage {
				t.Errorf("exit = %d, want %d; output: %s", code, exitUsage, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("output = %q, want it to mention %q", out, tc.want)
			}
		})
	}
}

func TestNewRefusesATitleContainingALineBreak(t *testing.T) {
	out, code := run(t, repository(t), "new", "rfc", "Line one\nLine two")

	if code != exitUsage {
		t.Errorf("exit = %d, want %d; output: %s", code, exitUsage, out)
	}
	if !strings.Contains(out, "line break") {
		t.Errorf("output = %q, want it to name the problem", out)
	}
}

func TestBackfillDateErrorNamesTheSameFlagEveryTime(t *testing.T) {
	dir := repository(t)
	first, _ := run(t, dir, "new", "rfc", "Trial", "--backfill", "--created", "nope", "--decided", "alsonope")
	for range 8 {
		if got, _ := run(t, dir, "new", "rfc", "Trial", "--backfill", "--created", "nope", "--decided", "alsonope"); got != first {
			t.Fatalf("the error alternates between runs:\n first %q\n then  %q", first, got)
		}
	}
}

// runWith is run, with something on stdin for the interactive paths.
func runWith(t *testing.T, dir, stdin string, args ...string) (string, int) {
	t.Helper()
	t.Chdir(dir)

	var out bytes.Buffer
	root := newRoot(&out, &out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)

	err := root.Execute()
	switch {
	case err == nil:
		return out.String(), exitOK
	default:
		var quiet silentExit
		if errors.As(err, &quiet) {
			return out.String(), int(quiet)
		}
		return out.String() + "archdoc: " + err.Error() + "\n", exitUsage
	}
}

func TestLinkSuggestWithoutATerminalPrintsAndFails(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "spec/queues.md",
		"---\ntitle: Queues\nincludes: []\n---\n\n# Queues\n\nA bare RFC-0001 mention.\n")
	before, _ := os.ReadFile(filepath.Join(dir, "spec", "queues.md"))

	out, code := run(t, dir, "link", "--suggest")

	if code != exitFindings {
		t.Errorf("exit = %d, want %d", code, exitFindings)
	}
	if !strings.Contains(out, "could link to") {
		t.Errorf("output = %q, want the suggestion printed", out)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "spec", "queues.md"))
	if string(after) != string(before) {
		t.Errorf("nothing should have been written:\n%s", after)
	}
}

func TestLinkSuggestApplyRewritesTheDocument(t *testing.T) {
	dir := repository(t)
	repotest.Write(t, dir, "spec/queues.md",
		"---\ntitle: Queues\nincludes: []\n---\n\n# Queues\n\nA bare RFC-0001 mention.\n")

	if out, code := run(t, dir, "link", "--suggest", "--apply"); code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	written, _ := os.ReadFile(filepath.Join(dir, "spec", "queues.md"))
	if !strings.Contains(string(written), "[RFC-0001](../rfc/0001-queues.md)") {
		t.Errorf("the suggestion was not applied:\n%s", written)
	}
}

func TestChooseHonoursEachAnswer(t *testing.T) {
	suggestions := []link.Suggestion{
		{Path: "a.md", Line: 1, Text: "x", Replacement: "[x](y)", LineText: "x"},
		{Path: "a.md", Line: 2, Text: "x", Replacement: "[x](y)", LineText: "x"},
		{Path: "b.md", Line: 1, Text: "x", Replacement: "[x](y)", LineText: "x"},
		{Path: "c.md", Line: 1, Text: "x", Replacement: "[x](y)", LineText: "x"},
	}

	for _, tc := range []struct {
		name, input string
		want        int
	}{
		{"yes then no", "y\nn\nn\nn\n", 1},
		{"all in file takes both", "a\nn\nn\n", 2},
		{"skip file passes over it", "s\ny\ny\n", 2},
		{"quit keeps what was accepted", "y\nq\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
			root.SetIn(strings.NewReader(tc.input))

			accepted, err := choose(root, suggestions)
			if err != nil {
				t.Fatalf("choose: %v", err)
			}
			if len(accepted) != tc.want {
				t.Errorf("accepted %d, want %d: %v", len(accepted), tc.want, accepted)
			}
		})
	}
}

func TestTermRenameAndRemove(t *testing.T) {
	dir := repository(t)

	if out, code := run(t, dir, "term", "rename", "Wings", "Node daemon"); code != exitOK {
		t.Fatalf("rename exited %d: %s", code, out)
	}
	written, _ := os.ReadFile(filepath.Join(dir, "spec", "glossary.md"))
	if !strings.Contains(string(written), "## Node daemon") || !strings.Contains(string(written), "Formerly *Wings*.") {
		t.Errorf("rename did not record the change:\n%s", written)
	}

	if out, code := run(t, dir, "term", "remove", "Node daemon"); code != exitOK {
		t.Fatalf("remove exited %d: %s", code, out)
	}
	if out, _ := run(t, dir, "term", "list"); strings.TrimSpace(out) != "" {
		t.Errorf("list = %q, want nothing left", out)
	}
}

func TestTermAddFromExtendsIncludes(t *testing.T) {
	dir := repository(t)

	out, code := run(t, dir, "term", "add", "Queue", "A place work waits.", "--from", "RFC-0001")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}
	written, _ := os.ReadFile(filepath.Join(dir, "spec", "glossary.md"))
	if !strings.Contains(string(written), "includes: [RFC-0001]") {
		t.Errorf("--from did not extend includes:\n%s", written)
	}
	if _, code := run(t, dir, "lint"); code != exitOK {
		t.Error("the rewritten glossary does not pass lint")
	}
}

func TestNewRefusesATitleThatWouldNotReadBack(t *testing.T) {
	for _, title := range []string{"Caching layer #", "Trailing space   "} {
		out, code := run(t, repository(t), "new", "rfc", title)
		if code != exitUsage {
			t.Errorf("new %q exited %d, want a refusal: %s", title, code, out)
		}
	}
}

func TestNewBackfillRejectedWritesEverySectionItRequires(t *testing.T) {
	// A documented flag combination must not produce a document the tool's own
	// linter reports as incomplete before anyone has touched it.
	dir := repository(t)

	out, code := run(t, dir, "new", "rfc", "Old idea",
		"--backfill", "--status", "rejected", "--created", "2023-01-05", "--decided", "2023-02-10")
	if code != exitOK {
		t.Fatalf("exit = %d: %s", code, out)
	}

	// The fixture already holds RFC-0001 and RFC-0002.
	written, err := os.ReadFile(filepath.Join(dir, "rfc", "0003-old-idea.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"status: rejected", "## Sources", "## Rejection rationale"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the document is missing %q:\n%s", want, written)
		}
	}

	// Every required section present, and the closing one last. What remains is
	// the empty-section prompt, which is the intended nudge to finish it.
	lintOut, _ := run(t, dir, "lint")
	for _, unwanted := range []string{"missing required section", "must be the last H2", "appears before"} {
		if strings.Contains(lintOut, unwanted) {
			t.Errorf("lint reports a structural fault in a freshly created document: %s", lintOut)
		}
	}
}

func TestEnsureGlossaryRefusesAFileTheIndexMissed(t *testing.T) {
	// The existence test asked the in-memory document index, which is keyed on
	// the page name taken verbatim from the filename. On a case-insensitive
	// filesystem, and both release targets are, spec/Glossary.md is invisible
	// to that lookup while naming the same file on disk, so the write truncated
	// a glossary the user could plainly see. Asked of the filesystem instead,
	// the answer does not depend on the filename's case.
	// The index is built without a glossary, and one then appears on disk. That
	// is the shape a case-differing filename produces: the file is there and
	// the lookup cannot see it.
	r := repotest.New(t, map[string]string{
		"rfc/0001-queues.md": rfc("RFC-0001", "Job queues", "accepted", " 2026-02-01"),
	})
	dir := r.Config().Dir()
	t.Chdir(dir)

	existing := "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Widget\n\nA small thing.\n"
	repotest.Write(t, dir, "spec/glossary.md", existing)

	if _, err := ensureGlossary(r); err == nil {
		t.Error("ensureGlossary reported success over a file that already existed")
	}

	after, err := os.ReadFile(filepath.Join(dir, "spec", "glossary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != existing {
		t.Errorf("the existing glossary was overwritten:\n%s", after)
	}
}

func TestTermAddWritesNothingWhenItsArgumentsAreMissing(t *testing.T) {
	// The glossary was created before the arguments were checked, so a bare
	// invocation left a file behind and then reported a usage error.
	// A repository with no glossary yet, which is the case that writes one.
	r := repotest.New(t, map[string]string{
		"rfc/0001-queues.md": rfc("RFC-0001", "Job queues", "accepted", " 2026-02-01"),
	})
	dir := r.Config().Dir()

	out, code := run(t, dir, "term", "add")
	if code == exitOK {
		t.Fatalf("exit = %d, want a failure: %s", code, out)
	}

	if _, err := os.Stat(filepath.Join(dir, "spec", "glossary.md")); err == nil {
		t.Error("a usage error left a glossary behind")
	}
}

func TestApplySuggestionsSkipsAFileThatChangedUnderneath(t *testing.T) {
	// The snapshot is taken before the first prompt and the write happens after
	// the last answer, so the whole session is a window in which an edit made
	// in another window is silently discarded: the file is rebuilt from the
	// bytes read at open time.
	r := repotest.New(t, map[string]string{
		"spec/glossary.md": "---\ntitle: Glossary\nincludes: []\n---\n\n# Glossary\n\n## Widget\n\nA small thing.\n",
		"spec/page.md":     "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\nThe widget matters.\n",
	})
	dir := r.Config().Dir()
	t.Chdir(dir)

	suggestions := link.Suggest(r)
	if len(suggestions) == 0 {
		t.Fatal("no suggestions to apply")
	}

	// The author edits the file while the prompt is open.
	edited := "---\ntitle: Page\nincludes: []\n---\n\n# Page\n\nThe widget matters.\n\nA paragraph added while answering.\n"
	repotest.Write(t, dir, "spec/page.md", edited)

	var out bytes.Buffer
	cmd := newRoot(&out, &out)
	if err := applySuggestions(cmd, r, suggestions); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(filepath.Join(dir, "spec", "page.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != edited {
		t.Errorf("the edit made while prompting was discarded:\n%s", after)
	}
}

func TestNewBackfillDoesNotDependOnTheHourOrTheZone(t *testing.T) {
	// The guard compared a date parsed to midnight UTC against a wall-clock
	// instant, so east of UTC it refused today's date for the first hours of
	// every local day, and west of it accepted tomorrow's. The same defect was
	// fixed in L13; this copy was never looked at.
	for _, zone := range []*time.Location{
		time.UTC,
		time.FixedZone("UTC+14", 14*60*60),
		time.FixedZone("UTC-11", -11*60*60),
	} {
		for _, hour := range []int{0, 2, 9, 14, 23} {
			at := time.Date(2026, 9, 12, hour, 30, 0, 0, zone)
			today := at.Format(repo.DateLayout)

			if _, err := newData(backfillOptions{
				On: true, Status: "accepted", Created: today, Decided: today,
			}, at); err != nil {
				t.Errorf("at %s: today's date was refused: %v", at, err)
			}

			tomorrow := at.AddDate(0, 0, 1).Format(repo.DateLayout)
			if _, err := newData(backfillOptions{
				On: true, Status: "accepted", Created: tomorrow, Decided: tomorrow,
			}, at); err == nil {
				t.Errorf("at %s: tomorrow's date (%s) was accepted", at, tomorrow)
			}
		}
	}
}

func TestIndexRefusesToWriteThroughASymbolicLink(t *testing.T) {
	// git stores a symlink (mode 120000) and clone restores it, so this arrives
	// on a fresh checkout. os.WriteFile follows it and truncates the target.
	dir := repository(t)
	victim := filepath.Join(filepath.Dir(dir), "victim.txt")
	if err := os.WriteFile(victim, []byte("PRECIOUS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, indexFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	out, code := run(t, dir, "index")
	if code == exitOK {
		t.Errorf("exit = %d, want a failure: %s", code, out)
	}
	if got, _ := os.ReadFile(victim); string(got) != "PRECIOUS\n" {
		t.Errorf("the target was overwritten:\n%s", got)
	}
}

func TestInitRefusesADanglingSymlinkInTheWay(t *testing.T) {
	// os.Stat reports a dangling link absent, so the collision check passed and
	// the write then followed it, creating the file outside the tree.
	// The link stands where a LATER file goes, so that the all-or-nothing
	// promise is what is under test. On the first file, O_EXCL alone would
	// refuse and nothing would have been written either way.
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.MkdirAll(filepath.Join(dir, "spec"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "spec", "glossary.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	out, code := run(t, dir, "init", "--name", "Dangling")
	if code == exitOK {
		t.Errorf("exit = %d, want a failure: %s", code, out)
	}
	if _, err := os.Stat(outside); err == nil {
		t.Error("init wrote through the dangling link, outside the directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		t.Error("init wrote part of the scaffold before refusing")
	}
}

func TestInitWritesNothingWhenTheSettingsAreUnusable(t *testing.T) {
	// The empty-name and resolved-root checks were first reached by loading the
	// archdoc.json that had already been written, which left a dead
	// configuration behind that init then refused to overwrite.
	// A root that is lexically fine and resolves outside the tree. ValidateRoot
	// passes it, so only the full config.Validate, which init used to reach
	// after writing everything, refuses it.
	dir := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "docs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	out, code := run(t, dir, "init", "--name", "Escape", "--root", "docs")
	if code == exitOK {
		t.Fatalf("exit = %d, want a failure: %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, config.Filename)); err == nil {
		t.Error("init left an archdoc.json it cannot itself load")
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		t.Error("init wrote the scaffold before refusing")
	}
}

func TestNewRefusesValueFlagsWithoutBackfill(t *testing.T) {
	// newData ignores Status, Created and Decided entirely when --backfill is
	// off, so without this check the flags vanish: the command prints a path,
	// exits 0, and has written a draft, with no signal that the flag did
	// nothing.
	for _, flag := range [][]string{
		{"--status", "accepted"},
		{"--created", "2023-01-05"},
		{"--decided", "2023-02-10"},
	} {
		args := append([]string{"new", "rfc", "Wings"}, flag...)
		out, code := run(t, repository(t), args...)
		if code == exitOK {
			t.Errorf("%v was accepted without --backfill: %s", flag, out)
		}
	}
}

func TestNewRefusesANonTerminalBackfillStatus(t *testing.T) {
	// newData promises to refuse anything L07 would report rather than writing
	// a document the tool's own linter fails. Without the check it writes
	// decided and backfilled onto a draft and L07 reports two errors on a file
	// archdoc itself created.
	out, code := run(t, repository(t), "new", "rfc", "Wings", "--backfill", "--status", "draft")
	if code == exitOK {
		t.Errorf("--backfill --status=draft was accepted: %s", out)
	}
}

func TestNewRefusesACreatedDateItCannotParse(t *testing.T) {
	// Masked whenever --decided defaults to --created, which is why nothing
	// caught it: with an explicit --decided the zero createdOn passes both
	// remaining comparisons and the unparsed string is written straight into a
	// document that is terminal from birth.
	out, code := run(t, repository(t), "new", "rfc", "Wings",
		"--backfill", "--created", "not-a-date", "--decided", "2023-02-10")
	if code == exitOK {
		t.Errorf("an unparseable --created was accepted: %s", out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("the error does not name the flag: %s", out)
	}
}

func TestInitRefusesALicenceItDoesNotKnow(t *testing.T) {
	// plan() only writes a LICENSE for "mit", so any other spelling is a no-op
	// and this validation is the only thing that tells the user. Without it
	// init scaffolds everything, exits 0, and leaves no licence file.
	dir := t.TempDir()
	out, code := run(t, dir, "init", "--name", "L", "--license", "gpl")
	if code == exitOK {
		t.Errorf("--license gpl was accepted: %s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, config.Filename)); err == nil {
		t.Error("init scaffolded the repository despite the unknown licence")
	}
}

func TestNoInteractionIsHonouredWhateverStdinIs(t *testing.T) {
	// Every other test runs with a non-terminal stdin, so the flag check itself
	// is never exercised there. Without it the flag does nothing on a TTY and a
	// wrapper script that passes it blocks on a form instead of failing.
	// stdin is never a terminal under `go test`, so the flag check is only
	// reachable with the terminal test stubbed: without that, the check could be
	// deleted and nothing would notice.
	original := onTerminal
	onTerminal = func(*cobra.Command) bool { return true }
	t.Cleanup(func() { onTerminal = original })

	cmd := newRoot(&bytes.Buffer{}, &bytes.Buffer{})
	if !interactive(cmd) {
		t.Fatal("the stub did not take effect, so this test asserts nothing")
	}
	// Parsed rather than set directly: cobra merges persistent flags into
	// Flags() during parsing, and interactive() reads them from there.
	if err := cmd.ParseFlags([]string{"--no-interaction"}); err != nil {
		t.Fatal(err)
	}
	if interactive(cmd) {
		t.Error("interactive() reported true with --no-interaction set")
	}
}

func TestOpenGitDistinguishesAMissingGitFromAMissingRepository(t *testing.T) {
	// Only git's own "not a repository" may become a nil Repository. Anything
	// else must be an error, because without the distinction L11 skips itself
	// with a warning whenever git cannot be run at all, which is exactly the CI
	// image where nobody would notice.
	dir := t.TempDir()
	repotest.Write(t, dir, "archdoc.json", `{"name":"T"}`)
	t.Chdir(dir)

	r, err := openRepo()
	if err != nil {
		t.Fatal(err)
	}

	// Not a repository: a nil Repository and no error, so L11 skips with a warning.
	if g, err := openGit(r); err != nil || g != nil {
		t.Errorf("outside a repository: got (%v, %v), want (nil, nil)", g, err)
	}

	// git missing entirely: an error, not a silent skip.
	t.Setenv("PATH", "")
	if _, err := openGit(r); err == nil {
		t.Error("git could not be run at all and openGit reported no error")
	}
}

func TestTransitionRefusesAnIdentifierItCannotFind(t *testing.T) {
	// lifecycle.Apply dereferences its argument on the first line, so this nil
	// check is the only thing between a mistyped identifier and a crash. Every
	// other transition test passes an identifier that exists.
	for _, verb := range []string{"propose", "accept", "reject", "withdraw"} {
		out, code := run(t, repository(t), verb, "RFC-9999")
		if code == exitOK {
			t.Errorf("%s accepted an unknown identifier: %s", verb, out)
		}
		if strings.Contains(out, "panic") || strings.Contains(out, "SIGSEGV") {
			t.Errorf("%s crashed on an unknown identifier: %s", verb, out)
		}
	}
}
