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
		"See [RFC-0001][r] now":            "see-rfc-0001-now",
		"Link [with [nested] text](a.md)":  "link-with-nested-text",
		"![logo](x.png) Foo":               "-foo",
		"हिन्दी":                           "हिन्दी",
		"Ⅻ roman":                          "ⅻ-roman",
		"a‿b":                              "a‿b",
		"snake_case_name":                  "snake_case_name",
		"The `_code_` and `&amp;` stay":    "the-_code_-and-amp-stay",
		"An \\_escaped\\_ underscore":      "an-_escaped_-underscore",
		"See <https://example.com> for it": "see-httpsexamplecom-for-it",
		"The [RFC-0007](path) approach":    "the-rfc-0007-approach",
		"A ` x ` b":                        "a-x-b",
		"See [_Em_](a.md)":                 "see-em",
		"[a\\]b](x.md) c":                  "ab-c",
	} {
		if got := repo.Anchor(heading); got != want {
			t.Errorf("Anchor(%q) = %q, want %q", heading, got, want)
		}
	}
}

func TestAnchorsInReadsOnlyWhatGitHubServes(t *testing.T) {
	source := "\xef\xbb\xbf# Overview\n\n<a id='single'></a>\n\n<!-- <a id=\"gone\"></a> -->\n\n" +
		"```\n<a name=\"code\"></a>\n```\n\n<div id=\"x\"><a name=\"inblock\"></a></div>\n"
	got := repo.AnchorsIn([]byte(source))
	for _, want := range []string{"overview", "single", "inblock"} {
		if !slices.Contains(got, want) {
			t.Errorf("AnchorsIn = %q, missing %q", got, want)
		}
	}
	for _, unwanted := range []string{"gone", "code"} {
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
