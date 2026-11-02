package repo_test

import (
	"slices"
	"testing"

	"archdoc.dev/internal/repo"
)

// TestHeadingAnchorsAreGitHubs: a heading is slugged from its rendered text,
// so markup that renders as nothing, or as something else, is not slugged as
// written. Each anchor is GitHub's, from its renderer and github-slugger.
func TestHeadingAnchorsAreGitHubs(t *testing.T) {
	for heading, want := range map[string]string{
		"_Rationale_":                      "rationale",
		"A &amp; B":                        "a--b",
		"Caf&eacute;":                      "café",
		"Foo <span>bar</span>":             "foo-bar",
		"[undef][nolabel] ref":             "undefnolabel-ref",
		"x [a [b](c) d](e)":                "x-a-b-de",
		"[![i](s)](l) y":                   "-y",
		"x [a \\![t](u) b](e)":             "x-a-t-be",
		"x [![a [t](u)](s) b](e)":          "x--be",
		"Link [with [nested] text](a.md)":  "link-with-nested-text",
		"![logo](x.png) Foo":               "-foo",
		"हिन्दी":                           "हिन्दी",
		"Ⅻ roman":                          "ⅻ-roman",
		"x² units":                         "x-units",
		"½ cup":                            "-cup",
		"٣ arabic":                         "٣-arabic",
		"a‿b":                              "a‿b",
		"snake_case_name":                  "snake_case_name",
		"The `_code_` and `&amp;` stay":    "the-_code_-and-amp-stay",
		"An \\_escaped\\_ underscore":      "an-_escaped_-underscore",
		"See <https://example.com> for it": "see-httpsexamplecom-for-it",
		"The [RFC-0007](path) approach":    "the-rfc-0007-approach",
		"A ` x ` b":                        "a-x-b",
		"x `  ` y":                         "x----y",
		"x ` a` y":                         "x--a-y",
		"x `a ` y":                         "x-a--y",
		"See [_Em_](a.md)":                 "see-em",
		"[a\\]b](x.md) c":                  "ab-c",
	} {
		if got := repo.Anchor(heading); got != want {
			t.Errorf("Anchor(%q) = %q, want %q", heading, got, want)
		}
	}
}

// TestOnlyPairedEmphasisLeavesAHeadingsAnchor: an underscore is dropped from
// an anchor only where it pairs as emphasis, which CommonMark decides from what
// flanks each run and which runs can match. Dropping every run that flanked a
// word rejected the-_id-field, GitHub's own anchor. Each is GitHub's.
func TestOnlyPairedEmphasisLeavesAHeadingsAnchor(t *testing.T) {
	for heading, want := range map[string]string{
		"The _id field":    "the-_id-field",
		"_foo":             "_foo",
		"foo_bar_":         "foo_bar_",
		"foo_ bar":         "foo_-bar",
		"_foo_":            "foo",
		"__a__ b":          "a-b",
		"__a_":             "_a",
		"a __b__c":         "a-__b__c",
		"_a_b_":            "a_b",
		"_a__":             "a_",
		"_a_ b_":           "a-b_",
		"_a._.b_":          "ab_",
		"x _.a_":           "x-a",
		"_a._ x":           "a-x",
		"._.a_":            "a",
		"ġ_a_":             "ġ_a_",
		"~a _b~~ c_":       "a-b-c",
		"~a~ _x~ y_":       "a-x-y",
		"~a~b _c~ d_":      "ab-c-d",
		"x«_a_»":           "xa",
		"€_a_":             "_a_",
		"$_a_":             "a",
		"*a _b* c_":        "a-_b-c_",
		"~a _b~ c_":        "a-_b-c_",
		"~~~a _b~~~ c_":    "a-b-c",
		"a~_b_":            "a_b_",
		"_b_~a":            "_b_a",
		"\"_\"__\"":        "___",
		"__a._.":           "__a_",
		"._.a__":           "_a__",
		"x \"___\"___\" y": "x--y",
	} {
		if got := repo.Anchor(heading); got != want {
			t.Errorf("Anchor(%q) = %q, want %q", heading, got, want)
		}
	}
}

