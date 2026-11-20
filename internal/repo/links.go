package repo

import (
	"html"
	"iter"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"archdoc.dev/internal/markdown"
)

// Links returns every markdown link in the body, in the order their
// destinations appear: inline links and images, and link reference
// definitions, whose destinations are targets like any other. An autolink
// and a link by reference are not listed; References lists the second. A
// footnote, [^label]:, is GitHub's and holds prose, so it is not one.
func (d *Document) Links() []Link {
	links, _ := d.links()
	return links
}

// References returns each use of a link reference definition, [text][label],
// [label][] or [label], with the destination of the first definition of its
// label. Its position is its text's closing bracket.
func (d *Document) References() []Link {
	_, uses := d.links()
	return uses
}

// links reads the links and the reference uses together from the outline.
func (d *Document) links() (links, uses []Link) {
	if d.outline == nil {
		return nil, nil
	}
	starts := lineStarts(d.Body)
	text, textAt := linkText(d.Body, d.outline)
	at := func(offset int) (line, column int) {
		i := lineIn(starts, offset)
		if i > 0 {
			column = offset - starts[i-1]
		} else {
			column = offset
		}
		return d.BodyLine + i, column
	}
	markdown.Walk(d.outline, func(n *markdown.Node) bool {
		switch {
		case n.Type == markdown.LinkReferenceDefinition:
			line, column := at(n.DestinationSpan.Start)
			links = append(links, withParts(Link{Text: n.Label, Written: d.Body[n.DestinationSpan.Start:n.DestinationSpan.End], Type: LinkDefinition,
				Line: line, Start: column, End: column + n.DestinationSpan.End - n.DestinationSpan.Start}, n.Destination))
		case n.Type != markdown.Link && n.Type != markdown.Image, n.AutoLink:
		case n.Reference:
			line, column := at(n.TextSpan.End)
			uses = append(uses, withParts(Link{Text: normalLabel(n.Label), Type: LinkReference,
				Line: line, Start: column, End: column}, n.Destination))
		default:
			kind := LinkInline
			if n.Type == markdown.Image {
				kind = LinkImage
			}
			line, column := at(n.DestinationSpan.Start)
			links = append(links, withParts(Link{Text: text[textAt[n.TextSpan.Start]:textAt[n.TextSpan.End]], Written: d.Body[n.DestinationSpan.Start:n.DestinationSpan.End], Type: kind,
				Line: line, Start: column, End: column + n.DestinationSpan.End - n.DestinationSpan.Start}, n.Destination))
		}
		return true
	})
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Line != links[j].Line {
			return links[i].Line < links[j].Line
		}
		return links[i].Start < links[j].Start
	})
	return links, uses
}

// linkText is body with each HTML comment's bytes spaces, its line endings
// kept, and each carriage return before a line feed removed, and the offset
// in it of each offset in body, so a link's text is one slice of it however
// deeply links and images nest.
func linkText(body string, doc *markdown.Node) (string, []int) {
	blanked := []byte(body)
	markdown.Walk(doc, func(n *markdown.Node) bool {
		if n.Type == markdown.RawHTML && strings.HasPrefix(n.Text, "<!--") {
			for i := n.Start; i < n.End; i++ {
				if blanked[i] != '\n' && blanked[i] != '\r' {
					blanked[i] = ' '
				}
			}
		}
		return true
	})
	out := make([]byte, 0, len(body))
	at := make([]int, len(body)+1)
	for i := range len(body) {
		at[i] = len(out)
		if body[i] == '\r' && i+1 < len(body) && body[i+1] == '\n' {
			continue
		}
		out = append(out, blanked[i])
	}
	at[len(body)] = len(out)
	return string(out), at
}

// decoded is a destination with its percent-encoding decoded, so a path is
// compared as it is on disk, or as written where it does not decode.
func decoded(destination string) string {
	if d, err := url.PathUnescape(destination); err == nil {
		return d
	}
	return destination
}

// withParts sets a link's target, path and fragment from its destination,
// split on the first # before either part is decoded: split after, a %23 in
// the path ended it there and began the fragment.
func withParts(l Link, destination string) Link {
	path, fragment, _ := strings.Cut(destination, "#")
	l.Target, l.Path, l.Fragment = decoded(destination), decoded(path), decoded(fragment)
	return l
}

// normalLabel compares labels as CommonMark does: case folded, with each run
// of whitespace one space.
func normalLabel(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(label), " "))
}

