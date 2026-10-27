package repo

import (
	"fmt"
	"slices"
	"strings"
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

// questionWalk holds what reading the question sections needs, so the checks
// below are methods rather than functions taking the same five arguments.
type questionWalk struct {
	d        *Document
	raw      []string
	masked   []string
	literal  []bool
	inQuote  []bool
	problems []Problem
}

func (w *questionWalk) report(index int, format string, args ...any) {
	w.problems = append(w.problems, Problem{Line: w.d.BodyLine + index, Message: fmt.Sprintf(format, args...)})
}

// blank is a line holding nothing but whitespace and comments. A fenced code
// block is never blank, as its opening line holds the fence.
func (w *questionWalk) blank(i int) bool {
	return strings.TrimSpace(w.masked[i]) == ""
}

// quoted is a blockquote line: one beginning with > at the start of the line,
// outside an HTML comment and any literal block the quote does not hold.
func (w *questionWalk) quoted(i int) bool {
	return (!w.literal[i] || w.inQuote[i]) && strings.HasPrefix(w.masked[i], ">")
}

// span is a half-open run of body line indexes.
type span struct{ start, end int }

func (d *Document) readQuestions(skipOpen bool) (Questions, []Problem) {
	// The walk fills d.literal, so it runs before the field is read: the order
	// of a call and a field read within one composite literal is unspecified.
	masked := d.maskedLines()
	w := &questionWalk{d: d, raw: d.bodyLines(), masked: masked, literal: d.literal, inQuote: d.quoted}
	var out Questions

	hasOpen := false
	for _, h := range d.headings {
		if h.Level == 2 && h.Text == OpenQuestions {
			hasOpen = true
		}
	}

	resolvedSeen := false
	for i, h := range d.headings {
		if h.Level != 2 || (h.Text != OpenQuestions && h.Text != ResolvedQuestions) {
			continue
		}
		open := h.Text == OpenQuestions
		section := span{h.index + 1, d.sectionEnd(i)}

		// The rows about a Resolved questions section itself apply to the
		// first alone; a later one is reported once, at its heading, and its
		// questions are checked as the first's are.
		first := true
		if !open {
			if resolvedSeen {
				w.report(h.index, "the document carries more than one %q section", ResolvedQuestions)
				first = false
			}
			resolvedSeen = true
		}
		check := !open || !skipOpen
		if first && !open && hasOpen && !w.followsOpen(i) {
			w.report(h.index, "%q must be the next level two heading after %q", ResolvedQuestions, OpenQuestions)
		}

		questions := w.questionsIn(section)
		if first && !open && w.empty(section) {
			w.report(h.index, "the %q section is empty; delete it, or move an answered question into it", ResolvedQuestions)
		} else if first && check {
			before := section.end
			if len(questions) > 0 {
				before = d.headings[questions[0]].index
			}
			for line := section.start; line < before; line++ {
				if w.blank(line) {
					continue
				}
				if len(questions) == 0 {
					w.report(line, "the %q section holds something and no question; each question is a level three heading", h.Text)
				} else {
					w.report(line, "the %q section holds something before its first question", h.Text)
				}
				break
			}
		}
		if check {
			w.checkQuotes(section)
		}

		for n, qi := range questions {
			q := w.read(qi, open, check)
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
					w.report(sub.index, "question %q has a heading beneath it; detail beneath a question is prose", q.Heading.Text)
				}
			}
			if check && !open && !beneath && q.Answer == nil {
				w.report(q.Heading.index, "resolved question %q does not end with a blockquote holding its answer", q.Heading.Text)
			}
			if check {
				w.checkAnchor(qi)
			}
			if open {
				out.Open = append(out.Open, q)
			} else {
				out.Resolved = append(out.Resolved, q)
			}
		}
	}
	slices.SortStableFunc(w.problems, func(a, b Problem) int { return a.Line - b.Line })
	return out, w.problems
}

// followsOpen reports whether the level two or higher heading before heading i
// is an Open questions section.
func (w *questionWalk) followsOpen(i int) bool {
	for j := i - 1; j >= 0; j-- {
		if h := w.d.headings[j]; h.Level <= 2 {
			return h.Level == 2 && h.Text == OpenQuestions
		}
	}
	return false
}

// empty reports whether a section holds nothing but whitespace and comments.
func (w *questionWalk) empty(s span) bool {
	for i := s.start; i < s.end; i++ {
		if !w.blank(i) {
			return false
		}
	}
	return true
}

// questionsIn lists the indexes in d.headings of the level three headings
// within a section.
func (w *questionWalk) questionsIn(s span) []int {
	var found []int
	for i, h := range w.d.headings {
		if h.index >= s.start && h.index < s.end && h.Level == 3 {
			found = append(found, i)
		}
	}
	return found
}

// quotes lists the blockquotes within a span: runs of consecutive quoted
// lines.
func (w *questionWalk) quotes(s span) []span {
	var runs []span
	for i := s.start; i < s.end; i++ {
		if !w.quoted(i) {
			continue
		}
		start := i
		for i < s.end && w.quoted(i) {
			i++
		}
		runs = append(runs, span{start, i})
	}
	return runs
}

