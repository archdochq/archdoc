package repo_test

import (
	"slices"
	"testing"

	"archdoc.dev/internal/repo"
)

func page(t *testing.T, body string) *repo.Document {
	t.Helper()
	d, ok := repo.Parse("spec/page.md", []byte("---\ntitle: Page\nincludes: []\n---\n\n# Page\n\n"+body))
	if !ok {
		t.Fatal("spec/page.md is not a document path")
	}
	return d
}

func headingTexts(d *repo.Document) []string {
	var texts []string
	for _, h := range d.Headings()[1:] {
		texts = append(texts, h.Text)
	}
	return texts
}

func linkTargets(d *repo.Document) []string {
	var targets []string
	for _, l := range d.Links() {
		targets = append(targets, l.Target)
	}
	return targets
}

// TestProseAfterABlockStaysVisible: each body below is prose CommonMark reads
// as closed, which opened a fence or a comment and hid every heading and link
// after it.
func TestProseAfterABlockStaysVisible(t *testing.T) {
	for name, body := range map[string]string{
		"backticks in a backtick fence's info string": "```go``` is how Go code is shown inline.\n",
		"an escaped comment marker":                   "Write \\<!-- to open a comment.\n",
		"a code span wrapped across a line":           "A span `foo <!--\nbar` done.\n",
		"a comment marker mid-line, never closed":     "Unclosed <!-- inline\n",
		"a fence inside a list item":                  "- item\n\n    ```\n    <!--\n    ```\n",
		"a fence inside a blockquote":                 "> ```\n> <!--\n> ```\n",
		"an indented code block":                      "Text.\n\n    <!-- indented\n",
		"an HTML block":                               "<div>\n<!-- inside\n</div>\n",
		"a fence after an empty item's blank line":    "-\n\n  ```\n# Heading\n ```\n",
	} {
		t.Run(name, func(t *testing.T) {
			d := page(t, body+"\n## Later\n\n[broken](nope.md)\n")
			if got := headingTexts(d); !slices.Equal(got, []string{"Later"}) {
				t.Errorf("headings = %q, want [Later]", got)
			}
			if got := linkTargets(d); !slices.Equal(got, []string{"nope.md"}) {
				t.Errorf("links = %q, want [nope.md]", got)
			}
			if d.Unclosed() {
				t.Error("the body reads as unclosed")
			}
		})
	}
}

// TestACommentOnTheBodysFirstLineHidesWhatItHolds: a comment opened on the
// first line of the body closes on a later line as it does anywhere else.
func TestACommentOnTheBodysFirstLineHidesWhatItHolds(t *testing.T) {
	d, _ := repo.Parse("spec/page.md", []byte("---\ntitle: Page\nincludes: []\n---\nText <!-- [[RFC-0001]]\n-->\n"))
	if got := d.WikiLinks(); len(got) != 0 {
		t.Errorf("wiki links = %v, want none", got)
	}
}

// TestLiteralBlocksHoldNoProse: GitHub shows these as code or raw HTML, so
// nothing in them is a heading, a link or a wiki link.
func TestLiteralBlocksHoldNoProse(t *testing.T) {
	for name, body := range map[string]string{
		"an indented code block":                        "Text.\n\n    ## Code\n    See [[RFC-0001]] and [x](hidden.md)\n",
		"an HTML block":                                 "<div>\n## Not heading\n[[RFC-0001]] [a](hidden.md)\n</div>\n",
		"a raw tag alone on a line":                     "<span class=\"x\">\n## Not heading\n[a](hidden.md)\n",
		"a pre block":                                   "<pre>\n## Not heading\n\n[a](hidden.md)\n</pre>\n",
		"a fence inside a list item":                    "- item\n\n  ```\n  ## Code\n  [a](hidden.md)\n  ```\n",
		"a code span wrapped across lines":              "A `span [x](hidden.md)\ncontinues` here.\n",
		"code after a numbered line inside a paragraph": "Text\n2. continued\n\n    code [x](hidden.md)\n",
		"code after a thematic break":                   "* * *\n\n    code [x](hidden.md)\n",
		"code as a list item's content":                 "-      code [x](hidden.md)\n",
		"an empty item inside a paragraph":              "Text\n-\n\n    code [x](hidden.md)\n",
		"a processing instruction":                      "<?php\n\n## Not heading [x](hidden.md)\n?>\n",
		"a declaration":                                 "<!DOCTYPE\n\n## Not heading [x](hidden.md)\n>\n",
		"a CDATA section":                               "<![CDATA[\n\n## Not heading [x](hidden.md)\n]]>\n",
	} {
		t.Run(name, func(t *testing.T) {
			d := page(t, body)
			if got := headingTexts(d); len(got) != 0 {
				t.Errorf("headings = %q, want none", got)
			}
			if got := linkTargets(d); len(got) != 0 {
				t.Errorf("links = %q, want none", got)
			}
			if got := d.WikiLinks(); len(got) != 0 {
				t.Errorf("wiki links = %v, want none", got)
			}
		})
	}
}

