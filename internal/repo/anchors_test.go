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
		"far along [<!-- b --> c](y.md)":   "far-along--c",
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
		"a\t_b_":           "ab",
		"a\f_b_":           "ab",
		"a\v_b_":           "a_b_",
		"a\u3000_b_":       "ab",
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
		"a*\"_b\"* c_":     "ab-c",
	} {
		if got := repo.Anchor(heading); got != want {
			t.Errorf("Anchor(%q) = %q, want %q", heading, got, want)
		}
	}
}

// TestABareURLInAHeadingIsLinkedAsGitHubLinksIt: GitHub links a bare
// http://, https://, ftp:// or www. address in a heading, and what it links is
// text, so an underscore in it is not emphasis. Each anchor is GitHub's.
func TestABareURLInAHeadingIsLinkedAsGitHubLinksIt(t *testing.T) {
	for heading, want := range map[string]string{
		"see https://a.com/_x_ y":        "see-httpsacom_x_-y",
		"HTTPS://a.com/_x_ y":            "httpsacom_x_-y",
		"ftp://a.com/_x_ y":              "ftpacom_x_-y",
		"see www.a.com/_x_ y":            "see-wwwacom_x_-y",
		"_www.a.com/_x_ y":               "wwwacom_x-y",
		"see www.a/_x_ y":                "see-wwwa_x_-y",
		"see https://é.com/_x_ y":        "see-httpsécom_x_-y",
		"see https://a.com/_x_)_ y":      "see-httpsacom_x__-y",
		"see https://a.com/(_x_)_ y":     "see-httpsacom_x__-y",
		"see https://a.com/_x&amp;_ y":   "see-httpsacom_x_-y",
		"see https://a.com/_x&a;_ y":     "see-httpsacom_xa_-y",
		"see https://a.com/_x<b>_y_</b>": "see-httpsacom_xy",
		"[x] https://a.com/_x_ y":        "x-httpsacom_x_-y",
		"[i [b](c) https://a.com/_x_ y":  "i-b-httpsacom_x_-y",
		"[a] ] https://a.com/_x_ y":      "a--httpsacom_x_-y",
		"[x https://a.com/_x_ y":         "x-httpsacomx-y",
		"![i [b](c) https://a.com/_x_ y": "i-b-httpsacomx-y",
		"xhttps://a.com/_x_ y":           "xhttpsacomx-y",
		"awww.a.com/_x_ y":               "awwwacomx-y",
		"see www.a_b.c/_x_ y":            "see-wwwa_bcx-y",
		"see https://a_b.c_d/_x_ y":      "see-httpsa_bc_dx-y",
		"see https://_x_ y":              "see-httpsx-y",
		"a_b@c.com_ x":                   "a_bccom_-x",
		"see\twww.a.com/_x_ y":           "seewwwacom_x_-y",
		"see https://a.com/_x_\ty":       "see-httpsacom_x_y",
		"*www.a.com/_x_ y":               "wwwacom_x_-y",
		"~www.a.com/_x_ y":               "wwwacom_x_-y",
		"(www.a.com/_x_ y":               "wwwacom_x_-y",
		"[see https://a.com/_x_ y](u)":   "see-httpsacomx-y",
		"![i https://a.com/_x_ y":        "i-httpsacomx-y",
		"x http:/":                       "x-http",
		"http://a.com/_x_ y":             "httpacom_x_-y",
		"_a https://a.com/x_?":           "a-httpsacomx",
		"_a https://a.com/x_!":           "a-httpsacomx",
		"_a https://a.com/x_.":           "a-httpsacomx",
		"_a https://a.com/x_,":           "a-httpsacomx",
		"_a https://a.com/x_:":           "a-httpsacomx",
		"_a https://a.com/x_*":           "a-httpsacomx",
		"_a https://a.com/x_~":           "a-httpsacomx",
		"_a https://a.com/x_'":           "a-httpsacomx",
		"_a https://a.com/x_\"":          "a-httpsacomx",
		"_a https://a.com/x_":            "a-httpsacomx",
		"_a https://a.com/x_;":           "a-httpsacomx",
		"_a https://a.com/x_&;":          "_a-httpsacomx_",
		"see https://a.com/x(_a; b_":     "see-httpsacomx_a-b_",
		"_a https://a.com/(x_)":          "_a-httpsacomx_",
		"see https://a-b_c.d/_x_ y":      "see-httpsa-b_cdx-y",
		"see https://a.b_c/_x_ y":        "see-httpsab_cx-y",
		"www.a.com/_x_ y":                "wwwacom_x_-y",
		"Xhttps://a.com/_x_ y":           "xhttpsacomx-y",
		"see https://.a.com/_x_ y":       "see-httpsacomx-y",
		"see https://a.com/x\t_y_":       "see-httpsacomxy",
		"_a https://a.com/x_)":           "a-httpsacomx",
		"_a https://a.com/x_&AMP;":       "a-httpsacomx",
		"see https://a_b.c.d/_x_ y":      "see-httpsa_bcd_x_-y",
		"_a www.b_.c":                    "a-wwwbc",
		"see https://é_x.c/_x_ y":        "see-httpsé_xc_x_-y",
		"see https://\uFFFD.com/_x_ y":   "see-httpscom_x_-y",
		"see https://\u3000_x_ y":        "see-httpsx-y",
		"see https://a\f_x_ y":           "see-httpsa_x_-y",
		"see https://a\v_x_ y":           "see-httpsa_x_-y",
		"\\http://a.b/_x_":               "httpab_x_",
	} {
		if got := repo.Anchor(heading); got != want {
			t.Errorf("Anchor(%q) = %q, want %q", heading, got, want)
		}
	}
}

