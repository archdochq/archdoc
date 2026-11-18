package markdown

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// shape is a body built from one pattern repeated, which a parser can read in
// time growing faster than its length.
type shape struct {
	name string
	body string
	// slower names why goldmark reads the shape in more than linear time,
	// skipping the test of goldmark's time until it does not.
	slower string
}

// shapes builds each shape at size k, where 4 is the size the time limit is
// for and each is about k times as long as at 1, or k squared for those whose
// pattern grows as it repeats.
func shapes(k int) []shape {
	n := 10000 * k
	r := strings.Repeat
	var nestedLists, backticks, refDefs strings.Builder
	for i := 0; i < 250*k; i++ {
		nestedLists.WriteString(r("  ", i) + "* a\n")
	}
	for i := 1; i < 500*k; i++ {
		backticks.WriteString("e" + r("`", i))
	}
	for i := 0; i < 1250*k; i++ {
		fmt.Fprintf(&refDefs, "[l%d]: u%d\n", i, i)
	}
	refDefs.WriteString(r("[l1]", 1250*k))
	unclosed := r("a <!--", n)
	const bracketsOnLines = "a bracketed text closed on each line of a paragraph"
	return []shape{
		// From cmark-gfm's pathological tests.
		{name: "nested strong emphasis", body: r("*a **a ", n/4) + "b" + r(" a** a*", n/4)},
		{name: "emphasis closers with no openers", body: r("a_ ", n)},
		{name: "emphasis openers with no closers", body: r("_a ", n)},
		{name: "link closers with no openers", body: r("a]", n)},
		{name: "link openers with no closers", body: r("[a", n)},
		{name: "mismatched openers and closers", body: r("*a_ ", n)},
		{name: "openers and closers multiple of 3", body: "a**b" + r("c* ", n)},
		{name: "link openers and emphasis closers", body: r("[ a_", n)},
		{name: "pattern [ (]( repeated", body: r("[ (](", n)},
		{name: "nested brackets", body: r("[", n) + "a" + r("]", n)},
		{name: "nested blockquotes", body: r("> ", 500*k) + "a"},
		{name: "deeply nested lists", body: nestedLists.String(), slower: "list items nested a level deeper on each line"},
		{name: "backticks", body: backticks.String(), slower: "backtick runs of every length"},
		{name: "unclosed links A", body: r("[a](<b", n)},
		{name: "unclosed links B", body: r("[a](b", n)},
		{name: "unclosed comments", body: "</" + r("<!--", n)},
		// Added beside them.
		{name: "comments in a heading", body: "# " + r("<!-- ", n) + "\n\ntext\n"},
		{name: "comments in a code span", body: "`" + r("<!--", n) + "\n"},
		{name: "pipes in a table's header", body: r("a|", n) + "\n-|\n"},
		{name: "rows of a table", body: "|x|y|\n|-|-|\n" + r("|a|b|\n", n)},
		{name: "addresses without < and >", body: r("www.a.io ", n)},
		{name: "fences each closed", body: r("```\n```\n", n)},
		{name: "fences each holding a line", body: r("~~~\na\n~~~\n", n)},
		{name: "link reference definitions", body: refDefs.String(), slower: "link reference definitions"},
		{name: "brackets on each line", body: r("[a]\n", n), slower: bracketsOnLines},
		{name: "collapsed references on each line", body: r("[a][]\n", n) + "\n[a]: u\n", slower: bracketsOnLines},
		{name: "collapsed references on each quoted line", body: r("> [a][]\n", n) + "\n[a]: u\n", slower: bracketsOnLines},
		// From the timing tests of the readers the outline replaces.
		{name: "links left open on a line", body: r("[a](b ", n/4) + "\n\n" + r("[", n/2) + r("[a](b)", n/2) + "\n", slower: "brackets left open before a run of links"},
		{name: "a paragraph of unclosed comments", body: r("x <!-- y\n", n/2)},
		{name: "comment markers each after a backtick", body: r("` <!--", n) + "\n`\n-->\n"},
		{name: "comment markers never closed", body: unclosed + "\n"},
		{name: "comment markers each closed", body: r("a <!-- b --> ", n) + "\n"},
		{name: "comment markers in a table row's cell", body: "| h |\n|---|\n| " + unclosed + " |\n"},
		{name: "comment markers in a heading", body: "### " + unclosed + "\n\ntext\n"},
		{name: "comment markers in link text", body: "[" + unclosed + "](x.md)\n"},
		{name: "comment markers inside a code span closing on the next line", body: "`` " + unclosed + "\n``\n-->\n"},
		{name: "comment markers after the close of a comment in a paragraph", body: "x <!-- y\n--> " + unclosed + "\n"},
		{name: "comment markers after a comment block", body: "<!-- x --> " + unclosed + "\n"},
		{name: "comment markers after a long indent", body: "para\n" + r(" ", 2*n) + unclosed + "\n"},
		{name: "code spans and one comment marker", body: r("`a` ", 75000*k) + "<!-- x\n"},
		{name: "a heading of brackets never closed", body: "## " + r("[a ", n) + "\n\ntext\n\n[a]: x.md\n"},
		{name: "a heading of nested links", body: "## " + r("[a ", n) + r("](x.md)", n) + "\n\ntext\n\n[a]: x.md\n"},
		{name: "a heading of nested brackets, none a link", body: "## " + r("[a ", n) + r("]", n) + "\n\ntext\n\n[a]: x.md\n"},
		{name: "a heading of brackets after a tag holding a backtick", body: "## <i title=\"`\">" + r("[a ", n) + "`\n\ntext\n\n[a]: x.md\n"},
		{name: "a heading of links with unclosed destinations", body: "## " + r("[a](b(", n) + "\n"},
		{name: "a line of many containers", body: r(">", n) + " a\n" + r(">", n) + " b\n"},
	}
}

