package repo_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"archdoc.dev/internal/repo"
	"archdoc.dev/internal/repotest"
)

// changelogDoc is an RFC created on 2026-01-01 with the given body after its
// H1. The body starts on line 12 of the file.
func changelogDoc(t *testing.T, decided, body string) *repo.Document {
	t.Helper()
	status := "draft"
	if decided != "" {
		status = "accepted"
	}
	source := "---\nid: RFC-0001\ntitle: T\nstatus: " + status + "\ncreated: 2026-01-01\ndecided: " + decided +
		"\ndepends: []\nupdates: []\nobsoletes: []\n---\n# RFC-0001: T\n" + body
	r := repotest.New(t, map[string]string{"rfc/0001-t.md": source})
	return repotest.Document(t, r, "rfc/0001-t.md")
}

// texts renders entries as "line date text", undated as "-".
func texts(entries []repo.ChangelogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		date := "-"
		if !e.Date.IsZero() {
			date = e.Date.Format(repo.DateLayout)
		}
		out = append(out, fmt.Sprint(e.Line, " ", date, " ", e.Text))
	}
	return out
}

// problemLines renders problems as "line: message".
func problemLines(problems []repo.Problem) []string {
	out := make([]string, 0, len(problems))
	for _, p := range problems {
		out = append(out, fmt.Sprintf("%d: %s", p.Line, p.Message))
	}
	return out
}

func TestChangelogEntriesAreTheListItemsCommonMarkReads(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want []string
	}{
		{
			name: "wrapped lines indented beneath the entry",
			body: "- 2026-09-02: Moved the question\n  to Resolved questions.\n- 2026-09-03: Checked dates.\n",
			want: []string{"13 2026-09-02 Moved the question\nto Resolved questions.", "15 2026-09-03 Checked dates."},
		},
		{
			name: "a lazy line directly after the entry's text",
			body: "- 2026-09-02: Moved\nthe question.\n",
			want: []string{"13 2026-09-02 Moved\nthe question."},
		},
		{
			name: "a second paragraph after a blank line",
			body: "- 2026-09-02: One.\n\n\n  Two.\n",
			want: []string{"13 2026-09-02 One.\n\n\nTwo."},
		},
		{
			name: "a nested item, whose date is text",
			body: "- 2026-09-02: One.\n  - 2026-01-01: detail\n",
			want: []string{"13 2026-09-02 One.\n- 2026-01-01: detail"},
		},
		{
			name: "a code block indented within the item",
			body: "- 2026-09-02: One.\n\n      code\n",
			want: []string{"13 2026-09-02 One.\n\n    code"},
		},
		{
			name: "a comment between entries, and one that is a whole line",
			body: "<!--\nnot an entry\n- 2026-09-01: hidden\n-->\n- 2026-09-02: One.\n<!-->\n- 2026-09-03: Two.\n",
			want: []string{"17 2026-09-02 One.", "19 2026-09-03 Two."},
		},
		{
			name: "a carriage return belongs to the line ending",
			body: "- 2026-09-02: One\r\n  more.\r\n- 2026-09-03:\r\n  Two.\r\n",
			want: []string{"13 2026-09-02 One\nmore.", "15 2026-09-03 Two."},
		},
		{
			name: "a line beginning - outside the section is not an entry",
			body: "\n## Changelog\n\n- 2026-09-02: One.\n\n## Other\n\n- 2026-09-03: Not read.\n",
			want: []string{"15 2026-09-02 One."},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			body := c.body
			if !strings.Contains(body, "## Changelog") {
				body = "## Changelog\n" + body
			}
			d := changelogDoc(t, "", body)
			if got := texts(d.Changelog()); strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("entries:\n got %q\nwant %q", got, c.want)
			}
			if problems := d.ChangelogProblems(time.Time{}); len(problems) > 0 {
				t.Errorf("reported %q, want nothing", problemLines(problems))
			}
		})
	}
}

