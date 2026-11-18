package repo

import (
	"fmt"
	"slices"
	"strings"

	"archdoc.dev/internal/markdown"
)

// The question sections. A question is a level three heading in one, and a
// question in Resolved questions ends on a blockquote holding its answer, so an
// answered question is kept rather than deleted.
const (
	OpenQuestions     = "Open questions"
	ResolvedQuestions = "Resolved questions"
)

// Question is one level three heading in a question section.
type Question struct {
	Heading Heading
	// Detail is the lines from the line after the heading to the answer, or to
	// the next heading or the end of the document where there is no answer, as
	// written.
	Detail string
	// Answer is the blockquote a resolved question ends with. Nil for an open
	// question, and for a resolved one that ends with something else.
	Answer *Answer
}

// Answer is the blockquote a resolved question ends with.
type Answer struct {
	// Line is 1-based within the file: the blockquote's first line.
	Line int
	// Text is the blockquote's lines with the leading > and one column of the
	// whitespace after it removed from each, as CommonMark removes them, so it
	// is Markdown without the quote.
	Text string
}

// Questions are a document's questions, each list in document order across
// every section of its name.
type Questions struct {
	Open     []Question
	Resolved []Question
}

// Questions reads the document's question sections.
func (d *Document) Questions() Questions {
	questions, _ := d.readQuestions(false)
	return questions
}

// QuestionProblems is every fault in the shape of the question sections, which
// L20 reports and the accept and reject gates refuse. skipOpen leaves out the
// Open questions sections, which L09 checks on an accepted document instead.
func (d *Document) QuestionProblems(skipOpen bool) []Problem {
	_, problems := d.readQuestions(skipOpen)
	return problems
}

// span is a half-open run of body line indexes.
type span struct{ start, end int }

// readQuestions reads every question section from the outline: a question
// is a top-level level three heading in one, its detail runs to the next
// top-level heading, and it ends with a blockquote when its last top-level
// block, passing over comment blocks and link reference definitions, is one.
func (d *Document) readQuestions(skipOpen bool) (Questions, []Problem) {
	var out Questions
	var problems []Problem
	report := func(index int, format string, args ...any) {
		problems = append(problems, Problem{Line: d.BodyLine + index, Message: fmt.Sprintf(format, args...)})
	}

	hasOpen := false
	questionAt := map[int]bool{}
	for j, h := range d.headings {
		if h.Level == 2 && h.Text == OpenQuestions {
			hasOpen = true
		}
		if d.isQuestion(j) {
			questionAt[h.Line] = true
		}
	}

	resolvedSeen := false
	for i, h := range d.headings {
		if h.Level != 2 || (h.Text != OpenQuestions && h.Text != ResolvedQuestions) {
			continue
		}
		open := h.Text == OpenQuestions
		section := span{h.last + 1, d.sectionEnd(i)}

		// The rows about a Resolved questions section itself apply to the
		// first alone; a later one is reported once, at its heading, and its
		// questions are checked as the first's are.
		first := true
		if !open {
			if resolvedSeen {
				report(h.index, "the document carries more than one %q section", ResolvedQuestions)
				first = false
			}
			resolvedSeen = true
		}
		check := !open || !skipOpen
		if first && !open && hasOpen && !d.followsOpen(i) {
			report(h.index, "%q must be the next level two heading after %q", ResolvedQuestions, OpenQuestions)
		}

		var questions []int
		for j, q := range d.headings {
			if q.index >= section.start && q.index < section.end && q.Level == 3 {
				questions = append(questions, j)
			}
		}
		var held []*markdown.Node
		for _, b := range d.blocksIn(section.start, section.end) {
			if !isCommentBlock(b) {
				held = append(held, b)
			}
		}
		if first && !open && len(held) == 0 {
			report(h.index, "the %q section is empty; delete it, or move an answered question into it", ResolvedQuestions)
		} else if first && check {
			before := section.end
			if len(questions) > 0 {
				before = d.headings[questions[0]].index
			}
			if len(held) > 0 && d.lineAt(held[0].Start) < before {
				if len(questions) == 0 {
					report(d.lineAt(held[0].Start), "the %q section holds something and no question; each question is a level three heading", h.Text)
				} else {
					report(d.lineAt(held[0].Start), "the %q section holds something before its first question", h.Text)
				}
			}
		}

		for n, qi := range questions {
			q := d.readQuestion(qi, open, check, report)
			// A heading beneath the question, reported where it stands.
			stop := section.end
			if n+1 < len(questions) {
				stop = d.headings[questions[n+1]].index
			}
			beneath := false
			for _, sub := range d.headings[qi+1:] {
				if sub.index >= stop {
					break
				}
				beneath = true
				if check {
					report(sub.index, "question %q has a heading beneath it; detail beneath a question is prose", q.Heading.Text)
				}
			}
			if check && !open && !beneath && q.Answer == nil {
				report(q.Heading.index, "resolved question %q does not end with a blockquote holding its answer", q.Heading.Text)
			}
			if check {
				d.checkAnchor(d.headings[qi], questionAt, report)
			}
			if open {
				out.Open = append(out.Open, q)
			} else {
				out.Resolved = append(out.Resolved, q)
			}
		}
	}
	slices.SortStableFunc(problems, func(a, b Problem) int { return a.Line - b.Line })
	return out, problems
}