// WikiLinks returns every [[...]] in the body: `[[`, a name holding no `[` or
// `]`, and `]]`, on one line, in text outside a link's or an image's text,
// read from the bytes as written, so an escaped bracket ends none.
func (d *Document) WikiLinks() []WikiLink {
	if d.outline == nil {
		return nil
	}
	starts := lineStarts(d.Body)
	var links []WikiLink
	eachTextRun(d.outline, func(run markdown.Span, heading bool) {
		written := d.Body[run.Start:run.End]
		for _, m := range wikiLinkPattern.FindAllStringSubmatchIndex(written, -1) {
			// Only the outer brackets can be escaped: each inner one follows
			// a bracket.
			if escapedAt(d.Body, run.Start+m[0]) || escapedAt(d.Body, run.Start+m[1]-2) {
				continue
			}
			i := lineIn(starts, run.Start+m[0])
			column := run.Start + m[0]
			if i > 0 {
				column -= starts[i-1]
			}
			at := run.Start + m[0]
			links = append(links, WikiLink{Name: written[m[2]:m[3]], Line: d.BodyLine + i,
				Start: column, End: column + m[1] - m[0], Heading: heading,
				Embed: at > 0 && d.Body[at-1] == '!' && !escapedAt(d.Body, at-1)})
		}
	})
	return links
}

