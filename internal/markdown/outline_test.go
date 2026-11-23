package markdown

import (
	"strconv"
	"strings"
	"testing"
)

// spans returns the bytes each node of type t spans in body, in order.
func spans(body string, t Type) []string {
	var out []string
	Walk(Parse([]byte(body)), func(n *Node) bool {
		if n.Type == t {
			out = append(out, body[n.Start:n.End])
		}
		return true
	})
	return out
}

func nodes(body string, t Type) []*Node {
	var out []*Node
	Walk(Parse([]byte(body)), func(n *Node) bool {
		if n.Type == t {
			out = append(out, n)
		}
		return true
	})
	return out
}

func wantSpans(t *testing.T, body string, typ Type, want ...string) {
	t.Helper()
	if got := spans(body, typ); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%q: %s spans %q, want %q", body, typ, got, want)
	}
}

func TestACodeSpanIncludesItsBackticks(t *testing.T) {
	wantSpans(t, "a `` ` `` b `c`\n", CodeSpan, "`` ` ``", "`c`")
	wantSpans(t, "a `b\nc` d\n", CodeSpan, "`b\nc`")
}

// TestACodeSpanInACellWithAnEscapedPipeIsFound: the parser gives no position
// to any code span in a table cell where a backtick comes before an escaped
// pipe, whether or not the span holds it.
func TestACodeSpanInACellWithAnEscapedPipeIsFound(t *testing.T) {
	body := "| a |\n|---|\n| b `x\\|y` c `z` |\n| `p` q \\| r |\n"
	wantSpans(t, body, CodeSpan, "`x\\|y`", "`z`", "`p`")
	if got := nodes(body, CodeSpan)[0].Text; got != "x|y" {
		t.Errorf("code span text = %q, want %q", got, "x|y")
	}
}

func TestAnAutolinkSpansItsText(t *testing.T) {
	body := "a <http://b.io/c> d www.e.io (http://f.io) g@h.io\n"
	wantSpans(t, body, Link, "<http://b.io/c>", "www.e.io", "http://f.io", "g@h.io")
	var dests, written []string
	for _, n := range nodes(body, Link) {
		dests = append(dests, n.Destination)
		written = append(written, body[n.DestinationSpan.Start:n.DestinationSpan.End])
	}
	if want := "http://b.io/c|http://www.e.io|http://f.io|mailto:g@h.io"; strings.Join(dests, "|") != want {
		t.Errorf("destinations = %q, want %q", dests, want)
	}
	if want := "http://b.io/c|www.e.io|http://f.io|g@h.io"; strings.Join(written, "|") != want {
		t.Errorf("destinations as written = %q, want %q", written, want)
	}
}

func TestHeadingsEndWithTheirUnderlineOrClosingSequence(t *testing.T) {
	wantSpans(t, "# A #  \n\nB\nC\n===\n\n##\n\n### D ###\n", Heading, "# A #", "B\nC\n===", "##", "### D ###")
	hs := nodes("# A\n\nB\n---\n", Heading)
	if hs[0].Setext || !hs[1].Setext || hs[1].Level != 2 {
		t.Errorf("setext = %v %v, level %d; want an ATX heading, then a setext one of level 2", hs[0].Setext, hs[1].Setext, hs[1].Level)
	}
	if got := hs[1].Lines; len(got) != 1 || got[0].Start != 5 || got[0].End != 6 {
		t.Errorf("setext heading lines = %v, want the line of its text alone", got)
	}
}

func TestAFenceEndsWithItsClosingFence(t *testing.T) {
	body := "```go\na\n```\n\n~~~~\nb\n~~~~~ \n\n> ```\n> c\n> ```\n\n```\nopen\n"
	wantSpans(t, body, CodeBlock, "```go\na\n```", "~~~~\nb\n~~~~~", "```\n> c\n> ```", "```\nopen")
	var closed []bool
	for _, n := range nodes(body, CodeBlock) {
		closed = append(closed, n.Closed)
	}
	if got, want := stringify(closed), "[true true true false]"; got != want {
		t.Errorf("closed = %s, want %s", got, want)
	}
}

func stringify(v []bool) string {
	var s []string
	for _, b := range v {
		if b {
			s = append(s, "true")
		} else {
			s = append(s, "false")
		}
	}
	return "[" + strings.Join(s, " ") + "]"
}

