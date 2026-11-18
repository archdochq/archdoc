package repo

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"archdoc.dev/internal/markdown"
)

// Changelog is the section recording the edits made to a document while it is
// proposed, one list item per entry.
const Changelog = "Changelog"

// ChangelogEntry is one entry of a Changelog section: an item of a top-level
// list in the section, whatever its marker.
type ChangelogEntry struct {
	// Line is 1-based within the file: the entry line.
	Line int
	// Date is zero where the entry is undated, including where what it begins
	// with is shaped as a date but is not one.
	Date time.Time
	// Text is the entry's lines as written, without its marker and date, each
	// later line with the item's content indentation removed where it has it,
	// a lazy line as written, and blank lines at either end dropped.
	Text string
}

// dateShape is four digits, two and two, the shape a date takes before
// anything asks whether it is a real one.
var dateShape = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}`)

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

// clBlank is a line holding nothing but spaces and tabs.
func clBlank(line string) bool { return strings.Trim(line, " \t") == "" }

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

		for _, block := range d.blocksIn(h.last+1, d.sectionEnd(i)) {
			switch {
			case block.Type == markdown.List:
				for _, item := range block.Children {
					at := d.lineAt(item.Start)
					entry, faults := d.readEntry(lines, item)
					entry.Line = d.BodyLine + at
					entries = append(entries, entry)
					for _, fault := range faults {
						report(at, "%s", fault)
					}
				}
			case passedOver(block):
			default:
				report(d.lineAt(block.Start), "this block belongs to no entry; an entry is an item of a list in the section, beginning with its date")
			}
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

// readEntry reads one list item as an entry, returning the faults in how it
// begins.
func (d *Document) readEntry(lines []string, item *markdown.Node) (ChangelogEntry, []string) {
	first, last := d.lineAt(item.Start), d.lineAt(item.End-1)
	line := lines[first]
	at := item.Start
	if first > 0 {
		at -= d.starts[first-1]
	}
	// The marker, then the spaces or tabs after it, and the column the
	// content starts at, which later lines are indented to.
	marker := at + 1
	for marker < len(line) && line[at] >= '0' && line[at] <= '9' && line[marker] >= '0' && line[marker] <= '9' {
		marker++
	}
	if line[at] >= '0' && line[at] <= '9' && marker < len(line) && (line[marker] == '.' || line[marker] == ')') {
		marker++
	}
	rest := strings.TrimLeft(line[min(marker, len(line)):], " \t")
	indent := columnAt(line, len(line)-len(rest))
	if rest == "" {
		indent = columnAt(line, marker) + 1
	}

	text := []string{rest}
	for _, l := range lines[first+1 : last+1] {
		text = append(text, outdent(l, indent))
	}
	for len(text) > 0 && clBlank(text[0]) {
		text = text[1:]
	}
	for len(text) > 0 && clBlank(text[len(text)-1]) {
		text = text[:len(text)-1]
	}

	// The date is read from the start of the entry's text, which may begin
	// on the line after its marker.
	var entry ChangelogEntry
	var faults []string
	start := ""
	if len(text) > 0 {
		start = text[0]
	}
	switch shape := dateShape.FindString(start); {
	case shape == "":
		faults = append(faults, "entry has no date; begin it with YYYY-MM-DD: ")
	case !strings.HasPrefix(start[len(shape):], ":") || (len(start) > len(shape)+1 && start[len(shape)+1] != ' '):
		faults = append(faults, fmt.Sprintf("entry begins with %s, which is not followed by a colon and a space", shape))
	default:
		date, err := time.Parse(DateLayout, shape)
		if err != nil {
			faults = append(faults, fmt.Sprintf("entry begins with %s, which is not a real date", shape))
			break
		}
		entry.Date = date
		text[0] = strings.TrimPrefix(start[len(shape)+1:], " ")
		if clBlank(text[0]) {
			text = text[1:]
		}
	}
	entry.Text = strings.Join(text, "\n")
	if !entry.Date.IsZero() && entry.Text == "" {
		faults = append(faults, "entry has a date and no text")
	}
	return entry, faults
}

// columnAt is the column of byte at in line, a tab running to the next
// multiple of four.
func columnAt(line string, at int) int {
	column := 0
	for i := 0; i < at && i < len(line); i++ {
		column = advance(column, line[i])
	}
	return column
}

// outdent removes indent columns of leading whitespace from line, a tab
// spanning past them leaving the rest as spaces, or returns a line indented
// less, a lazy line among them, as written.
func outdent(line string, indent int) string {
	column, i := 0, 0
	for i < len(line) && column < indent && (line[i] == ' ' || line[i] == '\t') {
		column = advance(column, line[i])
		i++
	}
	if column < indent {
		if clBlank(line) {
			return ""
		}
		return line
	}
	return strings.Repeat(" ", column-indent) + line[i:]
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