// TestProseThatOnlyLooksLiteralIsRead: indentation inside a list item is the
// item's, a paragraph's continuation cannot start code, and a blank line ends
// an HTML block.
func TestProseThatOnlyLooksLiteralIsRead(t *testing.T) {
	for name, body := range map[string]string{
		"a list item's second paragraph":      "- item\n\n    continued [x](kept.md)\n",
		"a numbered item's second line":       "10. item\n    continued [x](kept.md)\n",
		"a paragraph's indented line":         "Text\n    continued [x](kept.md)\n",
		"after an HTML block":                 "<details>\n<summary>More</summary>\n\nInside [x](kept.md)\n\n</details>\n",
		"a comment closed later in a line":    "Text <!-- note\nmore --> after [x](kept.md)\n",
		"an inline tag":                       "Some <span>text</span> and [x](kept.md)\n",
		"an autolink alone on a line":         "<https://example.com>\n[x](kept.md)\n",
		"an escaped comment around a link":    "Write \\<!-- [x](kept.md) --> as text.\n",
		"after a lazy continuation":           "- item\nlazy\n\n    continued [x](kept.md)\n",
		"after a fence its quote ends":        "> ```\n> code\nafter [x](kept.md)\n",
		"a lone tag inside a paragraph":       "Text\n<span>\n[x](kept.md)\n",
		"backticks in separate list items":    "- a `x\n- b [x](kept.md) `z\n",
		"a list starting at 1 in a paragraph": "Text\n1. item\n\n    continued [x](kept.md)\n",
		"a list starting at 2":                "2. item\n\n    continued [x](kept.md)\n",
		"after a processing instruction":      "<?php echo 1; ?>\n\n[x](kept.md)\n",
		"after a declaration":                 "<!DOCTYPE html>\n\n[x](kept.md)\n",
		"after a CDATA section":               "<![CDATA[ x ]]>\n\n[x](kept.md)\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := linkTargets(page(t, body)); !slices.Equal(got, []string{"kept.md"}) {
				t.Errorf("links = %q, want [kept.md]", got)
			}
		})
	}
}

func TestACommentOpeningALineStillRunsToTheEnd(t *testing.T) {
	d := page(t, "<!-- open\n\n## Hidden\n")
	if got := headingTexts(d); len(got) != 0 || !d.Unclosed() {
		t.Errorf("headings = %q, unclosed = %v; want none, true", got, d.Unclosed())
	}
}

func TestAnUnclosedPreBlockIsUnclosed(t *testing.T) {
	d := page(t, "<pre>\n\n## Hidden\n")
	if got := headingTexts(d); len(got) != 0 || !d.Unclosed() {
		t.Errorf("headings = %q, unclosed = %v; want none, true", got, d.Unclosed())
	}
}

func TestAHeadingOfOnlyAClosingSequenceIsEmpty(t *testing.T) {
	for _, line := range []string{"## #", "### ###"} {
		d := page(t, line+"\n")
		if got := headingTexts(d); !slices.Equal(got, []string{""}) {
			t.Errorf("%q reads as headings %q, want one empty heading", line, got)
		}
	}
}