func TestAnHTMLBlockKnowsWhetherItsEndMarkerClosedIt(t *testing.T) {
	var closed []bool
	for _, n := range nodes("<!-- a -->\n\n<!--\nb\n-->\n\n<div>\n\n<!-- c\n", HTMLBlock) {
		closed = append(closed, n.Closed)
	}
	if got, want := stringify(closed), "[true true false false]"; got != want {
		t.Errorf("closed = %s, want %s", got, want)
	}
}

// TestAnHTMLBlockKnowsWhetherOnlyItsEndMarkerClosesIt: types 1 to 5 run to
// their end marker, and types 6 and 7 end at a blank line.
func TestAnHTMLBlockKnowsWhetherOnlyItsEndMarkerClosesIt(t *testing.T) {
	for body, want := range map[string]bool{
		"<script>\n":       true,
		"<PRE\n":           true,
		"<style x>\n":      true,
		"<textarea>\n":     true,
		"<!-- a\n":         true,
		"<?php\n":          true,
		"<!DOCTYPE html>":  true,
		"<![CDATA[ a\n":    true,
		"<div>\n":          false,
		"</p>\n":           false,
		"<custom>\n":       false,
		"<a href=\"x\">\n": false,
	} {
		if n := nodes(body, HTMLBlock)[0]; n.ClosedByMarker != want {
			t.Errorf("%q: closed by marker %v, want %v", body, n.ClosedByMarker, want)
		}
	}
}

func TestBreaksAndDefinitionsEndAtTheirLastByte(t *testing.T) {
	wantSpans(t, "a\n\n***  \n\nb\n", ThematicBreak, "***")
	wantSpans(t, "[a]: b.md \"t\"  \n[c]:\n  <d e.md>\n", LinkReferenceDefinition, "[a]: b.md \"t\"", "[c]:\n  <d e.md>")
	wantSpans(t, "a  \nb\\\nc\nd\n", LineBreak, "  ", "\\", "")
	if bs := nodes("a  \nb\\\nc\nd\n", LineBreak); !bs[0].Hard || !bs[1].Hard || bs[2].Hard {
		t.Errorf("hard = %v %v %v, want two hard breaks and a soft one", bs[0].Hard, bs[1].Hard, bs[2].Hard)
	}
	wantSpans(t, "\\\nfoo\n", Text, "", "foo")
}

func TestEachFormOfALinkSpansWhatIsWritten(t *testing.T) {
	body := "[a](b.md \"t\") [c]() [d](<e f.md>) [g][h] [h][] [h] ![i](j.png)\n\n[h]: k.md\n"
	wantSpans(t, body, Link, "[a](b.md \"t\")", "[c]()", "[d](<e f.md>)", "[g][h]", "[h][]", "[h]")
	wantSpans(t, body, Image, "![i](j.png)")
	var written []string
	for _, n := range nodes(body, Link) {
		if n.Reference {
			written = append(written, "ref:"+n.Label+"="+n.Destination)
			continue
		}
		written = append(written, body[n.DestinationSpan.Start:n.DestinationSpan.End])
	}
	if want := "b.md||e f.md|ref:h=k.md|ref:h=k.md|ref:h=k.md"; strings.Join(written, "|") != want {
		t.Errorf("destinations = %q, want %q", written, want)
	}
	if c := nodes(body, Link)[1]; c.DestinationSpan.Start != 18 || c.DestinationSpan.Start != c.DestinationSpan.End {
		t.Errorf("empty destination spans %v, want empty after the (", c.DestinationSpan)
	}
}

func TestFootnotesKeepTheirLabelsAndPlaces(t *testing.T) {
	body := "a[^n]\n\n[^n]: first\n\n[^u]: nothing references it\n"
	refs, defs := nodes(body, FootnoteReference), nodes(body, FootnoteDefinition)
	if len(refs) != 1 || refs[0].Label != "n" || body[refs[0].Start:refs[0].End] != "[^n]" {
		t.Fatalf("references = %v, want one labelled n", refs)
	}
	if len(defs) != 2 || defs[0].Label != "n" || defs[1].Label != "u" {
		t.Fatalf("definitions = %d, want both, where they stand", len(defs))
	}
	if doc := Parse([]byte(body)); doc.Children[1].Type != FootnoteDefinition {
		t.Errorf("the document's second block is a %s, want the definition where it stands", doc.Children[1].Type)
	}
}

