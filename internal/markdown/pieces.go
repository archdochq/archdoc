package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	east "github.com/yuin/goldmark/v2/extension/ast"
	"github.com/yuin/goldmark/v2/text"
)

// pieces builds the inline pieces of the leaf block o, read from n. Each
// piece the parser gives no position is found from the end of the piece
// before it, or from the start of the block's content.
func (b *builder) pieces(n ast.Node, o *Node) []*Node {
	cur := o.Start
	if len(o.Lines) > 0 {
		cur = o.Lines[0].Start
	}
	pieces := b.inlines(n, &cur)
	// A block does not end with a line break, though the parser can leave
	// the flag on the last text of a paragraph whose next line it took as a
	// table's header.
	if k := len(pieces); k > 0 && pieces[k-1].Type == LineBreak {
		pieces = pieces[:k-1]
	}
	// Nor does its content end with spaces or tabs.
	if k := len(pieces); k > 0 && pieces[k-1].Type == Text {
		t := pieces[k-1]
		for t.End > t.Start && (b.src[t.End-1] == ' ' || b.src[t.End-1] == '\t') && strings.HasSuffix(t.Text, string(b.src[t.End-1])) {
			t.End--
			t.Text = t.Text[:len(t.Text)-1]
		}
	}
	return pieces
}

func (b *builder) inlines(n ast.Node, cur *int) []*Node {
	var out []*Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(ast.BlockNode); ok {
			continue
		}
		out = append(out, b.piece(c, cur)...)
	}
	return out
}

func (b *builder) piece(n ast.Node, cur *int) []*Node {
	switch v := n.(type) {
	case *ast.Text:
		return b.text(v, cur)
	case *ast.CodeSpan:
		o := &Node{Type: CodeSpan}
		var buf bytes.Buffer
		_, _ = v.Value.WriteTo(&buf, b.src)
		o.Text = withoutCarriageReturns(buf.String())
		if v.Value.IsOwned() {
			o.Span = b.findCodeSpan(*cur)
		} else {
			o.Span = b.codeSpanAround(multiSpan(v.Value))
		}
		*cur = o.End
		return []*Node{o}
	case *ast.Emphasis:
		return []*Node{b.delimited(n, Emphasis, 1, cur)}
	case *ast.Strong:
		return []*Node{b.delimited(n, Strong, 2, cur)}
	case *east.Strikethrough:
		return []*Node{b.delimited(n, Strikethrough, 0, cur)}
	case *ast.Link:
		return []*Node{b.link(Link, n, v.Destination, v.Title, v.Reference, cur)}
	case *ast.Image:
		return []*Node{b.link(Image, n, v.Destination, v.Title, v.Reference, cur)}
	case *ast.AutoLink:
		o := &Node{Type: Link, AutoLink: true, HasDestination: true}
		o.Destination, o.Text = v.Destination.Value(b.src), v.Label.Value(b.src)
		if v.Text.IsOwned() {
			o.Span = Span{n.Pos(), n.Pos() + len(o.Text)}
		} else {
			i := v.Text.Index()
			o.Span = Span{i.Start, i.Stop}
		}
		o.DestinationSpan = o.Span
		if b.src[o.Start] == '<' {
			o.DestinationSpan = Span{o.Start + 1, o.End - 1}
		}
		*cur = o.End
		return []*Node{o}
	case *ast.RawHTML:
		o := &Node{Type: RawHTML, Text: withoutCarriageReturns(v.Value.Value(b.src)), Span: multiSpan(v.Value)}
		*cur = o.End
		return []*Node{o}
	case *east.FootnoteReference:
		o := &Node{Type: FootnoteReference, Label: v.Label.Value(b.src), HasLabel: true}
		// The parser places a reference written ![^1] at its !, and reads
		// ! and any character before ^1] as one.
		o.Start = n.Pos()
		if b.src[o.Start] == '!' {
			o.Start++
		}
		o.End = o.Start + bytes.IndexByte(b.src[o.Start:], ']') + 1
		o.LabelSpan = Span{o.Start + 2, o.End - 1}
		*cur = o.End
		return []*Node{o}
	}
	return nil
}

