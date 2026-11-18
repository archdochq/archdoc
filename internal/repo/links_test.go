package repo_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"archdoc.dev/internal/repo"
)

// TestProseIsTextOutsideLinksAddressesBracketsAndHeadings: text is prose
// except in a heading, a link's or an image's text, between <a> and </a>, an
// address with a scheme to the next space, and between [ and ] on its line.
func TestProseIsTextOutsideLinksAddressesBracketsAndHeadings(t *testing.T) {
	for body, want := range map[string]string{
		"# Heading\n":                           "",
		"one [two](x.md) three\n":               "one three",
		"a ![b](i.png) c\n":                     "a c",
		"x <a href=\"u\">in</a> y\n":            "x y",
		"see foo://e.com/p and more\n":          "see and more",
		"a\twww.x.com/Wing b\n":                 "a b",
		"a [b] c [[d]] e\n":                     "a c e",
		"a `code` b <!-- c --> d\n":             "a b d",
		"a [ b\n":                               "a [ b",
		"| x | y |\n|---|---|\n| one | two |\n": "one two",
		"> quoted\n":                            "quoted",
	} {
		var last repo.ProseLine
		for line := range repo.ProseOf(body, 1) {
			if strings.TrimSpace(line.Raw) != "" {
				last = line
			}
		}
		if got := strings.Join(strings.Fields(last.Masked), " "); got != want || len(last.Masked) != len(last.Raw) {
			t.Errorf("%q: last prose line = %q, want the words %q at the length of %q", body, last.Masked, want, last.Raw)
		}
	}
}

// TestAWikiLinkIsReadFromTextAsWritten: on one line, outside a link's text,
// an escaped bracket ending none, and none where the document defines the
// label inside it.
func TestAWikiLinkIsReadFromTextAsWritten(t *testing.T) {
	d := page(t, "a [[RFC-0001]] b \\[[x]] [[y\\]] `[[z]]`\n\n"+
		"[[RFC-0007]] and [[w]](u.md) and [a [[v]]](u.md)\n\n[RFC-0007]: p.md\n\n## In [[RFC-0002]]\n\n"+
		"*[[RFC-0003]]* and [[RFC-*0004*]]\n")
	var got []string
	for _, l := range d.WikiLinks() {
		got = append(got, l.Name)
		line := strings.Split(string(d.Source), "\n")[l.Line-1]
		if line[l.Start:l.End] != "[["+l.Name+"]]" {
			t.Errorf("%s at %d:%d-%d is %q in its line", l.Name, l.Line, l.Start, l.End, line[l.Start:l.End])
		}
		if l.Heading != (l.Name == "RFC-0002") {
			t.Errorf("%s: heading %v", l.Name, l.Heading)
		}
	}
	if !slices.Equal(got, []string{"RFC-0001", "RFC-0002", "RFC-0003"}) {
		t.Errorf("wiki links = %q, want RFC-0001, RFC-0002 and RFC-0003", got)
	}
}

// TestExplicitAnchorsAreReadFromRawHTMLAndHTMLBlocks: outside comments, and
// not from a footnote definition that nothing references.
func TestExplicitAnchorsAreReadFromRawHTMLAndHTMLBlocks(t *testing.T) {
	d := page(t, "Text <a id=\"inline\"></a> and <!-- <a id=\"hidden\"> -->.\n\n"+
		"<div>\n<a name='block'></a>\n<!-- <a id=\"commented\"> -->\n</div>\n\n"+
		"See [^used].\n\n[^used]: <a id=\"used\"></a>\n\n[^unused]: <a id=\"unused\"></a>\n")
	want := []string{"page", "inline", "block", "used"}
	if got := d.Anchors(); !slices.Equal(got, want) {
		t.Errorf("anchors = %q, want %q", got, want)
	}
}

// TestALinksTextKeepsItsLinesWithItsCommentsBlank: the lines are joined by a
// line feed, a carriage return before one dropped, and a comment spanning
// them is spaces, the line feed kept.
func TestALinksTextKeepsItsLinesWithItsCommentsBlank(t *testing.T) {
	for body, want := range map[string]string{
		// The marker inside the comment is the comment's.
		"> [a <!-- c\n> d --> e](u.md)\n": "a       \n        e",
		"[a\r\nb](u.md)\r\n":              "a\nb",
	} {
		if links := page(t, body).Links(); len(links) != 1 || links[0].Text != want {
			t.Errorf("%q: links = %+v, want text %q", body, links, want)
		}
	}
}

// TestNestedImagesAreReadInLinearTime: each image's text was copied and
// walked for comments, so each level of nesting paid for every level inside
// it, and 21,000 took four seconds.
func TestNestedImagesAreReadInLinearTime(t *testing.T) {
	d := page(t, strings.Repeat("![", 21000)+"x"+strings.Repeat("](a)", 21000)+"\n")
	started := time.Now()
	d.Links()
	if took := time.Since(started); took > 2*time.Second {
		t.Errorf("reading the links took %v", took)
	}
}