// TestAHeadingsAnchorIsRenderedInItsDocument: a reference link renders as a
// link only where the document defines its label, and a comment renders as
// nothing, the space after it staying. Each anchor is GitHub's.
func TestAHeadingsAnchorIsRenderedInItsDocument(t *testing.T) {
	d := page(t, "## See [RFC-0001][r] now\n\n## [undef][nolabel] ref\n\n## <!-- c --> after\n\n"+
		"## ![logo] x\n\n## [Foo][] z\n\n## ![logo][] y\n\n## Closing ##\n\n[r]: x.md\n[logo]: l.png\n[foo]: f.md\n")
	want := []string{"page", "see-rfc-0001-now", "undefnolabel-ref", "-after", "-x", "foo-z", "-y", "closing"}
	if got := d.Anchors(); !slices.Equal(got, want) {
		t.Errorf("Anchors() = %q, want %q", got, want)
	}
	if got := d.Headings()[3].Text; got != "after" {
		t.Errorf("the heading's text = %q, want it read without its comment", got)
	}
}

func TestAnchorsInReadsOnlyWhatGitHubServes(t *testing.T) {
	source := "\xef\xbb\xbf# Overview\n\n<a id='single'></a>\n\n<!-- <a id=\"gone\"></a> -->\n\n" +
		"```\n<a name=\"code\"></a>\n```\n\n<div id=\"x\"><a name=\"inblock\"></a></div>\n\n" +
		"See `<a id=\"span\"></a>` here.\n\n" +
		"<div>\n<!-- <a id=\"hidden\"></a>\n<a id=\"also\"></a> -->\n<a id=\"caf&eacute;\"></a>\n</div>\n\n## Later\n"
	got := repo.AnchorsIn([]byte(source))
	for _, want := range []string{"overview", "single", "inblock", "café", "later"} {
		if !slices.Contains(got, want) {
			t.Errorf("AnchorsIn = %q, missing %q", got, want)
		}
	}
	for _, unwanted := range []string{"gone", "code", "span", "hidden", "also"} {
		if slices.Contains(got, unwanted) {
			t.Errorf("AnchorsIn = %q, holds %q from a comment or code", got, unwanted)
		}
	}
}

func TestADocumentsAnchorsHoldItsExplicitOnes(t *testing.T) {
	d := page(t, "<a id=\"here\"></a>\n\n## Later\n")
	if got := d.Anchors(); !slices.Equal(got, []string{"page", "later", "here"}) {
		t.Errorf("Anchors() = %q, want [page later here]", got)
	}
}

func TestALinkDestinationMayHoldParentheses(t *testing.T) {
	d := page(t, "[ok](foo(bar).md) and [ok2](foo\\)baz.md) and [t](a.md \"A (title)\")\n")
	if got := linkTargets(d); !slices.Equal(got, []string{"foo(bar).md", "foo)baz.md", "a.md"}) {
		t.Errorf("links = %q", got)
	}
	if got := linkTargets(page(t, "[bad](a(b c)\n")); len(got) != 0 {
		t.Errorf("an unbalanced destination read as links %q", got)
	}
}

// TestALinkDestinationIsReadAsGitHubReadsIt: each line is one link or none, as
// cmark-gfm renders it.
func TestALinkDestinationIsReadAsGitHubReadsIt(t *testing.T) {
	for line, want := range map[string][]string{
		"[a]()":                 {""},
		"[a](\tx.md)":           {"x.md"},
		"[a](x.md\t\"t\")":      {"x.md"},
		"[a](x\\a.md)":          {"x\\a.md"},
		"[a](x\\ y.md)":         nil,
		"[a](<x<y.md>)":         nil,
		"[a](<x.md> \"t\")":     {"x.md"},
		"[a](<x<)":              nil,
		"[a](x y.md)":           nil,
		"[a](x.md 't')":         {"x.md"},
		"[a](x.md (t) )":        {"x.md"},
		"[a](x.md (t(u)))":      nil,
		"[a](x.md (t\\(u))":     {"x.md"},
		"[a](x.md \"a)b\")":     {"x.md"},
		"[a](x.md \"t\\\" u\")": {"x.md"},
		"[a](x.md \"t)":         nil,
		"[a](x.md \"t\" u)":     nil,
		"[a](<x.md>\"t\")":      nil,
	} {
		if got := linkTargets(page(t, line+"\n")); !slices.Equal(got, want) {
			t.Errorf("%q: links = %q, want %q", line, got, want)
		}
	}
}

func TestADefinitionDirectlyUnderAHeadingIsRead(t *testing.T) {
	for name, body := range map[string]string{
		"under a heading": "See [the doc][x].\n\n## Sources\n[x]: def.md\n",
		"after a fence":   "See [the doc][x].\n```\ncode\n```\n[x]: def.md\n",
	} {
		if got := linkTargets(page(t, body)); !slices.Equal(got, []string{"def.md"}) {
			t.Errorf("%s: links = %q, want [def.md]", name, got)
		}
	}
}
