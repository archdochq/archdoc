package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestUnifiedDiffReportsNothingForIdenticalText(t *testing.T) {
	if got := unifiedDiff("INDEX.md", "a\nb\n", "a\nb\n"); got != "" {
		t.Errorf("diff = %q, want empty", got)
	}
}

func TestUnifiedDiffShowsTheChangedLinesWithContext(t *testing.T) {
	before := "one\ntwo\nthree\nfour\nfive\n"
	after := "one\ntwo\nTHREE\nfour\nfive\n"

	got := unifiedDiff("INDEX.md", before, after)

	for _, want := range []string{
		"--- INDEX.md",
		"+++ INDEX.md (generated)",
		"-three",
		"+THREE",
		" two",  // context before
		" four", // context after
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diff is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "-one") || strings.Contains(got, "+one") {
		t.Errorf("an unchanged line was reported as changed:\n%s", got)
	}
}

func TestUnifiedDiffHandlesAdditionsAndRemovals(t *testing.T) {
	got := unifiedDiff("INDEX.md", "a\nb\nc\n", "a\nc\nd\n")

	if !strings.Contains(got, "-b") {
		t.Errorf("a removed line was not reported:\n%s", got)
	}
	if !strings.Contains(got, "+d") {
		t.Errorf("an added line was not reported:\n%s", got)
	}
}

func TestUnifiedDiffHandlesAnEmptySide(t *testing.T) {
	// A missing index is the common case for --check on a fresh checkout.
	got := unifiedDiff("INDEX.md", "", "a\nb\n")

	if !strings.Contains(got, "+a") || !strings.Contains(got, "+b") {
		t.Errorf("every line should be an addition:\n%s", got)
	}
	// The header lines both begin with a dash, so only the body is checked.
	for _, line := range strings.Split(got, "\n")[2:] {
		if strings.HasPrefix(line, "-") {
			t.Errorf("nothing was removed, so no line should start with -:\n%s", got)
		}
	}
}

func TestUnifiedDiffHeaderCountsMatchTheHunk(t *testing.T) {
	// Nothing else asserts an @@ header, so the arithmetic behind it is
	// otherwise unguarded.
	got := unifiedDiff("INDEX.md", "one\ntwo\nthree\n", "one\nTWO\nthree\n")

	want := "--- INDEX.md\n+++ INDEX.md (generated)\n@@ -1,3 +1,3 @@\n one\n-two\n+TWO\n three\n"
	if got != want {
		t.Errorf("diff =\n%q\nwant\n%q", got, want)
	}
}

func TestUnifiedDiffSplitsDistantChangesAndMergesCloseOnes(t *testing.T) {
	lines := func(n int, changed ...int) string {
		var b strings.Builder
		for i := range n {
			text := fmt.Sprintf("line %d", i)
			if slices.Contains(changed, i) {
				text = fmt.Sprintf("CHANGED %d", i)
			}
			b.WriteString(text + "\n")
		}
		return b.String()
	}
	before := lines(40)

	// A gap of six unchanged lines keeps the two windows touching: one hunk.
	if got := unifiedDiff("f", before, lines(40, 5, 12)); strings.Count(got, "@@ -") != 1 {
		t.Errorf("changes six apart should be one hunk, got %d:\n%s", strings.Count(got, "@@ -"), got)
	}
	// A gap of twenty does not: two hunks.
	if got := unifiedDiff("f", before, lines(40, 5, 26)); strings.Count(got, "@@ -") != 2 {
		t.Errorf("changes twenty apart should be two hunks, got %d:\n%s", strings.Count(got, "@@ -"), got)
	}
}