// TestLinksAreReadWhereCommonMarkReadsThem holds each case review found read
// differently from CommonMark, by the links it shows.
func TestLinksAreReadWhereCommonMarkReadsThem(t *testing.T) {
	for name, c := range map[string]struct {
		body string
		want []string
	}{
		// A tab runs to the next multiple of four columns, not four on.
		"a tab after a list marker":           {"- \t[x](a.md)\n", []string{"a.md"}},
		"a tab after a quote marker":          {"> \t[x](a.md)\n", []string{"a.md"}},
		"a tab in a list item's continuation": {"- a\n\n  \tfoo [x](a.md)\n", []string{"a.md"}},
		"a tab four columns into a list item": {"- a\n\n  \t  code [x](a.md)\n", nil},
		// An inline comment's paragraph ends where a block interrupts it.
		"a heading ends a commented paragraph":                 {"Text <!-- x\n## Heading [y](a.md) -->\n\n[z](b.md)\n", []string{"a.md", "b.md"}},
		"a comment block ends a commented paragraph":           {"Text <!-- x\n<!-- y -->\n[z](b.md)\n", []string{"b.md"}},
		"a table ends a commented paragraph":                   {"Text <!-- x\n| [y](a.md) |\n| - |\n-->\n", []string{"a.md"}},
		"a row with no delimiter row continues one":            {"Text <!-- x\n| [y](a.md) |\n-->\n", nil},
		"a comment on a header row closes on it or not at all": {"Text <!-- [x](b.md)\n| - |\n-->\n", []string{"b.md"}},
		"a comment in a table row closes on it or not at all":  {"| x |\n| - |\n| <!-- [y](a.md) |\n| --> |\n", []string{"a.md"}},
		"an indented comment continues a paragraph":            {"p\n    <!-- c -->\n    code [x](a.md)\n", []string{"a.md"}},
		"a comment closing on its table row":                   {"| x |\n| - |\n| <!-- [y](a.md) --> |\n", nil},
		"a commented paragraph continues lazily in a quote":    {"> Text <!-- x\ny --> [z](b.md)\n", []string{"b.md"}},
		"a commented paragraph continues indented":             {"Text <!-- a\nb -->\n    more [x](a.md)\n", []string{"a.md"}},
		"a comment four spaces in, then an indented line":      {"Text\n    <!-- a\nb -->\n    more [x](a.md)\n", []string{"a.md"}},
		// A comment block ends with the container holding it.
		"a comment block in a quote":                {"> <!-- note\n\n## Later\n\n[x](a.md)\n", []string{"a.md"}},
		"a comment block in a list item":            {"- <!-- TODO\n\nAfter [x](a.md)\n", []string{"a.md"}},
		"a comment block in a list item, continued": {"- <!-- TODO\n\n  [x](a.md) -->\n\nAfter [y](b.md)\n", []string{"b.md"}},
		// A code span crossing a line holds a comment marker.
		"a comment after a comment block closes is part of the block":   {"<!-- a\nb --> <!-- c\n[x](a.md) -->\n", []string{"a.md"}},
		"a comment after one closing on its opening line is part of it": {"<!-- a --> <!-- c\n[x](a.md) -->\n", []string{"a.md"}},
		"a tab before an HTML block interrupting a nested quote":        {"> > a\n> \t<div>\n> > [x](a.md)\n", nil},
		"a tab before an HTML block interrupting a quoted item":         {"- > a\n  \t<div>\n  [x](a.md)\n", nil},
		"a tab before an HTML block in a list item":                     {"- a\n\n  \t<div>\n  [x](a.md)\n", nil},
		"a code span across lines holding <!--":                         {"A `<!--\n` and [x](a.md) -->\n", []string{"a.md"}},
		"a code span across lines in a quote":                           {"> A `span\n> [x](a.md)` end\n", nil},
		"a quote starting after a paragraph":                            {"A `span\n> [x](a.md)` end\n", []string{"a.md"}},
		// HTML blocks.
		"a closing pre is a block":           {"</pre>\nSee [x](a.md)\n", nil},
		"any of the four closes a raw block": {"<pre>\n[x](a.md)\n</script>\n\nAfter [y](b.md)\n", []string{"b.md"}},
		"<?> opens and closes at once":       {"<?>\n\nAfter [x](a.md)\n", []string{"a.md"}},
		// Where a comment opens and where its paragraph ends.
		"a comment block's depth outlasting it":                       {"> <!-- a -->\n\nText <!-- x\n[a](a.md) -->\n", nil},
		"a paragraph after a comment block's closing line":            {"<!-- a\nb --> <!-- c\nd -->\n    [x](a.md)\n", []string{"a.md"}},
		"a marker after a closing one, never closed":                  {"Text <!-- a\nb --> <!-- c\n\n[x](a.md)\n", []string{"a.md"}},
		"a marker after a closing one, closed later":                  {"Text <!-- a\nb --> <!-- c\n[x](a.md) -->\n", nil},
		"code after a comment block":                                  {"<!-- a\nb -->\n    [x](a.md)\n", nil},
		"a comment block three spaces in":                             {"   <!-- a\n\n[x](a.md) -->\n", nil},
		"a comment four spaces into a paragraph":                      {"Text\n    <!-- a\n\n[x](a.md) -->\n", []string{"a.md"}},
		"a commented paragraph after a comment block":                 {"<!-- a -->\nText <!-- b\nc -->\n    [x](a.md)\n", []string{"a.md"}},
		"code after a heading in a quote":                             {"> # H\n>     [x](a.md)\n", nil},
		"code after a one-line comment block with text after it":      {"<!-- a --> b\n    [x](a.md)\n", nil},
		"an underline after a one-line comment block is text":         {"<!-- a --> b\n===\n    [x](a.md)\n", []string{"a.md"}},
		"code after a one-line comment block":                         {"<!-- x -->\n    [x](a.md)\n", nil},
		"a closing marker before the opening one":                     {"a --> b <!-- c\n\n[x](a.md)\n", []string{"a.md"}},
		"a thematic break ends a commented paragraph, CRLF":           {"Text <!-- x\r\n***\r\n[y](a.md) -->\r\n", []string{"a.md"}},
		"a commented paragraph continues in a quote":                  {"> Text <!-- x\n> [y](a.md) -->\n", nil},
		"a comment four spaces in continues a paragraph":              {"Text <!-- x [a](a.md)\n    <!-- -->\n", nil},
		"a thematic break three spaces in ends a commented paragraph": {"Text <!-- x\n   ***\n[y](a.md) -->\n", []string{"a.md"}},
		"a numbered line continues a commented paragraph":             {"Text <!-- x\n2. y\n[z](a.md) -->\n", nil},
		"a second item ends a commented paragraph":                    {"1. a <!-- x\n2. b [z](a.md) -->\n", []string{"a.md"}},
		"a list numbered 2 ends a quoted commented paragraph":         {"> a <!-- x\n2. b [z](a.md) -->\n", []string{"a.md"}},
		"a tab before a comment ending a commented item":              {"- Text <!-- x [a](a.md)\n  \t<!-- -->\n", []string{"a.md"}},
		"code in a quote ending a commented item":                     {"- a <!-- x\n>     [z](a.md)\n", nil},
		"a line numbered 2 continues a commented item":                {"- a <!-- x\n  2. b [z](a.md) -->\n", nil},
		"a code span opening after the marker":                        {"Text <!-- [a](a.md) `y\nz` -->\n", nil},
		"a code span closing before the marker":                       {"`a` <!-- [x](a.md)\n`b` -->\n", nil},
		"a backtick closing only past the paragraph":                  {"A `x <!-- [a](a.md) -->\nmore\n\nb` c\n", nil},
		// Containers, lazy continuation and the leaf blocks they hold.
		"code after a quote's blank line":                      {"> a\n>\n    [x](a.md)\n", nil},
		"code after a blank line ends a quote":                 {"> a\n\n    [x](a.md)\n", nil},
		"a fence ends a quote":                                 {"> a\n~~~\n[x](a.md)\n~~~\n", nil},
		"code after a paragraph that ended a list":             {"- a\n\nb\n\n    [x](a.md)\n", nil},
		"a line three spaces in after code":                    {"    code\n   [x](a.md)\n", []string{"a.md"}},
		"a tab ending code in a list item":                     {"- a\n\n      code\n  \t[x](a.md)\n", []string{"a.md"}},
		"a fence in a quote":                                   {"> ```\n> [x](a.md)\n> ```\n", nil},
		"code in a quote interrupting a paragraph":             {"a\n>     [x](a.md)\n", nil},
		"code in a list item interrupting a paragraph":         {"a\n-      [x](a.md)\n", nil},
		"a tab after a list marker in a quote":                 {"> -\tfoo\n>\n>       [x](a.md)\n", nil},
		"a line three spaces in":                               {"   [x](a.md)\n", []string{"a.md"}},
		"a list item continued in a quote":                     {"> - a\n>\n>      [x](a.md)\n", []string{"a.md"}},
		"a blank line ends a quote holding a list item":        {"> - a\n\n>     x [x](a.md)\n", nil},
		"a tab continuing a nested list item":                  {"- a\n  1. b\n\n  \t    [x](a.md)\n", []string{"a.md"}},
		"code after a fence that interrupted a paragraph":      {"a\n```\nb\n```\n    [x](a.md)\n", nil},
		"code after a heading":                                 {"## H\n    [x](a.md)\n", nil},
		"code after a thematic break":                          {"***\n    [x](a.md)\n", nil},
		"code after a thematic break, CRLF":                    {"***\r\n    [x](a.md)\r\n", nil},
		"a list item ends a quote":                             {"> a\n-      [x](a.md)\n", nil},
		"an HTML block ends a quote":                           {"> a\n<div>\n[x](a.md)\n", nil},
		"a quote ends a list item":                             {"- a\n> ~~~\n> [x](a.md)\n> ~~~\n", nil},
		"a comment block ends a commented paragraph, unclosed": {"Text <!-- x [a](a.md)\n<!-- y -->\n", []string{"a.md"}},
		"an indented comment block ends a commented paragraph": {"Text <!-- x [a](a.md)\n  <!-- y -->\n", []string{"a.md"}},
		"a list starting at 01 in a paragraph":                 {"Text\n01. item\n\n    [x](a.md)\n", []string{"a.md"}},
		"an empty item continued":                              {"-\n     [x](a.md)\n", []string{"a.md"}},
		"code in an empty item":                                {"-\n      [x](a.md)\n", nil},
		"five spaces after a list marker":                      {"-     [x](a.md)\n", nil},
		"four spaces after a list marker":                      {"-    a\n\n      [x](a.md)\n", []string{"a.md"}},
		"code continuing an item opened by code":               {"-      code\n\n      [x](a.md)\n", nil},
		"a backtick in a tilde fence's info string":            {"~~~ a`b\n[x](a.md)\n~~~\n", nil},
		"a tag four spaces into a paragraph":                   {"Text\n    <div>\n[x](a.md)\n", []string{"a.md"}},
		"an HTML block three spaces in":                        {"   <div>\n[x](a.md)\n", nil},
		"a > inside a processing instruction":                  {"<?\na > b\n[x](a.md)\n?>\n", nil},
		"a > inside a CDATA section":                           {"<![CDATA[\na > b\n[x](a.md)\n]]>\n", nil},
		"an HTML block interrupting a paragraph":               {"Text\n<div>\n[x](a.md)\n", nil},
		"a raw block closed in capitals":                       {"<pre>\n[x](a.md)\n</PRE>\n\n[y](b.md)\n", []string{"b.md"}},
		"a tab continuing a list item":                         {"- a\n\n\t[x](a.md)\n", []string{"a.md"}},
		"a tilde line in a backtick fence":                     {"```\n~~~\n[x](a.md)\n```\n", nil},
		// Code spans and comment markers within a paragraph.
		"a code span into an HTML block":              {"A `x [a](a.md)\n<div>\nb`\n</div>\n", []string{"a.md"}},
		"a code span across a blank line":             {"A `x\n\n[a](a.md) b`\n", []string{"a.md"}},
		"a code span from a heading":                  {"## A `x\n[a](a.md) b`\n", []string{"a.md"}},
		"a code span across a quote's blank line":     {"> A `x\n>\n> [a](a.md) b`\n", []string{"a.md"}},
		"a code span after the marker":                {"Text <!-- [a](a.md) `x` -->\n", nil},
		"a marker in a code span":                     {"A `<!--` [a](a.md) -->\n", []string{"a.md"}},
		"a code span closing before one across lines": {"A `x` b `<!-- c\nd` [a](a.md) -->\n", []string{"a.md"}},
		"an escaped backslash before the marker":      {"Write \\\\<!-- [a](a.md) -->\n", nil},
		// A list item interrupts a paragraph only where CommonMark lets it: one
		// numbered other than 1, an empty one or one four spaces in continues
		// the paragraph, unless the line leaves the paragraph's container.
		"a code span across a line numbered 2":            {"A `x\n2) [a](a.md) y`\n", nil},
		"a code span across a line numbered 2 in an item": {"- a `x\n  2) [a](a.md) b`\n", nil},
		"a code span across an empty item":                {"A `x\n*\n[a](a.md) y`\n", nil},
		"a code span across an item four spaces in":       {"A `x\n    - [a](a.md) y`\n", nil},
		"link text across an empty item":                  {"[a\n+\nb](a.md)\n", []string{"a.md"}},
		"link text across a line numbered 2":              {"[a\n2) b](a.md)\n", []string{"a.md"}},
		"a code span ending at a second item":             {"1) a `x\n2) [a](a.md) b`\n", []string{"a.md"}},
		"a code span ending at a list numbered 2":         {"> a `x\n2) [a](a.md) b`\n", []string{"a.md"}},
		"a code span ending at an empty list numbered 2":  {"> a `x\n2)\n[a](a.md) y`\n", []string{"a.md"}},
		// A list item can begin with at most one blank line: one before any
		// content ends it.
		"a blank line ends an empty item":                      {"- \n\n   ~~~\n\n[x](a.md)\n", nil},
		"a blank line continues an item given content later":   {"-\n  a\n\n  ~~~\n[x](a.md)\n", []string{"a.md"}},
		"a blank line continues an item given content at once": {"- a\n\n  ~~~\n[x](a.md)\n", []string{"a.md"}},
		"a blank line continues an item holding an empty one":  {"- -\n\n  ~~~\n[x](a.md)\n", []string{"a.md"}},
		// A setext underline ends the paragraph it underlines, unless it is a
		// lazy continuation line.
		"a setext underline ends a commented paragraph":            {"a <!-- b\n===\n[x](a.md) -->\n", []string{"a.md"}},
		"a dash underline ends a commented paragraph":              {"a <!-- b\n-\n[x](a.md) -->\n", []string{"a.md"}},
		"a lazy underline continues a commented paragraph":         {"> a <!-- b\n==\n[x](a.md) -->\n", nil},
		"code after a setext heading":                              {"a\n===\n    [x](a.md)\n", nil},
		"an underline under no paragraph is text":                  {"## H\n===\n    [x](a.md)\n", []string{"a.md"}},
		"a lazy underline continued by an indented line":           {"> a\n==\n    [x](a.md)\n", []string{"a.md"}},
		"an underline followed by a comment continues a paragraph": {"a\n== <!-- c -->\n    [x](a.md)\n", []string{"a.md"}},
		// A definition counts only where a paragraph could start.
		"a definition after code under a heading":      {"## H\n    code\n\n[a]: b.md\n", []string{"b.md"}},
		"a definition after code and a line of spaces": {"## H\n    code\n    \n[a]: b.md\n", []string{"b.md"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := linkTargets(page(t, c.body)); !slices.Equal(got, c.want) {
				t.Errorf("links = %q, want %q in:\n%s", got, c.want, c.body)
			}
		})
	}
}

