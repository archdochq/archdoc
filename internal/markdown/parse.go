package markdown

import (
	"bytes"
	"sort"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	east "github.com/yuin/goldmark/v2/extension/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/text"
)

var gm = parser.New(parser.WithExtensions(
	extension.NewTableParser(),
	extension.NewStrikethroughParser(),
	extension.NewLinkifyParser(),
	extension.NewFootnoteParser(),
))

// Parse reads body into its outline.
func Parse(body []byte) *Node {
	src := withLoneCarriageReturnsAsLineFeeds(body)
	b := builder{src: src}
	doc := b.block(gm.Parse(src))
	b.settleFences(doc)
	return doc
}

// withLoneCarriageReturnsAsLineFeeds returns body, or a copy of it in which
// each carriage return not followed by a line feed is one, as the reference
// reads it. The copy is the body's length, so every position is the same.
func withLoneCarriageReturnsAsLineFeeds(body []byte) []byte {
	var out []byte
	for i, c := range body {
		if c == '\r' && (i+1 == len(body) || body[i+1] != '\n') {
			if out == nil {
				out = bytes.Clone(body)
			}
			out[i] = '\n'
		}
	}
	if out == nil {
		return body
	}
	return out
}

type builder struct {
	src []byte
}

func (b *builder) block(n ast.Node) *Node {
	o := &Node{Span: Span{n.Pos(), n.Pos()}}
	if bn, ok := n.(ast.BlockNode); ok {
		for _, s := range bn.Source() {
			o.Lines = append(o.Lines, Line{Span{s.Start, s.Stop}, s.Padding})
		}
	}
	switch v := n.(type) {
	case *ast.Document:
		o.Type, o.Start = Document, 0
	case *ast.Blockquote:
		o.Type = Blockquote
		o.End = o.Start + 1
	case *ast.List:
		o.Type, o.Ordered, o.Number, o.Delimiter, o.Tight = List, v.IsOrdered(), v.Start, v.Marker, v.IsTight
	case *ast.ListItem:
		o.Type = ListItem
		o.End = b.markerEnd(o.Start)
	case *east.FootnoteDefinition:
		o.Type, o.Label, o.HasLabel = FootnoteDefinition, v.Label.Value(b.src), true
		o.End = o.Start + bytes.Index(b.src[o.Start:], []byte("]:")) + 2
		o.LabelSpan = Span{o.Start + 2, o.End - 2}
	case *ast.Paragraph:
		o.Type = Paragraph
	case *ast.Heading:
		o.Type, o.Level, o.Setext = Heading, v.Level, v.HeadingKind == ast.HeadingKindSetext
	case *ast.ThematicBreak:
		o.Type = ThematicBreak
		o.End = b.lineEnd(o.Start)
	case *ast.CodeBlock:
		b.codeBlock(o, v)
		return o
	case *ast.HTMLBlock:
		b.htmlBlock(o, v)
		return o
	case *east.Table:
		o.Type = Table
	case *east.TableHeader:
		o.Type = TableHeader
	case *east.TableRow:
		o.Type = TableRow
	case *east.TableCell:
		o.Type = TableCell

		o.Align = map[east.Alignment]string{east.AlignLeft: "left", east.AlignCenter: "center", east.AlignRight: "right"}[v.Alignment]
	case *ast.LinkReferenceDefinition:
		b.definition(o, v)
		return o
	}
	inline := false
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*east.TableBody); ok {
			for r := c.FirstChild(); r != nil; r = r.NextSibling() {
				o.Children = append(o.Children, b.block(r))
			}
			continue
		}
		if _, ok := c.(ast.BlockNode); ok {
			o.Children = append(o.Children, b.block(c))
			continue
		}
		inline = true
	}
	if inline {
		o.Children = b.pieces(n, o)
	} else if o.Type == Document || o.Type == Blockquote || o.Type == ListItem || o.Type == FootnoteDefinition {
		// The parser can place a table before the paragraph whose last line
		// it took as the table's header.
		sort.SliceStable(o.Children, func(i, j int) bool { return o.Children[i].Start < o.Children[j].Start })
	}
	b.blockEnd(o)
	return o
}

// placeEmptyCells places the cells of a row the parser gives no position:
// an empty one as written after the pipe closing the cell before it, and
// one the row does not write, which the parser adds to fill the row, at the
// row's end.
func (b *builder) placeEmptyCells(row *Node, rowPos int) {
	end := b.lineEnd(rowPos)
	from := rowPos
	for _, c := range row.Children {
		if c.Start >= 0 {
			from = c.End
			continue
		}
		c.Start, c.End = end, end
		if i := bytes.IndexByte(b.src[min(from, end):end], '|'); i >= 0 {
			c.Start = from + i + 1
			c.End = c.Start
		}
		from = c.End
	}
}