// escapedAt reports whether the byte at is escaped by an odd run of
// backslashes before it.
func escapedAt(s string, at int) bool {
	n := 0
	for i := at - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// eachTextRun calls fn with each run of adjacent text pieces on one line of a
// leaf block, outside a link's or an image's text, and whether the run is in
// a heading.
func eachTextRun(doc *markdown.Node, fn func(run markdown.Span, heading bool)) {
	markdown.Walk(doc, func(n *markdown.Node) bool {
		if n.Type != markdown.Paragraph && n.Type != markdown.Heading && n.Type != markdown.TableCell {
			return n.Type.IsBlock()
		}
		run := markdown.Span{Start: -1}
		flush := func() {
			if run.Start >= 0 {
				fn(run, n.Type == markdown.Heading)
			}
			run = markdown.Span{Start: -1}
		}
		var visit func(*markdown.Node)
		visit = func(p *markdown.Node) {
			for _, c := range p.Children {
				switch c.Type {
				case markdown.Text:
					if run.Start >= 0 && c.Start == run.End {
						run.End = c.End
					} else {
						flush()
						run = c.Span
					}
				case markdown.Link, markdown.Image:
					flush()
				case markdown.Emphasis, markdown.Strong, markdown.Strikethrough:
					visit(c)
				default:
					flush()
				}
			}
		}
		visit(n)
		flush()
		return false
	})
}

// Prose iterates the lines of the body outside code blocks and HTML blocks,
// each with its prose text. Leaving the loop stops the walk.
func (d *Document) Prose() iter.Seq[ProseLine] {
	return func(yield func(ProseLine) bool) {
		if d.outline == nil {
			return
		}
		raw, masked := d.bodyLines(), d.proseLines()
		for i := range raw {
			if masked[i] == nil {
				continue
			}
			if !yield(ProseLine{Raw: raw[i], Masked: *masked[i], Number: d.BodyLine + i}) {
				return
			}
		}
	}
}

// proseLines is the prose text of each body line, nil for a line of a code
// block or an HTML block. Text is prose where it is not in a heading, not
// within a link's or an image's text, not between an <a> start tag and its
// </a> in the same leaf block, not part of an address with a scheme, from the
// scheme to the next space, and not between a [ and the ] after it on its
// line. Every other byte is a space.
func (d *Document) proseLines() []*string {
	body := d.Body
	out := []byte(body)
	for i := range out {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}
	starts := lineStarts(body)
	literal := map[int]bool{}
	markdown.Walk(d.outline, func(n *markdown.Node) bool {
		switch n.Type {
		case markdown.CodeBlock, markdown.HTMLBlock:
			if n.End > n.Start {
				for i := lineIn(starts, n.Start); i <= lineIn(starts, n.End-1); i++ {
					literal[i] = true
				}
			}
			return false
		case markdown.Paragraph, markdown.TableCell:
			inAnchor := false
			var visit func(*markdown.Node)
			visit = func(p *markdown.Node) {
				for _, c := range p.Children {
					switch c.Type {
					case markdown.Text:
						if !inAnchor {
							copy(out[c.Start:c.End], body[c.Start:c.End])
						}
					case markdown.RawHTML:
						if anchorOpens.MatchString(c.Text) {
							inAnchor = true
						} else if anchorCloses.MatchString(c.Text) {
							inAnchor = false
						}
					case markdown.Link, markdown.Image:
					default:
						visit(c)
					}
				}
			}
			visit(n)
			return false
		}
		return true
	})
	lines := strings.Split(string(out), "\n")
	masked := make([]*string, len(lines))
	for i := range lines {
		if literal[i] {
			continue
		}
		line := []byte(lines[i])
		for _, m := range schemeAddress.FindAllIndex(line, -1) {
			blank(line, m[0], m[1])
		}
		// Each matched pair counts one from its [ to past its ], and what any
		// pair covers is blanked in one sweep: blanking each pair's span in
		// turn blanked nested ones again for every level.
		var open []int
		depth := make([]int, len(line)+1)
		for p := 0; p < len(line); p++ {
			switch {
			case line[p] == '[':
				open = append(open, p)
			case line[p] == ']' && len(open) > 0:
				depth[open[len(open)-1]]++
				depth[p+1]--
				open = open[:len(open)-1]
			}
		}
		for p, d := 0, 0; p < len(line); p++ {
			if d += depth[p]; d > 0 {
				line[p] = ' '
			}
		}
		s := string(line)
		masked[i] = &s
	}
	return masked
}

var (
	anchorOpens   = regexp.MustCompile(`(?i)^<a[\s>/]`)
	anchorCloses  = regexp.MustCompile(`(?i)^</a\s*>`)
	schemeAddress = regexp.MustCompile(`(?i)\b(?:[a-z][a-z0-9+.\-]*://|www\.)\S*`)
)

// explicitAnchors are the ids and names of <a> elements in raw HTML and HTML
// blocks, outside comments, their entities resolved, leaving out a footnote
// definition nothing references, which GitHub does not render. An explicit
// anchor is a target too: GLOSSARY.md writes one for every former name of a
// term, so a link written before a rename still resolves.
func explicitAnchors(doc *markdown.Node) []string {
	if doc == nil {
		return nil
	}
	referenced := footnoteNumbers(doc)
	var anchors []string
	markdown.Walk(doc, func(n *markdown.Node) bool {
		var text string
		switch n.Type {
		case markdown.FootnoteDefinition:
			_, ok := referenced[n.Label]
			return ok
		case markdown.RawHTML:
			if strings.HasPrefix(n.Text, "<!--") {
				return false
			}
			text = n.Text
		case markdown.HTMLBlock:
			text = stripComments(n.Text)
		default:
			return true
		}
		for _, m := range explicitAnchor.FindAllStringSubmatch(text, -1) {
			anchors = append(anchors, html.UnescapeString(m[1]+m[2]))
		}
		return false
	})
	return anchors
}

// commentMasked is body with every byte of each HTML comment in an HTML block
// of doc a space, one left open running to the block's end. A comment within
// a paragraph shares it with text, so it never leaves a section empty.
func commentMasked(body string, doc *markdown.Node) []byte {
	out := []byte(body)
	if doc == nil {
		return out
	}
	markdown.Walk(doc, func(n *markdown.Node) bool {
		switch n.Type {
		case markdown.HTMLBlock:
			// The block's content, line by line, with where each byte is.
			var content []byte
			var at []int
			for _, l := range n.Lines {
				for i := l.Start; i < l.End; i++ {
					content = append(content, body[i])
					at = append(at, i)
				}
			}
			for _, c := range commentSpans(string(content)) {
				for i := c.Start; i < c.End; i++ {
					if out[at[i]] != '\n' && out[at[i]] != '\r' {
						out[at[i]] = ' '
					}
				}
			}
			return false
		}
		return true
	})
	return out
}

// commentSpans are the HTML comments in s, `<!-->` and `<!--->` among them, one
// left open running to the end.
func commentSpans(s string) []markdown.Span {
	var spans []markdown.Span
	for from := 0; ; {
		open := strings.Index(s[from:], "<!--")
		if open < 0 {
			return spans
		}
		open += from
		rest := s[open+4:]
		end := len(s)
		switch {
		case strings.HasPrefix(rest, ">"):
			end = open + 5
		case strings.HasPrefix(rest, "->"):
			end = open + 6
		default:
			if close := strings.Index(rest, "-->"); close >= 0 {
				end = open + 4 + close + 3
			}
		}
		spans = append(spans, markdown.Span{Start: open, End: end})
		from = end
	}
}

// stripComments is s with every HTML comment removed, a comment left open
// running to the end.
func stripComments(s string) string {
	var b strings.Builder
	for {
		open := strings.Index(s, "<!--")
		if open < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:open])
		rest := s[open+4:]
		switch {
		case strings.HasPrefix(rest, ">"):
			s = rest[1:]
		case strings.HasPrefix(rest, "->"):
			s = rest[2:]
		default:
			end := strings.Index(rest, "-->")
			if end < 0 {
				return b.String()
			}
			s = rest[end+3:]
		}
	}
}