// backslashBreak reports whether the line break after i is a backslash
// before the line ending.
func (b *builder) backslashBreak(i int) bool {
	for i < len(b.src) && b.src[i] != '\n' && b.src[i] != '\r' {
		i++
	}
	return i > 0 && b.src[i-1] == '\\'
}

func (b *builder) text(v *ast.Text, cur *int) []*Node {
	o := &Node{Type: Text, Text: v.Value.Value(b.src)}
	if v.Value.IsOwned() {
		start := *cur
		if i := bytes.Index(b.src[*cur:], []byte(o.Text)); i >= 0 {
			start += i
		}
		o.Span = Span{start, start + len(o.Text)}
	} else {
		i := v.Value.Index()
		o.Span = Span{i.Start, i.Stop}
		// The parser places delimiters left over from emphasis at the
		// start of their run, before the ones it used.
		if o.Start < *cur && bytes.HasPrefix(b.src[*cur:], b.src[o.Start:o.End]) {
			o.Span = Span{*cur, *cur + o.End - o.Start}
		}
	}
	*cur = o.End
	if !v.SoftLineBreak() && !v.HardLineBreak() {
		return []*Node{o}
	}
	// Spaces and tabs before a line break are the break's, not the text's,
	// unless a backslash makes the break.
	if !v.Value.IsOwned() && !b.backslashBreak(o.End) {
		for o.End > o.Start && strings.IndexByte(" \t\r", b.src[o.End-1]) >= 0 {
			o.End--
			// A space written as an entity is the text's.
			if c := b.src[o.End]; c != '\r' {
				if !strings.HasSuffix(o.Text, string(c)) {
					o.End++
					break
				}
				o.Text = o.Text[:len(o.Text)-1]
			}
		}
		o.Text = strings.TrimSuffix(o.Text, "\r")
	}
	end := o.End
	for end < len(b.src) && b.src[end] != '\n' && b.src[end] != '\r' {
		end++
	}
	*cur = end
	return []*Node{o, {Type: LineBreak, Hard: v.HardLineBreak(), Span: Span{o.End, end}}}
}

// codeSpanAround returns the span of the code span whose content is c: the
// backticks either side of it, and between them and the content the space
// or line ending stripped from either end, and the markers and indentation
// of the containers on a line the span continues on. The closing run gives
// the length, as a backtick before the opening run can be escaped and a
// backslash inside a code span cannot escape one.
func (b *builder) codeSpanAround(c Span) Span {
	e := c.End
	for e < len(b.src) && b.src[e] != '`' && isPrefixByte(b.src[e]) {
		e++
	}
	n := run(b.src, e, '`')
	s := c.Start
	for s > 0 && b.src[s-1] != '`' && isPrefixByte(b.src[s-1]) {
		s--
	}
	return Span{max(s-n, 0), e + n}
}

// isPrefixByte reports whether c can stand between a code span's content
// and its backticks: the space or line ending stripped from the content, and
// the markers and indentation of the containers on the line after one.
func isPrefixByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '>'
}

// findCodeSpan finds the first code span from i, as CommonMark reads one: a
// run of backticks closed by the next run of the same length.
func (b *builder) findCodeSpan(i int) Span {
	for i < len(b.src) {
		j := bytes.IndexByte(b.src[i:], '`')
		if j < 0 {
			break
		}
		open := i + j
		n := run(b.src, open, '`')
		for k := open + n; k < len(b.src); {
			m := bytes.IndexByte(b.src[k:], '`')
			if m < 0 {
				break
			}
			k += m
			if r := run(b.src, k, '`'); r == n {
				return Span{open, k + n}
			} else {
				k += r
			}
		}
		i = open + n
	}
	return Span{i, i}
}

func run(src []byte, i int, c byte) int {
	n := 0
	for i+n < len(src) && src[i+n] == c {
		n++
	}
	return n
}

