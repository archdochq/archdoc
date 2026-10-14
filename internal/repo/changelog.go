package repo

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// Changelog is the section recording the edits made to a document while it is
// proposed, one list item per entry.
const Changelog = "Changelog"

// ChangelogEntry is one entry of a Changelog section: a list item beginning
// "- " at column 0, and the lines CommonMark gives that item.
type ChangelogEntry struct {
	// Line is 1-based within the file: the entry line.
	Line int
	// Date is zero where the entry is undated, including where what it begins
	// with is shaped as a date but is not one.
	Date time.Time
	// Text is the entry without its marker and date, each later line with the
	// two columns of the item's indentation removed, and blank lines at either
	// end dropped.
	Text string
}

var (
	// dateShape is four digits, two and two, the shape a date takes before
	// anything asks whether it is a real one.
	dateShape = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	// orderedMarker begins an ordered list item, which a lazy line may not be.
	orderedMarker = regexp.MustCompile(`^[0-9]+[.)]`)
)

// Changelog reads the document's Changelog entries, from every Changelog
// section in document order.
func (d *Document) Changelog() []ChangelogEntry {
	entries, _ := d.readChangelog(time.Time{})
	return entries
}

// ChangelogProblems is every fault L21 reports in the document's Changelog.
// decided is the date entries may not come after, which is the document's own
// for lint and the date about to be written for the accept and reject gates.
func (d *Document) ChangelogProblems(decided time.Time) []Problem {
	_, problems := d.readChangelog(decided)
	return problems
}

// The reading below is a subset of CommonMark's list syntax in which the tool's
// reading of an item and CommonMark's agree. Any line outside it belongs to no
// entry and is reported, rather than read in a way that renders differently.

// clBlank is a line holding nothing but spaces and tabs.
func clBlank(line string) bool { return strings.Trim(line, " \t") == "" }

// entryLine is "- " at column 0 with something other than whitespace after the
// space, that is not a thematic break.
func entryLine(line string) bool {
	if !strings.HasPrefix(line, "- ") || len(line) < 3 || line[2] == ' ' || line[2] == '\t' {
		return false
	}
	return strings.Trim(line, "- \t") != "" || strings.Count(line, "-") < 3
}

// blockOpener is a line that may begin a block other than a paragraph, so it
// cannot be read as a lazy continuation. The set is wider than CommonMark's,
// which only ever reports a line that would have been joined.
func blockOpener(line string) bool {
	return line != "" && (strings.ContainsRune("#><-*+_=|[`~", rune(line[0])) || orderedMarker.MatchString(line))
}

// indented is a line indented by two or more spaces, with no tab before its
// first non-blank character, which puts it within the item.
func indented(line string) bool {
	if !strings.HasPrefix(line, "  ") || clBlank(line) {
		return false
	}
	lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	return !strings.Contains(lead, "\t")
}

