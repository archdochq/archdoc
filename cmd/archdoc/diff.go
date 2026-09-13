package main

import (
	"fmt"
	"strings"
)

// contextLines is how much unchanged text surrounds each hunk, as diff -u uses.
const contextLines = 3

// unifiedDiff renders the difference between two texts. It exists because the
// index check is specified to show what changed, and the dependency list does
// not run to a diff library. An index is a few hundred lines, so the quadratic
// table below is not worth avoiding.
func unifiedDiff(name, before, after string) string {
	edits := diffLines(splitLines(before), splitLines(after))

	var hunks []string
	for _, h := range group(edits) {
		hunks = append(hunks, render(h, edits))
	}
	if len(hunks) == 0 {
		return ""
	}
	return fmt.Sprintf("--- %s\n+++ %s (generated)\n%s", name, name, strings.Join(hunks, ""))
}

// splitLines divides text into lines, discarding the empty element a trailing
// newline produces so that it is not reported as a change.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// edit is one line of the result: kept, removed or added.
type edit struct {
	op   byte // ' ', '-' or '+'
	text string
	// oldLine and newLine are 1-based positions, 0 where the line is absent
	// from that side.
	oldLine, newLine int
}

// diffLines produces an edit script from the longest common subsequence.
func diffLines(old, updated []string) []edit {
	// lengths[i][j] is the LCS length of old[i:] and new[j:].
	lengths := make([][]int, len(old)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(updated)+1)
	}
	for i := len(old) - 1; i >= 0; i-- {
		for j := len(updated) - 1; j >= 0; j-- {
			if old[i] == updated[j] {
				lengths[i][j] = lengths[i+1][j+1] + 1
				continue
			}
			lengths[i][j] = max(lengths[i+1][j], lengths[i][j+1])
		}
	}

	var edits []edit
	i, j := 0, 0
	for i < len(old) && j < len(updated) {
		switch {
		case old[i] == updated[j]:
			edits = append(edits, edit{' ', old[i], i + 1, j + 1})
			i, j = i+1, j+1
		case lengths[i+1][j] >= lengths[i][j+1]:
			edits = append(edits, edit{'-', old[i], i + 1, 0})
			i++
		default:
			edits = append(edits, edit{'+', updated[j], 0, j + 1})
			j++
		}
	}
	for ; i < len(old); i++ {
		edits = append(edits, edit{'-', old[i], i + 1, 0})
	}
	for ; j < len(updated); j++ {
		edits = append(edits, edit{'+', updated[j], 0, j + 1})
	}
	return edits
}

// span is a run of edits to render together, by index into the edit script.
type span struct{ start, end int }

// group gathers changed lines into hunks. Every index within the context
// window of a change is marked, then each run of marks is one hunk: two changes
// whose windows touch produce one contiguous run, so merging needs no rule of
// its own.
func group(edits []edit) []span {
	keep := make([]bool, len(edits))
	for i, e := range edits {
		if e.op == ' ' {
			continue
		}
		for j := max(0, i-contextLines); j < min(len(edits), i+contextLines+1); j++ {
			keep[j] = true
		}
	}

	var spans []span
	for i := 0; i < len(edits); {
		if !keep[i] {
			i++
			continue
		}
		start := i
		for i < len(edits) && keep[i] {
			i++
		}
		spans = append(spans, span{start, i})
	}
	return spans
}

// render writes one hunk with its @@ header.
func render(s span, edits []edit) string {
	var b strings.Builder
	oldStart, newStart, oldCount, newCount := 0, 0, 0, 0
	for _, e := range edits[s.start:s.end] {
		if e.oldLine > 0 {
			if oldStart == 0 {
				oldStart = e.oldLine
			}
			oldCount++
		}
		if e.newLine > 0 {
			if newStart == 0 {
				newStart = e.newLine
			}
			newCount++
		}
	}
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for _, e := range edits[s.start:s.end] {
		fmt.Fprintf(&b, "%c%s\n", e.op, e.text)
	}
	return b.String()
}