func TestALoneCarriageReturnEndsALine(t *testing.T) {
	wantSpans(t, "# A\r## B\r\n", Heading, "# A", "## B")
	wantSpans(t, "- a\r- b\n", ListItem, "- a", "- b")
}

func TestEmphasisSpansTheDelimitersItUses(t *testing.T) {
	wantSpans(t, "**a* x ~~c~~ ~d~\n", Emphasis, "*a*")
	wantSpans(t, "**a* x ~~c~~ ~d~\n", Strikethrough, "~~c~~", "~d~")
	wantSpans(t, "***a** b*\n", Strong, "**a**")
}

func TestATableCellStartsAfterItsPipe(t *testing.T) {
	body := "| a | b |\n|---|---|\n|  | x \\| y |\nc\n"
	wantSpans(t, body, TableRow, "|  | x \\| y |", "c")
	var starts []int
	for _, n := range nodes(body, TableCell) {
		starts = append(starts, n.Start)
	}
	if got, want := sprint(starts), "1 5 21 24 34 35"; got != want {
		t.Errorf("cell starts = %s, want %s", got, want)
	}
}

func sprint(v []int) string {
	var s []string
	for _, i := range v {
		s = append(s, strconv.Itoa(i))
	}
	return strings.Join(s, " ")
}

// TestAFenceClosesOnlyInsideItsContainer: a blank line before the closing
// fence leaves it closing, and a fence opening a block after the container
// holding the first has ended does not close it.
func TestAFenceClosesOnlyInsideItsContainer(t *testing.T) {
	for _, c := range []struct {
		body, span string
		closed     bool
	}{
		{"```\na\n\n```", "```\na\n\n```", true},
		{"```\n\n```", "```\n\n```", true},
		{"```\na\n  \n```", "```\na\n  \n```", true},
		{"> ```\n> a\n```\nb", "```\n> a", false},
		{"- ```\n  a\n```", "```\n  a", false},
		{">> ```\n>> a\n> ```", "```\n>> a", false},
	} {
		n := nodes(c.body, CodeBlock)[0]
		if got := c.body[n.Start:n.End]; got != c.span || n.Closed != c.closed {
			t.Errorf("%q: the first fence spans %q, closed %v; want %q, closed %v", c.body, got, n.Closed, c.span, c.closed)
		}
	}
}

func TestAnHTMLBlockOfTheFirstTypeClosesAtItsEndTag(t *testing.T) {
	for body, closed := range map[string]bool{
		"<pre>\na": false, "<script>\na": false, "<style>\na": false,
		"<pre>\na\n</PRE>": true, "<textarea>\n</textarea>": true, "<pre>a</pre>": true,
	} {
		if n := nodes(body, HTMLBlock)[0]; n.Closed != closed {
			t.Errorf("%q: closed %v, want %v", body, n.Closed, closed)
		}
	}
}

func TestACodeSpanClosingOnALaterLineEndsAtItsBackticks(t *testing.T) {
	wantSpans(t, "`a\n   ` b\n", CodeSpan, "`a\n   `")
	wantSpans(t, "> `a\n> ` b\n", CodeSpan, "`a\n> `")
	wantSpans(t, "- `a\n  ` b\n", CodeSpan, "`a\n  `")
	wantSpans(t, "`\n  a`\n", CodeSpan, "`\n  a`")
}

func TestALinkWithAnEmptyTitleEndsAfterIt(t *testing.T) {
	wantSpans(t, "[a](/u ()) b)\n", Link, "[a](/u ())")
	wantSpans(t, "[a](/u \"\") b)\n", Link, "[a](/u \"\")")
	if n := nodes("[a](/u ()) b)\n", Link)[0]; !n.HasTitle || n.TitleSpan.Start != n.TitleSpan.End {
		t.Errorf("title %v at %v, want an empty title", n.HasTitle, n.TitleSpan)
	}
}

func TestALinkByReferenceHasNoTitleSpan(t *testing.T) {
	n := nodes("[a]\n\n[a]: /u \"t\"\n", Link)[0]
	if n.Title != "t" || n.HasTitle || n.HasDestination {
		t.Errorf("title %q, has title %v, has destination %v; want the definition's title, with no span of its own", n.Title, n.HasTitle, n.HasDestination)
	}
}