// TestACommentBlockHoldsTheRestOfTheLineItClosesOn: each reading is cmark-gfm's.
// A comment opening a block runs to the end of the line holding its "-->", and
// the line after it is read as written.
func TestACommentBlockHoldsTheRestOfTheLineItClosesOn(t *testing.T) {
	for name, c := range map[string]struct {
		body string
		want []string
	}{
		"a link after the marker":                              {"<!-- c --> [y](n.md)\n", nil},
		"a heading after the marker":                           {"<!-- c --> # x [y](n.md)\n", nil},
		"in a list item":                                       {"- <!-- c --> [y](n.md)\n", nil},
		"interrupting a paragraph":                             {"p\n<!-- c --> [y](n.md)\n", nil},
		"in a blockquote":                                      {"> <!-- c --> [y](n.md)\n", nil},
		"in a CRLF file":                                       {"<!-- c --> [y](n.md)\r\n", nil},
		"spanning lines":                                       {"<!-- a\nb --> [y](n.md)\n", nil},
		"spanning lines in a blockquote":                       {"> <!-- a\n> b --> [y](n.md)\n", nil},
		"spanning lines in a list item":                        {"- <!-- a\n  b --> [y](n.md)\n", nil},
		"spanning lines in a CRLF file":                        {"<!-- a\r\nb --> [y](n.md)\r\n", nil},
		"three spaces in":                                      {"   <!-- c --> [y](n.md)\n", nil},
		"a second comment after the marker":                    {"<!-- a --> <!-- b --> [y](n.md)\n", nil},
		"a code span after the marker":                         {"<!-- a --> `b` [y](n.md)\n", nil},
		"the next line":                                        {"<!-- c --> [y](n.md)\nnext [z](m.md)\n", []string{"m.md"}},
		"the next line in a blockquote":                        {"> <!-- c --> [y](n.md)\n> next [z](m.md)\n", []string{"m.md"}},
		"the next line in a list item":                         {"- <!-- c --> [y](n.md)\n  next [z](m.md)\n", []string{"m.md"}},
		"the next line in a CRLF file":                         {"<!-- c --> [y](n.md)\r\nnext [z](m.md)\r\n", []string{"m.md"}},
		"the next line after spanning lines":                   {"<!-- a\nb --> [y](n.md)\nnext [z](m.md)\n", []string{"m.md"}},
		"the next line after a bare comment":                   {"<!-- c -->\nnext [z](m.md)\n", []string{"m.md"}},
		"four spaces in, as code":                              {"    <!-- c --> [y](n.md)\n", nil},
		"a tab in, as code":                                    {"\t<!-- c --> [y](n.md)\n", nil},
		"after text, in a paragraph":                           {"text <!-- c --> [y](n.md)\n", []string{"n.md"}},
		"after text, spanning lines":                           {"text <!-- a\nb --> [y](n.md)\n", []string{"n.md"}},
		"four spaces into a paragraph":                         {"p\n    <!-- c --> [y](n.md)\n", []string{"n.md"}},
		"a lone carriage return ends the line":                 {"<!-- c -->\r[y](n.md)\n", []string{"n.md"}},
		"a lone carriage return after text":                    {"<!-- c --> x\r[y](n.md)\n", []string{"n.md"}},
		"a link before a lone carriage return":                 {"<!-- c --> [a](b.md)\r[y](n.md)\n", []string{"n.md"}},
		"a link before a lone carriage return, spanning lines": {"<!--\n--> [a](b.md)\r[y](n.md)\n", []string{"n.md"}},
		"a link before a lone carriage return, quoted":         {"> <!-- c --> [a](b.md)\r> [y](n.md)\n", []string{"n.md"}},
		"a link before an anchor and a lone carriage return":   {"<!-- c --> [a](b.md) <a id=\"x\"></a>\r[y](n.md)\n", []string{"n.md"}},
		"a link before a lone carriage return ending the line": {"<!-- c --> [a](b.md)\r \n", nil},
		"a definition after a bare comment in an item":         {"- <!-- c -->\n  [r]: n.md\n", []string{"n.md"}},
		"a definition after text in an item":                   {"- <!-- c --> x\n  [r]: n.md\n", []string{"n.md"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := linkTargets(page(t, c.body)); !slices.Equal(got, c.want) {
				t.Errorf("links = %q, want %q in:\n%s", got, c.want, c.body)
			}
		})
	}
}