// blockEnd sets the end of a block not already ended by its type: a leaf
// block after its last line, a container after its last child.
func (b *builder) blockEnd(o *Node) {
	switch o.Type {
	case Document:
		o.End = len(b.src)
		return
	case Heading:
		if o.Setext {
			last := o.Start
			if len(o.Lines) > 0 {
				last = o.Lines[len(o.Lines)-1].End
			}
			under := b.nextLine(last)
			if !isUnderline(b.src[under:b.lineEnd(under)]) {
				// The parser can take a paragraph's last line as a table's
				// header and make what is left of the paragraph a heading
				// underlined by a line after the table; it is the paragraph.
				o.Type, o.Setext, o.Level = Paragraph, false, 0
				o.End = trimRight(b.src, o.Start, last)
				return
			}
			o.End = b.lineEnd(under)
		} else {
			o.End = b.lineEnd(o.Start)
		}
		return
	case Paragraph, TableCell:
		if len(o.Lines) > 0 {
			o.End = trimRight(b.src, o.Start, o.Lines[len(o.Lines)-1].End)
		}
	case TableHeader, TableRow:
		b.placeEmptyCells(o, o.Start)
		// A row starts at its first cell's pipe, or its first cell where
		// none opens it; the parser can place a table taking a paragraph's
		// last line as its header at the paragraph's start.
		if len(o.Children) > 0 {
			o.Start = o.Children[0].Start
			if o.Start > 0 && b.src[o.Start-1] == '|' {
				o.Start--
			}
		}
		o.End = b.lineEnd(o.Start)
		return
	case Table:
		if len(o.Children) > 0 {
			o.Start = o.Children[0].Start
		}
	}
	for _, c := range o.Children {
		o.End = max(o.End, c.End)
	}
}

func (b *builder) codeBlock(o *Node, v *ast.CodeBlock) {
	o.Type, o.Fenced = CodeBlock, v.CodeBlockKind == ast.CodeBlockKindFenced
	segs := v.Value.Segments()
	for _, s := range segs {
		o.Lines = append(o.Lines, Line{Span{s.Start, s.Stop}, s.Padding})
	}
	var buf bytes.Buffer
	_, _ = v.Value.WriteTo(&buf, b.src)
	o.Text = withoutCarriageReturns(buf.String())
	if !o.Fenced {
		o.End = trimRight(b.src, o.Start, segs[len(segs)-1].Stop)
		return
	}
	o.Info = v.Info.Value(b.src)
	open := b.lineEnd(o.Start)
	o.End = open
	if len(segs) > 0 {
		o.End = trimRight(b.src, o.Start, segs[len(segs)-1].Stop)
	}
	fence := b.src[o.Start]
	n := 0
	for o.Start+n < len(b.src) && b.src[o.Start+n] == fence {
		n++
	}
	after := b.nextLine(o.Start)
	if len(segs) > 0 {
		after = b.nextLine(segs[len(segs)-1].Start)
	}
	if after < len(b.src) && closesFence(b.src[after:b.lineEnd(after)], fence, n) {
		o.Closed = true
		o.End = b.lineEnd(after)
	}
}

// settleFences reopens each fenced code block whose closing fence, as read
// from the bytes, is a line on which another block starts: the opening fence
// of a block after the container holding the first has closed it.
func (b *builder) settleFences(doc *Node) {
	reopened := false
	var starts []int
	Walk(doc, func(n *Node) bool {
		if n.Type.IsBlock() && n.Type != Document {
			starts = append(starts, n.Start)
		}
		return true
	})
	sort.Ints(starts)
	Walk(doc, func(n *Node) bool {
		if n.Type != CodeBlock || !n.Closed {
			return true
		}
		line := bytes.LastIndexByte(b.src[:n.End], '\n') + 1
		if i := sort.SearchInts(starts, line); i < len(starts) && starts[i] < n.End {
			n.Closed = false
			n.End = b.lineEnd(n.Start)
			if len(n.Lines) > 0 {
				n.End = max(n.End, trimRight(b.src, n.Start, n.Lines[len(n.Lines)-1].End))
			}
			reopened = true
		}
		return true
	})
	if reopened {
		refitContainers(doc)
	}
}

// refitContainers ends each container holding a child after its last child
// again, once a child has ended earlier than it did.
func refitContainers(n *Node) {
	for _, c := range n.Children {
		refitContainers(c)
	}
	switch n.Type {
	case Blockquote, List, ListItem, FootnoteDefinition:
		if len(n.Children) > 0 {
			n.End = n.Children[len(n.Children)-1].End
		}
	}
}

// isUnderline reports whether line, as written, is a setext underline after
// the markers and indentation of its containers.
func isUnderline(line []byte) bool {
	line = bytes.TrimLeft(line, " \t>")
	line = bytes.TrimRight(line, " \t")
	return len(line) > 0 && (len(bytes.Trim(line, "=")) == 0 || len(bytes.Trim(line, "-")) == 0)
}