// TestEachShapeIsReadInLinearTime reads each shape under a time limit,
// skipping a shape goldmark reads in more than linear time, naming it, until
// it does not; and asserts of every shape that building the outline from
// goldmark's tree takes time growing no faster than the body's length.
func TestEachShapeIsReadInLinearTime(t *testing.T) {
	// The outline's growth is measured from size 4 to 16, where both sizes
	// are past a machine's caches, but from 1 to 4 for a shape goldmark reads
	// in more than linear time, which it would take minutes to read at 16.
	small, large := shapes(1), shapes(16)
	for i, s := range shapes(4) {
		from, to := small[i], s
		if s.slower == "" {
			from, to = s, large[i]
		}
		t.Run(s.name, func(t *testing.T) {
			if s.slower == "" {
				started := time.Now()
				Parse([]byte(s.body))
				if took := time.Since(started); took > 2*time.Second {
					t.Errorf("reading %d bytes took %v", len(s.body), took)
				}
			} else {
				t.Log("goldmark v2.1.16 reads " + s.slower + " in more than linear time, so its time is not tested")
			}
			// Twice the growth of the body allows for noise, and so does
			// measuring again before failing, as a shared machine can slow
			// one measurement where growth with the square of the body's
			// length slows every one.
			longer := float64(len(to.body)) / float64(len(from.body))
			var before, after time.Duration
			for range 3 {
				before, after = outlineTime(from.body), outlineTime(to.body)
				if after <= 20*time.Millisecond || float64(after)/float64(before) <= 2*longer {
					return
				}
			}
			t.Errorf("building the outline took %v for %d bytes and %v for %d", before, len(from.body), after, len(to.body))
		})
	}
}

// outlineTime is the shortest of three builds of body's outline from the
// parser's tree, the parser's own time left out.
func outlineTime(body string) time.Duration {
	src := withLoneCarriageReturnsAsLineFeeds([]byte(body))
	tree := gm.Parse(src)
	best := time.Duration(1 << 62)
	for range 3 {
		runtime.GC()
		started := time.Now()
		outline(src, tree)
		best = min(best, time.Since(started))
	}
	return best
}