// read builds the question at heading qi, reporting the faults in how it ends.
func (w *questionWalk) read(qi int, open, check bool) Question {
	h := w.d.headings[qi]
	end := len(w.raw)
	if qi+1 < len(w.d.headings) {
		end = w.d.headings[qi+1].index
	}
	body := span{h.index + 1, end}
	q := Question{Heading: h}

	runs := w.quotes(body)
	detailEnd := body.end
	if len(runs) > 0 && w.endsWith(runs[len(runs)-1], body) {
		last := runs[len(runs)-1]
		switch {
		case open:
			if check {
				w.report(last.start, "open question %q ends with a blockquote, which would read as its answer once it is resolved", h.Text)
			}
		default:
			detailEnd = last.start
			q.Answer = w.answer(last)
			if check {
				if len(runs) > 1 && w.endsWith(runs[len(runs)-2], span{body.start, last.start}) {
					w.report(last.start, "the answer to %q has another blockquote before it; join them with a line holding only >", h.Text)
				}
				if w.quoteEmpty(last) {
					w.report(last.start, "the answer to %q is empty", h.Text)
				}
			}
		}
	}
	q.Detail = strings.Join(w.raw[body.start:detailEnd], "\n")
	return q
}

// endsWith reports whether a blockquote is followed by nothing but blank lines
// and comments to the end of the span.
func (w *questionWalk) endsWith(run, within span) bool {
	for i := run.end; i < within.end; i++ {
		if !w.blank(i) {
			return false
		}
	}
	return true
}

// quoteEmpty reports whether a blockquote holds nothing but its markers,
// whitespace and comments.
func (w *questionWalk) quoteEmpty(run span) bool {
	for i := run.start; i < run.end; i++ {
		if strings.TrimSpace(strings.TrimPrefix(w.masked[i], ">")) != "" {
			return false
		}
	}
	return true
}

// answer is a blockquote's text without its markers.
func (w *questionWalk) answer(run span) *Answer {
	lines := make([]string, 0, run.end-run.start)
	for i := run.start; i < run.end; i++ {
		lines = append(lines, unquote(strings.TrimSuffix(w.raw[i], "\r")))
	}
	return &Answer{Line: w.d.BodyLine + run.start, Text: strings.Join(lines, "\n")}
}

// unquote removes a blockquote line's > and one column of the whitespace after
// it, as CommonMark does. The whitespace before the text is expanded to spaces
// at the columns it occupied inside the quote first, because a tab moved two
// columns left changes width, and a paragraph would read as an indented code
// block once the quote is gone.
func unquote(line string) string {
	rest := strings.TrimPrefix(line, ">")
	column, lead := 1, 0
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

// checkQuotes reports a heading, a fence or a comment inside a blockquote. An
// answer is read from its lines as written, which holds none of them as the
// rendered quote does, so they are refused rather than read some other way.
//
// A fence is reported at the line opening it, and the lines it holds are code,
// so a # or a comment marker among them is neither a heading nor a comment.
func (w *questionWalk) checkQuotes(s span) {
	for _, run := range w.quotes(s) {
		var fenceChar byte
		fenceLen := 0
		for i := run.start; i < run.end; i++ {
			content := unquote(strings.TrimSuffix(w.raw[i], "\r"))
			switch {
			case fenceLen > 0:
				if closesFence(content, fenceChar, fenceLen) {
					fenceLen = 0
				}
			case IsHeading(content):
				w.report(i, "a blockquote in a question section holds a heading")
				continue
			case fencePattern.MatchString(content):
				m := fencePattern.FindStringSubmatch(content)
				fenceChar, fenceLen = m[1][0], len(m[1])
				w.report(i, "a blockquote in a question section holds a fenced code block")
				continue
			}
			if w.masked[i] != w.raw[i] {
				w.report(i, "a blockquote in a question section holds an HTML comment")
			}
		}
	}
}

// checkAnchor reports a question whose heading produces no anchor, or the same
// anchor as another heading. Anchors are compared before any suffix is added,
// because a suffix is assigned in document order and moves when a heading
// before it does.
func (w *questionWalk) checkAnchor(qi int) {
	q := w.d.headings[qi]
	base := Anchor(q.Text)
	if base == "" {
		w.report(q.index, "question %q produces no anchor, so it cannot be linked to", q.Text)
		return
	}
	for j, other := range w.d.headings {
		if j == qi || Anchor(other.Text) != base {
			continue
		}
		// Between two questions, the later one is reported.
		if w.isQuestion(j) && j > qi {
			continue
		}
		w.report(q.index, "question %q produces the same anchor as the heading on line %d, so a link to either is unreliable", q.Text, other.Line)
		return
	}
}

// isQuestion reports whether heading j is a level three heading within a
// question section.
func (w *questionWalk) isQuestion(j int) bool {
	if w.d.headings[j].Level != 3 {
		return false
	}
	for k := j - 1; k >= 0; k-- {
		if h := w.d.headings[k]; h.Level <= 2 {
			return h.Level == 2 && (h.Text == OpenQuestions || h.Text == ResolvedQuestions)
		}
	}
	return false
}

// questionQuotes marks the blockquote lines of every question section, by body
// line index.
func (d *Document) questionQuotes() map[int]bool {
	// The walk fills d.literal, so it runs before the field is read: the order
	// of a call and a field read within one composite literal is unspecified.
	masked := d.maskedLines()
	w := &questionWalk{d: d, raw: d.bodyLines(), masked: masked, literal: d.literal, inQuote: d.quoted}
	marked := map[int]bool{}
	for i, h := range d.headings {
		if h.Level != 2 || (h.Text != OpenQuestions && h.Text != ResolvedQuestions) {
			continue
		}
		for line, end := h.index+1, d.sectionEnd(i); line < end; line++ {
			marked[line] = w.quoted(line)
		}
	}
	return marked
}