func TestALinksTextSpansTheBytesBetweenItsBrackets(t *testing.T) {
	body := "[a *b*](c) [](d) ![e](f)\n"
	var got []string
	for _, typ := range []Type{Link, Image} {
		for _, n := range nodes(body, typ) {
			got = append(got, body[n.TextSpan.Start:n.TextSpan.End])
		}
	}
	if want := "a *b*||e"; strings.Join(got, "|") != want {
		t.Errorf("texts = %q, want %q", got, want)
	}
}

func TestAFootnoteDefinitionsMarkerEndsAfterItsLabel(t *testing.T) {
	wantSpans(t, "[^a:b]:\n\nx[^a:b]\n", FootnoteDefinition, "[^a:b]:")
}

func TestASpaceWrittenAsAnEntityBeforeALineBreakIsText(t *testing.T) {
	for body, want := range map[string]string{"a&#32;\nb\n": "a ", "a&#9;  \nb\n": "a\t", "a  \nb\n": "a"} {
		if got := nodes(body, Text)[0].Text; got != want {
			t.Errorf("%q: first text %q, want %q", body, got, want)
		}
	}
}

func TestAFootnoteReferenceAndDefinitionSpanTheirLabels(t *testing.T) {
	body := "a[^n1]\n\n[^n1]: b\n"
	r, d := nodes(body, FootnoteReference)[0], nodes(body, FootnoteDefinition)[0]
	if got := body[r.LabelSpan.Start:r.LabelSpan.End] + "|" + body[d.LabelSpan.Start:d.LabelSpan.End]; got != "n1|n1" {
		t.Errorf("labels span %q, want %q", got, "n1|n1")
	}
	wantSpans(t, "a !0^1]\n\n[^1]: x\n", FootnoteReference, "0^1]")
}

func TestADestinationsEscapedAmpersandBeginsNoEntity(t *testing.T) {
	if got := resolve(`\&quot;`); got != "&quot;" {
		t.Errorf("resolve = %q, want %q", got, "&quot;")
	}
}

func TestALabelIsAsWritten(t *testing.T) {
	body := "[a\\*b] [a\\*b][] [x][a&amp;b]\n\n> [a\\*\n> b][]\n\n[a\\*b]: u\n[a&amp;b]: v\n[a\\* b]: w\n[^a\\]b]: y\n\nx[^a\\]b]\n"
	var labels []string
	for _, typ := range []Type{Link, LinkReferenceDefinition, FootnoteDefinition, FootnoteReference} {
		for _, n := range nodes(body, typ) {
			labels = append(labels, n.Label)
		}
	}
	if got, want := strings.Join(labels, "|"), "a\\*b|a\\*b|a&amp;b|a\\*\nb|a\\*b|a&amp;b|a\\* b|a\\]b|a\\]b"; got != want {
		t.Errorf("labels %q, want %q", got, want)
	}
	wantSpans(t, body, FootnoteReference, "[^a\\]b]")
	wantSpans(t, body, FootnoteDefinition, "[^a\\]b]: y")
}

func TestAnHTMLBlocksEndMarkerMayOverlapItsStartMarker(t *testing.T) {
	for _, body := range []string{"<!-->\nfoo\n", "<!--->\n", "   <!-->\n", "<?>\nx\n"} {
		if n := nodes(body, HTMLBlock)[0]; !n.Closed {
			t.Errorf("%q: not closed, want closed", body)
		}
	}
}

func TestAnIndentedCodeBlockStartsAtItsContent(t *testing.T) {
	wantSpans(t, "        foo\n    bar\n", CodeBlock, "    foo\n    bar")
	wantSpans(t, "\t\tfoo\n", CodeBlock, "\tfoo")
}

func TestABlockEndsBeforeBlankLinesOfItsContainer(t *testing.T) {
	wantSpans(t, "> <!-- a\n>\n", HTMLBlock, "<!-- a")
	wantSpans(t, "> ```\n> a\n>\n", CodeBlock, "```\n> a")
	wantSpans(t, "> ```\n>\n", CodeBlock, "```")
	wantSpans(t, "> ```\n> a\n>\n", Blockquote, "> ```\n> a")
	wantSpans(t, "| a |\n|---|\n", Table, "| a |\n|---|")
}

func TestADefinitionEndsAfterAnEmptyTitle(t *testing.T) {
	wantSpans(t, "[r]: /u \"\"\n[s]:\n/v\n''\n> [t]: /w\n> ()\n\n[u]: /x\n\n\"\"\n", LinkReferenceDefinition,
		"[r]: /u \"\"", "[s]:\n/v\n''", "[t]: /w\n> ()", "[u]: /x")
}