// delimited builds emphasis, strong emphasis or strikethrough, whose k
// delimiters open and close it next to its text. The parser places it at the
// start of the run of delimiters it was taken from. Strikethrough's k, 0
// here, is the run of tildes on either side of its text.
func (b *builder) delimited(n ast.Node, t Type, k int, cur *int) *Node {
	o := &Node{Type: t}
	entry := *cur
	inner := max(n.Pos(), *cur)
	o.Children = b.inlines(n, &inner)
	if len(o.Children) == 0 {
		o.Start = max(n.Pos(), *cur)
		o.End = o.Start + 2*max(k, 1)
	} else {
		first, last := o.Children[0].Start, o.Children[len(o.Children)-1].End
		if o.Children[len(o.Children)-1].Type == LineBreak {
			// The closing delimiters begin the next line, after the markers
			// of its containers.
			for last < len(b.src) && isPrefixByte(b.src[last]) && b.src[last] != '`' {
				last++
			}
		}
		if k == 0 {
			before, after := 0, run(b.src, last, '~')
			for first-before > entry && b.src[first-before-1] == '~' {
				before++
			}
			k = min(before, after)
		}
		o.Start, o.End = first-k, last+k
	}
	*cur = o.End
	return o
}

// link builds a link or an image, whose text starts after its [ or ![.
func (b *builder) link(t Type, n ast.Node, dest text.SingleLineValue, title text.MultiLineValue, ref *ast.ReferenceLink, cur *int) *Node {
	o := &Node{Type: t}
	k := 1
	if t == Image {
		k = 2
	}
	o.Start = n.Pos()
	inner := o.Start + k
	o.Children = b.inlines(n, &inner)
	rb := inner + bytes.IndexByte(b.src[inner:], ']')
	o.Destination, o.Title = dest.Value(b.src), title.Value(b.src)
	o.TextSpan = Span{o.Start + k, rb}
	if ref != nil {
		o.Reference, o.HasLabel = true, true
		o.Label = ref.Value.Value(b.src)
		switch ref.ReferenceLinkKind {
		case ast.ReferenceLinkKindFull:
			o.LabelSpan = multiSpan(ref.Value)
			o.End = o.LabelSpan.End + 1
			if o.LabelSpan.Start < 0 {
				o.End = rb + 2 + bytes.IndexByte(b.src[rb+2:], ']') + 1
			}
		case ast.ReferenceLinkKindCollapsed:
			o.LabelSpan = Span{o.Start + k, rb}
			o.End = rb + 1 + bytes.IndexByte(b.src[rb+1:], ']') + 1
			o.Label = string(b.src[o.LabelSpan.Start:o.LabelSpan.End])
		default:
			o.LabelSpan, o.End = Span{o.Start + k, rb}, rb+1
			o.Label = string(b.src[o.LabelSpan.Start:o.LabelSpan.End])
		}
		*cur = o.End
		return o
	}
	o.HasDestination = true
	p := rb + 2
	if !dest.IsOwned() {
		i := dest.Index()
		o.DestinationSpan = Span{i.Start, i.Stop}
		p = b.pastAngle(i.Stop)
	} else {
		p = skipSpace(b.src, p)
		if p+1 < len(b.src) && b.src[p] == '<' && b.src[p+1] == '>' {
			o.DestinationSpan = Span{p + 1, p + 1}
			p += 2
		} else {
			o.DestinationSpan = Span{p, p}
		}
	}
	if !title.IsEmpty() {
		o.HasTitle, o.TitleSpan = true, multiSpan(title)
		p = o.TitleSpan.End + 1
	} else if q := skipSpace(b.src, p); q < len(b.src) && strings.IndexByte(`"'(`, b.src[q]) >= 0 && b.src[q] != ')' {
		// An empty title, written "", '' or ().
		o.HasTitle, o.TitleSpan = true, Span{q + 1, q + 1}
		p = q + 2
	}
	o.End = p + bytes.IndexByte(b.src[p:], ')') + 1
	*cur = o.End
	return o
}

func skipSpace(src []byte, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i++
	}
	return i
}