func TestChangelogReportsWhatItCannotReadAsCommonMarkDoes(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want string
	}{
		{"prose before the first entry", "Edits:\n- 2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"a paragraph after a blank line", "- 2026-09-02: One.\n\nTwo.\n", "15: this line belongs to no entry"},
		{"a lazy line after an indented one", "- 2026-09-02: One.\n  two\nthree\n", "15: this line belongs to no entry"},
		{"a lazy line opening a block", "- 2026-09-02: One.\n> two\n", "14: this line belongs to no entry"},
		{"a footnote definition", "- 2026-09-02: One.\n[^1]: note\n", "14: this line belongs to no entry"},
		{"an ordered item", "- 2026-09-02: One.\n1. two\n", "14: this line belongs to no entry"},
		{"another marker", "* 2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"a thematic break", "- - -\n", "13: this line belongs to no entry"},
		{"a bare marker", "-\n  2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"two spaces after the marker", "-  2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"a tab after the marker", "- \t2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"a lazy + item", "- 2026-09-02: One.\n+ two\n", "14: this line belongs to no entry"},
		{"a lazy _", "- 2026-09-02: One.\n_two_\n", "14: this line belongs to no entry"},
		{"a lazy =", "- 2026-09-02: One.\n===\n", "14: this line belongs to no entry"},
		{"a lazy table row", "- 2026-09-02: One.\n| a |\n", "14: this line belongs to no entry"},
		{"a lazy backtick fence", "- 2026-09-02: One.\n```\n", "14: this line belongs to no entry"},
		{"a lazy tilde fence", "- 2026-09-02: One.\n~~~\n", "14: this line belongs to no entry"},
		{"a tab in the indentation", "- 2026-09-02: One.\n\ttwo\n", "14: this line belongs to no entry"},
		{"a tab after two spaces", "- 2026-09-02: One.\n  \ttwo\n", "14: this line belongs to no entry"},
		{"one space of indentation", "- 2026-09-02: One.\n two\n", "14: this line belongs to no entry"},
		{"a fence at column 0", "```\ncode\n```\n", "13: this line belongs to no entry"},
		{"a level three heading", "### 2026\n- 2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"a comment closing before other text", "<!-- note --> more\n", "13: this line belongs to no entry"},
		{"a comment that never closes", "<!-- note\n- 2026-09-02: One.\n", "13: this line belongs to no entry"},
		{"no date", "- Moved it.\n", "13: entry has no date"},
		{"a date without its colon", "- 2026-09-02 Moved it.\n", "13: entry begins with 2026-09-02, which is not followed by a colon"},
		{"a date with no space after its colon", "- 2026-09-02:Moved it.\n", "13: entry begins with 2026-09-02, which is not followed by a colon"},
		{"a date that is not one", "- 2026-02-30: Moved it.\n", "13: entry begins with 2026-02-30, which is not a real date"},
		{"a date and no text", "- 2026-09-02:\n", "13: entry has a date and no text"},
		{"dates out of order", "- 2026-09-02: One.\n- 2026-09-01: Two.\n", "14: entry dated 2026-09-01 is earlier than the entry before it"},
		{"a date before created", "- 2025-12-31: One.\n", "13: entry dated 2025-12-31 is before the document was created"},
		{"an indented closing heading", "- 2026-09-02: One.\n  ## Next\n", "14: \"Next\" is indented by 2 spaces"},
		{"a closing heading only once masked", "- 2026-09-02: One.\n#<!-- c -->Next\n", "14: \"Next\" is a heading only once a comment on its line is masked"},
		{"a misspelt heading", "- 2026-09-02: One.\n\n## Change-Log\n", "15: \"Change-Log\" is not read as the \"Changelog\" section"},
		{"a second section", "- 2026-09-02: One.\n\n## Changelog\n\n- 2026-09-03: Two.\n", "15: the document carries more than one \"Changelog\" section"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := changelogDoc(t, "", "## Changelog\n"+c.body)
			got := problemLines(d.ChangelogProblems(time.Time{}))
			if len(got) != 1 || !strings.HasPrefix(got[0], c.want) {
				t.Errorf("reported %q, want one finding beginning %q", got, c.want)
			}
		})
	}
}

func TestChangelogReportsOneFindingPerGapBetweenEntries(t *testing.T) {
	d := changelogDoc(t, "", "## Changelog\nOne\nTwo\n\nThree\n- 2026-09-02: One.\n\nFour\n")
	got := problemLines(d.ChangelogProblems(time.Time{}))
	if len(got) != 2 || !strings.HasPrefix(got[0], "13:") || !strings.HasPrefix(got[1], "19:") {
		t.Errorf("reported %q, want the first line of each gap, 13 and 19", got)
	}
}

func TestChangelogDatesAreCheckedAgainstTheDecidedDateGiven(t *testing.T) {
	d := changelogDoc(t, "2026-09-02", "## Changelog\n- 2026-09-03: One.\n")
	if got := problemLines(d.ChangelogProblems(d.FrontMatter.Decided)); len(got) != 1 ||
		!strings.Contains(got[0], "after the document was decided, on 2026-09-02") {
		t.Errorf("against its own decided date, reported %q", got)
	}
	later := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	if got := d.ChangelogProblems(later); len(got) != 0 {
		t.Errorf("against a later decided date, reported %q", problemLines(got))
	}
	// Inverted dates are L07's to report, so the range is not checked.
	before := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := d.ChangelogProblems(before); len(got) != 0 {
		t.Errorf("against a decided date before created, reported %q", problemLines(got))
	}
}

func TestAnIndentedChangelogHeadingIsReported(t *testing.T) {
	d := changelogDoc(t, "", "## Abstract\n\n- a list\n  ## Changelog\n- 2026-09-02: One.\n")
	got := problemLines(d.ChangelogProblems(time.Time{}))
	if len(got) != 1 || !strings.HasPrefix(got[0], "15: \"Changelog\" is indented by 2 spaces") {
		t.Errorf("reported %q", got)
	}
}

func TestAnUndatedEntryIsReadWithWhatWasWritten(t *testing.T) {
	d := changelogDoc(t, "", "## Changelog\n- Moved the question.\n- 2026-02-30: Not a date.\n")
	want := []string{"13 - Moved the question.", "14 - 2026-02-30: Not a date."}
	if got := texts(d.Changelog()); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("entries:\n got %q\nwant %q", got, want)
	}
}

func TestALazyLineCannotFollowAnEntryOpeningABlock(t *testing.T) {
	// CommonMark reads "# Heading" as a heading inside the item, which takes
	// no lazy continuation, so the line after it leaves the item.
	d := changelogDoc(t, "", "## Changelog\n- # Heading\nthree\n")
	got := problemLines(d.ChangelogProblems(time.Time{}))
	if len(got) != 2 || !strings.HasPrefix(got[1], "14: this line belongs to no entry") {
		t.Errorf("reported %q, want the undated entry and then line 14", got)
	}
}

func TestAHeadingIndentedOneSpaceEndsTheSectionAsItRenders(t *testing.T) {
	// Below the item's content column, so CommonMark ends the list there too.
	d := changelogDoc(t, "", "## Changelog\n- 2026-09-02: One.\n ## Next\n")
	if got := d.ChangelogProblems(time.Time{}); len(got) != 0 {
		t.Errorf("reported %q", problemLines(got))
	}
}

func TestASecondSectionsHeadingIsCheckedOnce(t *testing.T) {
	d := changelogDoc(t, "", "## Changelog\n- 2026-09-02: One.\n  ## Changelog\n- 2026-09-03: Two.\n")
	indented := 0
	for _, p := range problemLines(d.ChangelogProblems(time.Time{})) {
		if strings.Contains(p, "is indented by 2 spaces") {
			indented++
		}
	}
	if indented != 1 {
		t.Errorf("the indented heading was reported %d times, want once", indented)
	}
}

func TestTwoDashesAreAnEntryHoldingAList(t *testing.T) {
	// Fewer than three dashes is not a thematic break.
	d := changelogDoc(t, "", "## Changelog\n- -\n")
	got := problemLines(d.ChangelogProblems(time.Time{}))
	if entries := d.Changelog(); len(entries) != 1 || entries[0].Text != "-" {
		t.Errorf("entries = %+v, want one holding \"-\"", entries)
	}
	if len(got) != 1 || !strings.Contains(got[0], "entry has no date") {
		t.Errorf("reported %q, want only the missing date", got)
	}
}