// TestALinkInAHeadingIsReadAsALinkIsRead: a heading's link closes where a
// link in prose does, its destination and title read as L17 reads them and a
// bracket inside a code span in its text not one of its brackets. Closing the
// parentheses at their balancing ) linked what GitHub renders as text, and
// rejected GitHub's anchor. Each anchor is GitHub's.
func TestALinkInAHeadingIsReadAsALinkIsRead(t *testing.T) {
	for heading, want := range map[string]string{
		"See [a](x y.md)":            "see-ax-ymd",
		"See [b](x.md \"title)":      "see-bxmd-title",
		"See [c](x.md title)":        "see-cxmd-title",
		"See [d](x.md \"t\" junk)":   "see-dxmd-t-junk",
		"See [g](<x)y>)":             "see-g",
		"See [h](<x>y)":              "see-hy",
		"See [l](x.md (t(u)))":       "see-lxmd-tu",
		"See [o](x.md \"a\" \"b\")":  "see-oxmd-a-b",
		"See [t](<x.md>\"t\")":       "see-txmdt",
		"See ![v](x y.png) w":        "see-vx-ypng-w",
		"See *[cc](x y.md)*":         "see-ccx-ymd",
		"See [ff](http://a.b/_c_ d)": "see-ffhttpab_c_-d",
		"See [a](x.md \"t\") b":      "see-a-b",
		"See [e](x.md (y)) f":        "see-e-f",
		"See [j `]` k](x.md)":        "see-j--k",
		"x [a `b](c)` d](e)":         "x-a-bc-d",
		"x [a ``]`` b](e)":           "x-a--b",
		"x [a `]`` b](e) c]":         "x-a--be-c",
		"x [a `[b](c)` d](e)":        "x-a-bc-d",
		"x [a `b`](e)":               "x-a-b",
		"x [a `b`](e) `c`":           "x-a-b-c",
	} {
		if got := repo.Anchor(heading); got != want {
			t.Errorf("Anchor(%q) = %q, want %q", heading, got, want)
		}
	}
}

// TestAHeadingsBracketsAreMatchedOnceAsEachWasOnItsOwn: one walk of a
// heading's text matches its brackets, and reads a backtick inside a tag as
// opening a code span where the renderer reads the tag first. A [ the walk
// passed over, or did not close before the end of the link text holding it,
// is matched on its own. A bracket once closed, an image's as a link's, no
// longer keeps a bare address from being linked. Each anchor is the one read
// when each bracket was matched on its own; the third reads a link inside a
// link, as holdsLink reads the tag's backtick as the walk does.
func TestAHeadingsBracketsAreMatchedOnceAsEachWasOnItsOwn(t *testing.T) {
	for heading, want := range map[string]string{
		"<i title=\"`\">[a](x.md) b `":                            "a-b-",
		"<i title=\"`\">[a <i title=\"`\">[b `](x.md)":            "a-b-",
		"<i title=\"`\">[a <i title=\"`\">[b ``](c.md)`](d.md)``": "a-b-",
		"![a] https://b.com/_c_ d":                                "a-httpsbcom_c_-d",
		"[x [a](b) ] [ https://c.com/_d_ e":                       "x-a---httpsccomd-e",
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
		"## ![logo] x\n\n## [Foo][] z\n\n## ![logo][] y\n\n## Closing ##\n\n## [x][a`]`] y\n\n"+
		"[r]: x.md\n[logo]: l.png\n[foo]: f.md\n[a`]: u.md\n")
	want := []string{"page", "see-rfc-0001-now", "undefnolabel-ref", "-after", "-x", "foo-z", "-y", "closing", "x-y"}
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