func TestACommentBlocksLastLineHoldsNoWikiLink(t *testing.T) {
	for _, body := range []string{"<!-- c --> [[RFC-0001]]\n", "<!-- a\nb --> [[RFC-0001]]\n"} {
		if got := page(t, body).WikiLinks(); len(got) != 0 {
			t.Errorf("wiki links = %v in:\n%s", got, body)
		}
	}
}

// TestTextOnACommentBlocksLastLineIsNotAHeading: with no space after the
// marker, the masked line is indented only three columns.
func TestTextOnACommentBlocksLastLineIsNotAHeading(t *testing.T) {
	for _, body := range []string{"<!--\n--># x\n", "<!--\n-->## Next\n", "<!-- c --> # x\n"} {
		if got := headingTexts(page(t, body)); len(got) != 0 {
			t.Errorf("headings = %q in:\n%s", got, body)
		}
	}
}

// TestTextOnACommentBlocksLastLineIsContent: it renders, as raw HTML, so the
// line is literal rather than blanked, and an anchor on it is still a target.
func TestTextOnACommentBlocksLastLineIsContent(t *testing.T) {
	for _, body := range []string{"<!-- c --> text\n", "<!-- a\nb --> text\n", "<!-- c --> text\r \n"} {
		if page(t, "## A\n\n"+body).Sections("A")[0].Empty() {
			t.Errorf("the section is empty, holding:\n%s", body)
		}
	}
	for _, body := range []string{"<!-- c -->\n", "<!-- c -->   \r\n", "<!-- a\nb -->\n"} {
		if !page(t, "## A\n\n"+body).Sections("A")[0].Empty() {
			t.Errorf("the section is not empty, holding only:\n%s", body)
		}
	}
	for _, body := range []string{"<!-- c --> <a id=\"x\"></a>\n", "<!-- a\nb --> <a id=\"x\"></a>\n", "<!-- c --> <a id=\"x\"></a>\rtext\n", "> <!-- a\n> b --> <a name=\"x\">\r> text\n"} {
		if got := page(t, body).Anchors(); !slices.Contains(got, "x") {
			t.Errorf("anchors = %q, want x in:\n%s", got, body)
		}
	}
}

