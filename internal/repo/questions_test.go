package repo_test

import (
	"strings"
	"testing"
)

func TestQuestionsCarryTheirDetailAndAnswer(t *testing.T) {
	d := changelogDoc(t, "", "## Open questions\n\n### Does a withdrawn document's question survive?\n\n"+
		"## Resolved questions\n\n### Should `index.json` carry the questions?\n\n> A quote in the detail.\n\nRaised in review.\n\n"+
		"> Yes, without the bodies,\n>\n>\tas `index.json` does.\n<!-- trailing -->\n")
	q := d.Questions()
	if len(q.Open) != 1 || len(q.Resolved) != 1 {
		t.Fatalf("read %d open and %d resolved, want one of each", len(q.Open), len(q.Resolved))
	}
	open := q.Open[0]
	if open.Heading.Line != 14 || open.Heading.Anchor != "does-a-withdrawn-documents-question-survive" ||
		open.Detail != "" || open.Answer != nil {
		t.Errorf("open question read as %+v", open)
	}
	resolved := q.Resolved[0]
	if resolved.Heading.Line != 18 || resolved.Detail != "\n> A quote in the detail.\n\nRaised in review.\n" {
		t.Errorf("resolved question read as %+v, detail %q", resolved, resolved.Detail)
	}
	if resolved.Answer == nil || resolved.Answer.Line != 24 ||
		resolved.Answer.Text != "Yes, without the bodies,\n\n  as `index.json` does." {
		t.Errorf("answer read as %+v", resolved.Answer)
	}
	if problems := d.QuestionProblems(false); len(problems) > 0 {
		t.Errorf("reported %q, want nothing", problemLines(problems))
	}
}

func TestQuestionSectionFaultsAreReportedWhereTheyStand(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want string
	}{
		{"something before the first question", "## Open questions\nPreamble.\n### Q?\n", "13: the \"Open questions\" section holds something before its first question"},
		{"something and no question", "## Open questions\n- a list\n", "13: the \"Open questions\" section holds something and no question"},
		{"a question with no anchor", "## Open questions\n### ???\n", "13: question \"???\" produces no anchor"},
		{"a question sharing an anchor", "## Abstract\n## Open questions\n### Abstract\n", "14: question \"Abstract\" produces the same anchor as the heading on line 12"},
		{"two questions sharing one, reported at the later", "## Open questions\n### Q?\n### Q\n", "14: question \"Q\" produces the same anchor as the heading on line 13"},
		{"a heading beneath a question", "## Open questions\n### Q?\n#### Detail\n", "14: question \"Q?\" has a heading beneath it"},
		{"a heading beneath a resolved question, reported once", "## Resolved questions\n### Q?\n#### Detail\n", "14: question \"Q?\" has a heading beneath it"},
		{"a heading in a blockquote", "## Open questions\n### Q?\n> ## Not\n> more\n\nText.\n", "14: a blockquote in a question section holds a heading"},
		{"a fence in a blockquote", "## Open questions\n### Q?\n> ```\n> x\n\nText.\n", "14: a blockquote in a question section holds a fenced code block"},
		{"a comment in a blockquote", "## Open questions\n### Q?\n> a <!-- b -->\n\nText.\n", "14: a blockquote in a question section holds an HTML comment"},
		{"an open question ending with a blockquote", "## Open questions\n### Q?\n\n> quoted\n", "15: open question \"Q?\" ends with a blockquote"},
		{"an empty resolved section", "## Resolved questions\n\n<!-- nothing -->\n", "12: the \"Resolved questions\" section is empty"},
		{"a resolved question with no answer", "## Resolved questions\n### Q?\nDetail.\n", "13: resolved question \"Q?\" does not end with a blockquote"},
		{"two blockquotes at the end", "## Resolved questions\n### Q?\n> one\n\n<!-- c -->\n> two\n", "17: the answer to \"Q?\" has another blockquote before it"},
		{"an empty answer", "## Resolved questions\n### Q?\n>\n>  \n", "14: the answer to \"Q?\" is empty"},
		{"resolved not after open", "## Open questions\n## Abstract\n## Resolved questions\n### Q?\n> A.\n", "14: \"Resolved questions\" must be the next level two heading after \"Open questions\""},
		{"a second resolved section", "## Resolved questions\n### Q?\n> A.\n## Resolved questions\n### R?\n> B.\n", "15: the document carries more than one \"Resolved questions\" section"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := changelogDoc(t, "", c.body)
			got := problemLines(d.QuestionProblems(false))
			if len(got) != 1 || !strings.HasPrefix(got[0], c.want) {
				t.Errorf("reported %q, want one finding beginning %q", got, c.want)
			}
		})
	}
}

func TestAnAcceptedDocumentsOpenQuestionsAreLeftToL09(t *testing.T) {
	d := changelogDoc(t, "", "## Open questions\nPreamble.\n")
	if got := d.QuestionProblems(true); len(got) != 0 {
		t.Errorf("with Open questions skipped, reported %q", problemLines(got))
	}
	if got := d.QuestionProblems(false); len(got) != 1 {
		t.Errorf("without, reported %q, want one finding", problemLines(got))
	}
}

func TestADefinitionBehindAQuoteInAQuestionSectionIsALink(t *testing.T) {
	d := changelogDoc(t, "", "## Resolved questions\n### Q?\nDetail.\n> Yes, see [the RFC][r].\n>\n> [r]: 0002-other.md\n\n"+
		"### R?\nDetail.\n> [t]: 0003-other.md\n\n"+
		"## Abstract\n\n> [s]: not-read.md\n")
	var targets []string
	for _, l := range d.Links() {
		targets = append(targets, l.Target)
	}
	if strings.Join(targets, " ") != "0002-other.md 0003-other.md" {
		t.Errorf("links read: %q, want the definitions inside the answers alone", targets)
	}
}

func TestAQuotedLineInAFenceIsCode(t *testing.T) {
	// Left unclosed, and with no newline after it, so the quoted line is the
	// last thing in the question.
	d := changelogDoc(t, "", "## Open questions\n### Q?\n```\n> ## not a quote")
	if got := d.QuestionProblems(false); len(got) != 0 {
		t.Errorf("reported %q, want nothing: the > is inside a fence", problemLines(got))
	}
}