// clLines is the body's lines with each carriage return before a line feed
// removed, which belongs to the line ending.
func (d *Document) clLines() []string {
	raw := d.bodyLines()
	lines := make([]string, len(raw))
	for i, line := range raw {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

func (d *Document) readChangelog(decided time.Time) ([]ChangelogEntry, []Problem) {
	lines := d.clLines()
	var entries []ChangelogEntry
	var problems []Problem
	report := func(index int, format string, args ...any) {
		problems = append(problems, Problem{Line: d.BodyLine + index, Message: fmt.Sprintf(format, args...)})
	}

	seen := false
	for i, h := range d.headings {
		if h.Level != 2 {
			continue
		}
		if h.Text != Changelog {
			if normalised(h.Text) == normalised(Changelog) {
				report(h.index, "%q is not read as the %q section; spell the heading %q", h.Text, Changelog, Changelog)
			}
			continue
		}
		if seen {
			report(h.index, "the document carries more than one %q section", Changelog)
		}
		seen = true

		end := d.sectionEnd(i)
		var closing *Heading
		if next := d.nextSection(i); next >= 0 {
			closing = &d.headings[next]
		}
		// A second Changelog section's heading is checked as the one opening
		// it, rather than twice.
		if closing != nil && closing.Level == 2 && closing.Text == Changelog {
			closing = nil
		}
		for _, edge := range []*Heading{&h, closing} {
			if edge == nil {
				continue
			}
			written := lines[edge.index]
			switch indent := len(written) - len(strings.TrimLeft(written, " ")); {
			case !IsHeading(written):
				report(edge.index, "%q is a heading only once a comment on its line is masked, and renders as something else", edge.Text)
			case indent >= 2:
				report(edge.index, "%q is indented by %d spaces, so it may render inside the list item before it", edge.Text, indent)
			}
		}

		gapReported := false
		for j := h.index + 1; j < end; {
			line := lines[j]
			switch {
			case entryLine(line):
				entry, next, faults := readEntry(lines, j, end)
				entry.Line = d.BodyLine + j
				entries = append(entries, entry)
				for _, fault := range faults {
					report(j, "%s", fault)
				}
				j, gapReported = next, false
				continue
			case clBlank(line):
				j++
				continue
			case strings.HasPrefix(line, commentOpen):
				if last, ok := commentLines(lines, j, end); ok {
					j = last + 1
					continue
				}
			}
			if !gapReported {
				report(j, "this line belongs to no entry; an entry is a list item beginning \"- \" at column 0, continued on lines indented by two spaces")
				gapReported = true
			}
			j++
		}
	}

	// Order and range are across every section, as one list.
	created := d.FrontMatter.Created
	inverted := !created.IsZero() && !decided.IsZero() && decided.Before(created)
	var previous time.Time
	for _, e := range entries {
		if e.Date.IsZero() {
			continue
		}
		at := e.Line - d.BodyLine
		if !previous.IsZero() && e.Date.Before(previous) {
			report(at, "entry dated %s is earlier than the entry before it, dated %s", e.Date.Format(DateLayout), previous.Format(DateLayout))
		}
		previous = e.Date
		if inverted {
			continue
		}
		if !created.IsZero() && e.Date.Before(created) {
			report(at, "entry dated %s is before the document was created, on %s", e.Date.Format(DateLayout), created.Format(DateLayout))
		}
		if !decided.IsZero() && e.Date.After(decided) {
			report(at, "entry dated %s is after the document was decided, on %s", e.Date.Format(DateLayout), decided.Format(DateLayout))
		}
	}
	slices.SortStableFunc(problems, func(a, b Problem) int { return a.Line - b.Line })
	return entries, problems
}

// commentLines finds the end of a comment opening at column 0 outside every
// entry: the first line, starting with the opening one, that holds "-->". ok is
// false where there is none in the section, or where that line does not end
// with it, because the text after it renders while the tool reads it as part
// of nothing.
func commentLines(lines []string, from, end int) (last int, ok bool) {
	for i := from; i < end; i++ {
		if strings.Contains(lines[i], commentClose) {
			return i, strings.HasSuffix(lines[i], commentClose)
		}
	}
	return 0, false
}

// readEntry reads the entry whose entry line is at index from, returning it, the
// index after its last line, and the faults in how it begins.
func readEntry(lines []string, from, end int) (ChangelogEntry, int, []string) {
	rest := lines[from][2:]
	lazy := !blockOpener(rest)
	afterText := true // the entry line, or a lazy continuation of it
	last := from
	for j := from + 1; j < end; {
		line := lines[j]
		switch {
		case indented(line):
			last, afterText = j, false
			j++
			continue
		case clBlank(line):
			k := j
			for k < end && clBlank(lines[k]) {
				k++
			}
			if k < end && indented(lines[k]) {
				j = k
				continue
			}
		case lazy && afterText && line[0] != ' ' && line[0] != '\t' && !blockOpener(line):
			last = j
			j++
			continue
		}
		break
	}

	var entry ChangelogEntry
	var faults []string
	first := rest
	switch shape := dateShape.FindString(rest); {
	case shape == "":
		faults = append(faults, "entry has no date; begin it with YYYY-MM-DD: ")
	case !strings.HasPrefix(rest[len(shape):], ":") || (len(rest) > len(shape)+1 && rest[len(shape)+1] != ' '):
		faults = append(faults, fmt.Sprintf("entry begins with %s, which is not followed by a colon and a space", shape))
	default:
		date, err := time.Parse(DateLayout, shape)
		if err != nil {
			faults = append(faults, fmt.Sprintf("entry begins with %s, which is not a real date", shape))
			break
		}
		entry.Date = date
		first = strings.TrimPrefix(rest[len(shape)+1:], " ")
	}

	text := []string{first}
	for _, line := range lines[from+1 : last+1] {
		text = append(text, strings.TrimPrefix(line, "  "))
	}
	for len(text) > 0 && clBlank(text[0]) {
		text = text[1:]
	}
	for len(text) > 0 && clBlank(text[len(text)-1]) {
		text = text[:len(text)-1]
	}
	entry.Text = strings.Join(text, "\n")
	if !entry.Date.IsZero() && entry.Text == "" {
		faults = append(faults, "entry has a date and no text")
	}
	return entry, last + 1, faults
}

// normalised is a heading reduced to its letters and digits, lowercased, which
// is how a heading meant as Changelog but spelled differently is recognised.
func normalised(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