// closesFence reports whether line, as written, is a closing fence of at
// least n of fence after the markers and indentation of its containers.
func closesFence(line []byte, fence byte, n int) bool {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '>') {
		i++
	}
	run := 0
	for i < len(line) && line[i] == fence {
		i, run = i+1, run+1
	}
	return run >= n && len(bytes.Trim(line[i:], " \t")) == 0
}

func (b *builder) htmlBlock(o *Node, v *ast.HTMLBlock) {
	o.Type = HTMLBlock
	segs := v.Value.Segments()
	for _, s := range segs {
		o.Lines = append(o.Lines, Line{Span{s.Start, s.Stop}, s.Padding})
	}
	var buf bytes.Buffer
	_, _ = v.Value.WriteTo(&buf, b.src)
	o.Text = withoutCarriageReturns(buf.String())
	o.End = trimRight(b.src, o.Start, segs[len(segs)-1].Stop)
	if v.HTMLBlockKind <= ast.HTMLBlockKind5 {
		last := b.src[min(segs[len(segs)-1].Start, o.End):o.End]
		if len(segs) == 1 {
			last = last[min(len(last), htmlOpenLength[v.HTMLBlockKind]):]
		}
		for _, end := range htmlEndMarkers[v.HTMLBlockKind] {
			o.Closed = o.Closed || bytes.Contains(bytes.ToLower(last), []byte(end))
		}
	}
}

// htmlEndMarkers are what close an HTML block of each type that only its end
// marker closes, and htmlOpenLength the bytes of the start marker that end
// cannot overlap.
var htmlEndMarkers = map[ast.HTMLBlockKind][]string{
	ast.HTMLBlockKind1: {"</script>", "</pre>", "</style>", "</textarea>"},
	ast.HTMLBlockKind2: {"-->"}, ast.HTMLBlockKind3: {"?>"}, ast.HTMLBlockKind4: {">"}, ast.HTMLBlockKind5: {"]]>"},
}

var htmlOpenLength = map[ast.HTMLBlockKind]int{
	ast.HTMLBlockKind1: 1, ast.HTMLBlockKind2: 4, ast.HTMLBlockKind3: 2, ast.HTMLBlockKind4: 2, ast.HTMLBlockKind5: 9,
}

func (b *builder) definition(o *Node, v *ast.LinkReferenceDefinition) {
	o.Type = LinkReferenceDefinition
	o.Label, o.LabelSpan, o.HasLabel = v.Label.Value(b.src), multiSpan(v.Label), true
	o.Destination, o.HasDestination = v.Destination.Value(b.src), true
	o.Title = v.Title.Value(b.src)
	o.HasTitle = !v.Title.IsEmpty()
	o.End = o.LabelSpan.End + 2
	if !v.Destination.IsOwned() {
		i := v.Destination.Index()
		o.DestinationSpan = Span{i.Start, i.Stop}
		o.End = b.pastAngle(i.Stop)
	}
	if o.HasTitle {
		o.TitleSpan = multiSpan(v.Title)
		o.End = o.TitleSpan.End + 1
	}
}

// pastAngle returns i, or i+1 where the byte there closes a destination
// written within < and >.
func (b *builder) pastAngle(i int) int {
	if i < len(b.src) && b.src[i] == '>' {
		return i + 1
	}
	return i
}

func multiSpan(v interface {
	IsOwned() bool
	Indices() []text.Index
}) Span {
	if v.IsOwned() {
		return Span{-1, -1}
	}
	idx := v.Indices()
	if len(idx) == 0 {
		return Span{-1, -1}
	}
	return Span{idx[0].Start, idx[len(idx)-1].Stop}
}

// markerEnd returns the end of the list marker at i.
func (b *builder) markerEnd(i int) int {
	for i < len(b.src) && b.src[i] >= '0' && b.src[i] <= '9' {
		i++
	}
	return i + 1
}

// lineEnd returns the end of the line holding i, before its line ending and
// any spaces and tabs ending it.
func (b *builder) lineEnd(i int) int {
	start := i
	for i < len(b.src) && b.src[i] != '\n' && b.src[i] != '\r' {
		i++
	}
	return trimRight(b.src, start, i)
}

// nextLine returns the start of the line after the one holding i.
func (b *builder) nextLine(i int) int {
	if j := bytes.IndexByte(b.src[i:], '\n'); j >= 0 {
		return i + j + 1
	}
	return len(b.src)
}

// withoutCarriageReturns removes each carriage return from content read as
// written. Every one left in the parsed copy is followed by a line feed, so
// belongs to a line ending.
func withoutCarriageReturns(s string) string { return strings.ReplaceAll(s, "\r", "") }

func trimRight(src []byte, start, end int) int {
	for end > start && (src[end-1] == ' ' || src[end-1] == '\t' || src[end-1] == '\n' || src[end-1] == '\r') {
		end--
	}
	return end
}
