package repo_test

import (
	"slices"
	"strings"
	"testing"

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
		"[[RFC-0007]] and [[w]](u.md) and [a [[v]]](u.md)\n\n[RFC-0007]: p.md\n\n## In [[RFC-0002]]\n")
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
	if !slices.Equal(got, []string{"RFC-0001", "RFC-0002"}) {
		t.Errorf("wiki links = %q, want RFC-0001 and RFC-0002", got)
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