// followsOpen reports whether the level two or higher heading before heading i
// is an Open questions section.
func (d *Document) followsOpen(i int) bool {
	for j := i - 1; j >= 0; j-- {
		if h := d.headings[j]; h.Level <= 2 {
			return h.Level == 2 && h.Text == OpenQuestions
		}
	}
	return false
}

// readQuestion builds the question at heading qi, reporting the faults in how
// it ends.
func (d *Document) readQuestion(qi int, open, check bool, report func(int, string, ...any)) Question {
	h := d.headings[qi]
	lines := d.bodyLines()
	end := len(lines)
	if qi+1 < len(d.headings) {
		end = max(d.headings[qi+1].index, h.last+1)
	}
	q := Question{Heading: h}
	var kept []*markdown.Node
	for _, b := range d.blocksIn(h.last+1, end) {
		if !passedOver(b) {
			kept = append(kept, b)
		}
	}
	detailEnd := end
	if n := len(kept); n > 0 && kept[n-1].Type == markdown.Blockquote {
		last := kept[n-1]
		at := d.lineAt(last.Start)
		switch {
		case open:
			if check {
				report(at, "open question %q ends with a blockquote, which would read as its answer once it is resolved", h.Text)
			}
		default:
			detailEnd = at
			q.Answer = d.answer(last)
			if check {
				if n > 1 && kept[n-2].Type == markdown.Blockquote {
					report(at, "the answer to %q has another blockquote before it; join them with a line holding only >", h.Text)
				}
				if !slices.ContainsFunc(last.Children, func(c *markdown.Node) bool { return !isCommentBlock(c) }) {
					report(at, "the answer to %q is empty", h.Text)
				}
			}
		}
	}
	q.Detail = strings.Join(lines[h.last+1:max(detailEnd, h.last+1)], "\n")
	return q
}

// answer is a blockquote's lines with any indentation before each >, the >
// and one column after it removed, and a lazy line as written.
func (d *Document) answer(quote *markdown.Node) *Answer {
	raw := d.bodyLines()
	first, last := d.lineAt(quote.Start), d.lineAt(quote.End-1)
	lines := make([]string, 0, last-first+1)
	for i := first; i <= last; i++ {
		lines = append(lines, unquote(strings.TrimSuffix(raw[i], "\r")))
	}
	return &Answer{Line: d.BodyLine + first, Text: strings.Join(lines, "\n")}
}

// unquote removes a blockquote line's indentation, its > and one column of
// the whitespace after it, as CommonMark does, or returns a lazy line, which
// has no marker, as written. The whitespace before the text is expanded to
// spaces at the columns it occupied inside the quote first, because a tab
// moved left changes width, and a paragraph would read as an indented code
// block once the quote is gone.
func unquote(line string) string {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 || !strings.HasPrefix(line[indent:], ">") {
		return line
	}
	rest := line[indent+1:]
	column, lead := indent+1, 0
	var spaces strings.Builder
	for ; lead < len(rest) && (rest[lead] == ' ' || rest[lead] == '\t'); lead++ {
		width := 1
		if rest[lead] == '\t' {
			width = 4 - column%4
		}
		spaces.WriteString(strings.Repeat(" ", width))
		column += width
	}
	expanded := spaces.String()
	if expanded != "" {
		expanded = expanded[1:]
	}
	return expanded + rest[lead:]
}

// checkAnchor reports a question whose heading produces no anchor, or the same
// anchor as another heading, one inside a container included. Anchors are
// compared before any suffix is added, because a suffix is assigned in
// document order and moves when a heading before it does. Between two
// questions, the later one is reported.
func (d *Document) checkAnchor(q Heading, questionAt map[int]bool, report func(int, string, ...any)) {
	if q.base == "" {
		report(q.index, "question %q produces no anchor, so it cannot be linked to", q.Text)
		return
	}
	if d.bases == nil {
		d.bases = map[string][]Heading{}
		for _, h := range d.allHeadings {
			d.bases[h.base] = append(d.bases[h.base], h)
		}
	}
	for _, other := range d.bases[q.base] {
		if other.Line == q.Line && other.Text == q.Text {
			continue
		}
		if questionAt[other.Line] && other.Line > q.Line {
			continue
		}
		report(q.index, "question %q produces the same anchor as the heading on line %d, so a link to either is unreliable", q.Text, other.Line)
		return
	}
}

// isQuestion reports whether heading j is a level three heading within a
// question section.
func (d *Document) isQuestion(j int) bool {
	if d.headings[j].Level != 3 {
		return false
	}
	for k := j - 1; k >= 0; k-- {
		if h := d.headings[k]; h.Level <= 2 {
			return h.Level == 2 && (h.Text == OpenQuestions || h.Text == ResolvedQuestions)
		}
	}
	return false
}
