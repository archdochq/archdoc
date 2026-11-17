package markdown

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestEveryCaseIsPlacedByTheRules asserts the position rules of every block
// and piece in every case: each starts at the first byte of its construct,
// lies within its parent, and follows its siblings without overlapping them;
// a text's bytes are its text where it holds no escape or entity; and a
// destination's bytes, resolved, are its destination.
func TestEveryCaseIsPlacedByTheRules(t *testing.T) {
	for _, c := range cases(t) {
		src := withLoneCarriageReturnsAsLineFeeds(c.body)
		for _, problem := range placementProblems(src, Parse(c.body)) {
			t.Errorf("%s: %s", c.name, problem)
		}
	}
}

func placementProblems(src []byte, doc *Node) []string {
	var out []string
	var check func(n *Node, leaf *Node)
	check = func(n *Node, leaf *Node) {
		if n.Start < 0 || n.End < n.Start || n.End > len(src) {
			out = append(out, describe(src, n, "has no span within the body"))
			return
		}
		if n.Type.IsBlock() && len(n.Lines) > 0 && n.Type != CodeBlock && n.Type != HTMLBlock {
			leaf = n
		}
		if p := startProblem(src, n); p != "" {
			out = append(out, describe(src, n, p))
		}
		if p := endProblem(src, n); p != "" {
			out = append(out, describe(src, n, p))
		}
		if !n.Type.IsBlock() && leaf != nil && (n.Start < leaf.Start || n.End > leaf.End) {
			out = append(out, describe(src, n, "lies outside its block "+leaf.Type.String()))
		}
		if n.Type == Text && !strings.ContainsAny(string(src[n.Start:n.End]), `\&`) && string(src[n.Start:n.End]) != n.Text {
			out = append(out, describe(src, n, "holds "+quote(n.Text)))
		}
		if n.HasDestination && !n.Reference {
			written := string(src[n.DestinationSpan.Start:n.DestinationSpan.End])
			if n.AutoLink {
				if !strings.HasSuffix(n.Destination, written) {
					out = append(out, describe(src, n, "has destination "+quote(n.Destination)+" not ending in "+quote(written)))
				}
			} else if resolve(written) != n.Destination && !strings.Contains(written, "&#") {
				// goldmark and html.UnescapeString resolve some numeric
				// references, such as a C1 control's, differently.
				out = append(out, describe(src, n, "has destination "+quote(n.Destination)+" where its bytes are "+quote(written)))
			}
		}
		for i, ch := range n.Children {
			if ch.Start < n.Start || ch.End > n.End {
				out = append(out, describe(src, ch, "lies outside its parent "+n.Type.String()+" "+quote(string(src[n.Start:n.End]))))
			}
			if i > 0 && ch.Start < n.Children[i-1].End {
				out = append(out, describe(src, ch, "overlaps the "+n.Children[i-1].Type.String()+" before it"))
			}
			check(ch, leaf)
		}
	}
	check(doc, nil)
	return out
}

// startProblem says how n does not start at the first byte of its construct.
func startProblem(src []byte, n *Node) string {
	if n.Type == Document || n.Type == LineBreak || n.Start == n.End && (n.Type == Text || n.Start == len(src)) {
		return ""
	}
	if n.Start >= len(src) {
		return "starts at the end of the body"
	}
	first := src[n.Start]
	want := ""
	switch n.Type {
	case Blockquote:
		want = ">"
	case List, ListItem:
		want = "-+*0123456789"
	case Heading:
		if !n.Setext {
			want = "#"
		}
	case CodeBlock:
		if n.Fenced {
			want = "`~"
		}
	case ThematicBreak:
		want = "*-_"
	case HTMLBlock, RawHTML:
		want = "<"
	case FootnoteDefinition, LinkReferenceDefinition:
		want = "["
	case FootnoteReference:
		// goldmark reads ! and any character before ^1] as one; the
		// reference reads text, and KNOWN lists it.
		if n.Start == 0 || src[n.Start-1] != '!' {
			want = "["
		}
	case CodeSpan:
		want = "`"
	case Emphasis, Strong:
		want = "*_"
	case Strikethrough:
		want = "~"
	case Image:
		want = "!"
	case Link:
		if !n.AutoLink {
			want = "["
		}
	case TableCell:
		if n.Start < n.End && n.Start > 0 && src[n.Start-1] != '|' && (first == ' ' || first == '\t') {
			return "starts at a space not after a pipe"
		}
	case Paragraph, Text:
		if n.Type == Paragraph && (first == ' ' || first == '\t' || first == '\n') {
			return "starts at a space"
		}
	}
	if want != "" && !strings.ContainsRune(want, rune(first)) {
		return "starts at " + quote(string(first)) + ", not at one of " + quote(want)
	}
	return ""
}

// endProblem says how n does not end at the last byte of its construct.
func endProblem(src []byte, n *Node) string {
	if n.End == n.Start {
		return ""
	}
	last := src[n.End-1]
	want := ""
	switch n.Type {
	case CodeSpan:
		want = "`"
	case Emphasis, Strong:
		want = "*_"
	case Strikethrough:
		want = "~"
	case Link, Image:
		if !n.AutoLink {
			want = ")]"
		}
	case FootnoteReference:
		want = "]"
	case CodeBlock:
		if n.Closed {
			want = string(src[n.Start])
		}
	}
	if want != "" && !strings.ContainsRune(want, rune(last)) {
		return "ends at " + quote(string(last)) + ", not at one of " + quote(want)
	}
	if n.Type.IsBlock() && n.Type != Document && (last == ' ' || last == '\t' || last == '\n' || last == '\r') && n.Type != TableCell {
		return "ends at a space or line ending"
	}
	return ""
}

// resolve resolves backslash escapes and entities as a destination's are,
// in one pass, so an escaped & begins no entity.
func resolve(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", s[i+1]) >= 0:
			i++
			b.WriteByte(s[i])
		case s[i] == '&':
			m := entity.FindString(s[i:])
			// html.UnescapeString also decodes a known name's prefix, as in
			// &Auml0;, which is no entity.
			if d := html.UnescapeString(m); m != "" && (strings.HasPrefix(m, "&#") || !strings.HasSuffix(d, ";")) {
				b.WriteString(d)
				i += len(m) - 1
			} else {
				b.WriteByte('&')
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// entity matches what CommonMark reads as an entity at the start of a
// string: one ending in ;, where html.UnescapeString also reads some without.
var entity = regexp.MustCompile(`^&(#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});`)

func describe(src []byte, n *Node, problem string) string {
	text := ""
	if n.Start >= 0 && n.End <= len(src) && n.Start <= n.End {
		text = string(src[n.Start:n.End])
	}
	return n.Type.String() + " " + quote(text) + " at " + strconv.Itoa(n.Start) + " " + problem
}

func quote(s string) string {
	if len(s) > 60 {
		s = s[:60] + "..."
	}
	return "\"" + strings.ReplaceAll(s, "\n", `\n`) + "\""
}