// TestLinkTextAcrossACRLFCodeSpanReadsAsLF: masking a code span keeps the
// carriage return ending each line it crosses, so the text of a link holding
// one is the same in a CRLF file as in an LF one.
func TestLinkTextAcrossACRLFCodeSpanReadsAsLF(t *testing.T) {
	lf := page(t, "[a `x\ny` b](a.md)\n").Links()
	crlf := page(t, "[a `x\r\ny` b](a.md)\r\n").Links()
	if len(lf) != 1 || len(crlf) != 1 || crlf[0].Text != lf[0].Text {
		t.Errorf("links = %+v with CRLF, want the text of %+v", crlf, lf)
	}
}

func TestAnUnclosedFenceIsUnclosed(t *testing.T) {
	d := page(t, "```\n\n## Hidden\n")
	if got := headingTexts(d); len(got) != 0 || !d.Unclosed() {
		t.Errorf("headings = %q, unclosed = %v; want none, true", got, d.Unclosed())
	}
}

// TestAnHTMLBlockAtTheEndIsClosed: a blank line closes a <div> block, so the
// end of a file with no final newline closes it too.
func TestAnHTMLBlockAtTheEndIsClosed(t *testing.T) {
	if d := page(t, "Text.\n\n<div>"); d.Unclosed() {
		t.Error("the body reads as unclosed")
	}
}
