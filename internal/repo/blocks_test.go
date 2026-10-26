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
