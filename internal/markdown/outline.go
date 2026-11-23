// Package markdown reads a document's body into an outline: the tree of its
// blocks and inline pieces as goldmark parses them, each holding its position
// in the body as written. It knows nothing of documents, sections or rules,
// and is the only package that imports goldmark.
package markdown

// Type is the type of a block or piece. Its name is the element the
// reference's XML gives the same construct.
type Type uint8

const (
	Document Type = iota
	Blockquote
	List
	ListItem
	FootnoteDefinition
	Paragraph
	Heading
	ThematicBreak
	CodeBlock
	HTMLBlock
	Table
	TableHeader
	TableRow
	TableCell
	LinkReferenceDefinition

	Text
	LineBreak
	CodeSpan
	Emphasis
	Strong
	Strikethrough
	Link
	Image
	RawHTML
	FootnoteReference
)

var typeNames = [...]string{
	Document:                "document",
	Blockquote:              "block_quote",
	List:                    "list",
	ListItem:                "item",
	FootnoteDefinition:      "footnote_definition",
	Paragraph:               "paragraph",
	Heading:                 "heading",
	ThematicBreak:           "thematic_break",
	CodeBlock:               "code_block",
	HTMLBlock:               "html_block",
	Table:                   "table",
	TableHeader:             "table_header",
	TableRow:                "table_row",
	TableCell:               "table_cell",
	LinkReferenceDefinition: "link_reference_definition",
	Text:                    "text",
	LineBreak:               "linebreak",
	CodeSpan:                "code",
	Emphasis:                "emph",
	Strong:                  "strong",
	Strikethrough:           "strikethrough",
	Link:                    "link",
	Image:                   "image",
	RawHTML:                 "html_inline",
	FootnoteReference:       "footnote_reference",
}

func (t Type) String() string { return typeNames[t] }

// IsBlock reports whether the type is a block rather than an inline piece.
func (t Type) IsBlock() bool { return t < Text }

// Span is a range of bytes in the body, the end exclusive.
type Span struct{ Start, End int }

// Line is one line of a leaf block's content: the bytes of the body it was
// read from, and the columns of a tab before them that a container's marker
// covered only in part, which the content holds as spaces.
type Line struct {
	Span
	Padding int
}

// Node is a block or an inline piece of the outline.
type Node struct {
	Type Type
	Span
	Children []*Node

	// Lines is a leaf block's content, line by line.
	Lines []Line
	// Text is a text's text with escapes and entities resolved, a code span's
	// or code block's content, raw HTML and an HTML block as written, and an
	// autolink's text.
	Text string

	Level     int  // a heading's level
	Setext    bool // a heading written with an underline
	Ordered   bool // a list
	Number    int  // an ordered list's first number
	Delimiter byte // a list's marker: '-', '+', '*', '.' or ')'
	Tight     bool // a list
	Fenced    bool // a code block
	Info      string
	Closed    bool   // a fenced code block or an HTML block its end marker closes
	Align     string // a table cell: "left", "center", "right" or ""
	Hard      bool   // a line break

	// ClosedByMarker is an HTML block of CommonMark types 1 to 5, opened by
	// script, pre, style or textarea, a comment, a processing instruction, a
	// declaration or CDATA, which only its end marker closes.
	ClosedByMarker bool

	// Destination and Title are a link's, an image's or a link reference
	// definition's, resolved. Label is a reference's or a definition's label
	// as written, which the parser matches labels by, ignoring case and runs
	// of spaces for a link and neither for a footnote. Each part's span is its bytes as written, and a
	// link's or an image's TextSpan the bytes between its brackets. A link
	// or image by reference has its definition's destination and title, and
	// no span for either.
	TextSpan                              Span
	Destination, Title, Label             string
	DestinationSpan, TitleSpan, LabelSpan Span
	HasDestination, HasTitle, HasLabel    bool
	AutoLink                              bool // a link written as <...> or found by the linkify extension
	Reference                             bool // a link or image by reference
}

// Walk calls fn for n and each node beneath it in document order, skipping
// the nodes beneath any for which fn returns false.
func Walk(n *Node, fn func(*Node) bool) {
	if !fn(n) {
		return
	}
	for _, c := range n.Children {
		Walk(c, fn)
	}
}
