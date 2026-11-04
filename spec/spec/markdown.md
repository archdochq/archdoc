---
title: Markdown
includes: [ADR-0006, ADR-0007, RFC-0008, RFC-0009, RFC-0011]
---

# Markdown

The body of a document is everything after the closing `---` of the front matter. The tool reads headings, HTML comments, code blocks, HTML blocks and links throughout it, blockquotes and list items as far as they hold those blocks, blockquotes within the question sections, and list items within a `Changelog` section. Everything else is text. A carriage return before a line feed is part of the line ending, and a construct is recognised in a CRLF file exactly as in an LF one.

## Headings

Headings are `#` through `######`, at most three spaces in. A heading of nothing but a closing sequence, such as `## #`, is empty. A line inside a code block or an HTML block is not a heading. This holds wherever headings are read: section parsing, anchors and link scanning. Setext headings are not read.

The first heading is an H1 of the form `<id>: <title>` for a numbered type, or `<title>` for a spec page, a term or a group.

## Anchors

Every heading has an anchor, computed as GitHub computes it, from the heading's content as written, comments included, rendered to text: backslash escapes and HTML entities resolved, a code span's content as written, HTML tags and comments removed, an inline link, or a reference link to a label the document defines, as its text, an image as nothing, an autolink as its address, and a bare `http://`, `https://`, `ftp://` or `www.` address that GitHub links, outside a link's text, as written. An underscore is removed only where it pairs as emphasis, the runs of `*`, `_` and `~` paired as cmark-gfm pairs them, so `## The _id field` keeps its underscore. The text is lowercased, everything that is not a letter, mark, decimal digit, letter number, connector punctuation, hyphen or space is removed, and each space becomes a hyphen: `Ⅻ` is kept, and `²` and `½` are removed. `## <!-- note --> Scope` anchors as `-scope`. A heading whose anchor is already taken within the document gains a `-1`, `-2` suffix. Anchors are assigned by walking a document's headings in order; a heading in isolation has no anchor. `## The [RFC-0007](path) approach` anchors as `the-rfc-0007-approach`.

## Links

A link is `[text](destination)` and an image `![text](destination)`, with the destination bare or within `<...>`, and an optional title after it. The destination may be empty, as in `[text]()`. A title is set apart from the destination by spaces or tabs and is written within `"..."`, `'...'` or `(...)`; anything else before the closing parenthesis makes the brackets text. A bare destination holds parentheses only in balanced pairs, and a backslash before a punctuation character makes it literal. The text may run across the lines of one paragraph, and holds brackets only in balanced pairs. It is kept as written, its lines joined by a line feed, with any quote markers and indentation. A `\[` or `\]` is a literal bracket, and a bracket inside code or an HTML comment is not a bracket. A paragraph ends at a blank line, a code block, an HTML block, a heading, a list item able to interrupt it, a table, a thematic break, a setext underline and the first line of a blockquote; a quote's later lines continue it. A heading, like every other block, is read past the markers of the blockquotes and list items holding the line, so `    # x` continues the paragraph above it; a heading, a thematic break and a setext underline are indented at most three columns past them, a tab running to the next multiple of four. A table begins at a line followed, within the same blockquotes and list items, by a delimiter row of as many cells, such as `| - | :-: |`, indented at most three spaces past their markers, that is not a list item or a setext underline. A paragraph holding a line followed by a delimiter row of another number of cells begins no table after it. Cells are parted at every `|` but one straight after a backslash, one inside a code span or a comment included, and a `|` at either end of a row opens or closes it, so a row of a lone `|` has no cells and begins no table. A table runs to a line with no cells, a blank one among them, or the start of another block, a lone HTML tag included, and each of its rows and cells is read on its own: link text, a code span or a comment does not run from one cell into the next. A line beginning `|` with no delimiter row beneath it is text. The destination follows the closing `]` on the same line, and the link is on that line.

A line holding only `[label]: destination` and an optional title, after a blank line, a heading, a code or HTML block, or another such line, is a link reference definition. A label beginning `^` is a footnote, not a definition. `[text][label]`, `[text][]` and `[text]` are reference links where the label, or the text for the last two, matches a definition, compared without regard to case and with each run of whitespace as one space, and is at most 999 characters. A reference link is not listed among a document's links; its definition is.

An image may be the text of a link. A link within another link's text, inline or by reference, is a link, and the outer one is not.

## Sections

A section is an H2 and everything under it, up to the next heading of equal or higher level. It includes its own subsections. A section is empty if it holds nothing but whitespace and HTML comments.

Section presence is checked by exact H2 text. The required sections by type:

- RFC: `Abstract`, `Motivation`, `Proposal`, `Alternatives considered`, `Backwards compatibility`, in that order. `Rejection rationale` is required when the status is `rejected` and forbidden otherwise. `Sources` is required when `backfilled` is set. A document requiring either has a closing section that is its last H2: `Rejection rationale` when rejected, `Sources` otherwise. A document requiring neither may carry extra sections after the required ones.
- ADR: `Context`, `Decision`, `Alternatives`, `Consequences`, in that order. `Rejection rationale` and `Sources` as for an RFC.
- Ref: `Sources` is the last H2.
- Spec: no requirements.
- Term: no required sections. The definition is everything from the H1 to the end of the file, and L15 wants it to be exactly one paragraph.
- Group: no required sections. Its description is the body after the H1.

Extra H2 sections are permitted between or after the required ones. `Open questions`, `Resolved questions` and `Changelog` are optional for every type, and are read as described below wherever they appear.

## Questions

The question sections are H2s titled `Open questions` and `Resolved questions`. A question is an H3 within one, and its text is the question. Its detail is the lines after it, to its answer or to the next heading. A document carries at most one `Resolved questions` section, and where it has an `Open questions` section, `Resolved questions` is the next H2 or H1 after one.

Within a question section, a blockquote is a run of consecutive lines each beginning with `>` at the start of the line, outside an HTML comment and any code block or HTML block the blockquote does not hold. A question ends with a blockquote when its last one is followed by nothing but blank lines and comments before the next heading or the end of the document. In `Resolved questions`, that blockquote is the question's answer: its lines with the `>` and one column of the whitespace after it removed from each, a tab after the marker leaving two spaces. A link reference definition behind `>` in a question section is read as a link.

An `Open questions` section is empty or holds questions and nothing before the first. A `Resolved questions` section holds at least one question and nothing before the first. No question has a heading beneath it, and no blockquote in a question section holds a heading, a fenced code block or an HTML comment. An open question does not end with a blockquote. A resolved question ends with a non-empty one, with no other blockquote before it separated by only blank lines and comments. No question's heading produces an empty anchor, or the same anchor as another heading in the document, compared before any suffix is added.

## Changelog entries

A `Changelog` section runs from its H2 to the next H1 or H2. A carriage return before a line feed is removed before a line of it is read, and a blank line holds nothing but spaces and tabs.

An entry line begins `- ` at column 0 with a character other than a space or a tab after the space, and is not a line made only of three or more `-`, spaces and tabs. A block opener is a line beginning `#`, `>`, `<`, `-`, `*`, `+`, `_`, `=`, `|`, `[`, a backtick or `~`, or digits followed by `.` or `)`. After an entry line, each line in turn belongs to the entry while it is indented by two or more spaces with no tab before its first non-blank character; or blank, where a later line belongs to the entry; or a line at column 0 that is neither blank nor a block opener, directly after the entry line or another such line, where what follows the entry line's `- ` is not a block opener either. The first other line ends the entry.

Comment lines run from a line beginning `<!--` at column 0 outside every entry to the first line, starting with that one, that holds `-->`, where that line ends with `-->`. A line within them is not an entry line. Every line of the section that is not in an entry, blank or within comment lines belongs to no entry.

An entry's date is four digits, two and two, separated by `-`, directly after `- `, followed by `:` and then a space or the end of the line, that is a real calendar date. An entry without one is undated. Its text is its lines with `- ` and any date, its `:` and one space removed from the first, the first two spaces removed from each later line indented by two or more, and blank lines at either end dropped. A nested item is text within its entry.

Entries are read from every `Changelog` section, in document order. A level two heading differing from `Changelog` only in case, spacing or punctuation is not a `Changelog` section.

## Comments

Text inside an HTML comment is not content anywhere. It is passed over when finding headings and sections, when resolving `[[...]]` links, and when suggesting them. A comment opening a block, its `<!--` at most three columns in, a tab running to the next multiple of four, runs to the first line holding `-->`, or to where the blockquote or list item holding it ends, or to the end of the document where neither comes first; nothing after its `-->` on that line opens another comment, and a comment block closing on the line it opens leaves no paragraph open after it. A `<!--` anywhere else opens a comment only where a `-->` follows it before its paragraph ends, and is text otherwise; a paragraph ends at a line that is blank once its containers' markers are passed, that is a setext underline other than a lazy continuation line, or that starts a block able to interrupt one, read in the blockquotes and list items it continues. A paragraph holding a comment continues past it, so a line after the comment indented four columns continues the paragraph. A `<!--` after an odd run of backslashes is text. Text sharing a line with a comment, before its `<!--` or after its `-->`, is content.

## Code and HTML blocks

A fenced code block opens on a run of three or more backticks or tildes at most three spaces in. A backtick fence's info string holds no backtick, so ```go``` opening a line is a code span. The block closes on a run of the same character at least as long, carrying no info string, or where the blockquote or list item holding it ends. An indented code block is a run of lines indented four or more columns, a tab running to the next multiple of four, that does not continue a paragraph (a setext underline, a run of `=` or `-` alone at most three spaces in, ends the paragraph above it), and ends at the first line indented less.

An HTML block opens a line, at most three spaces in. One opening with `<script`, `<pre`, `<style` or `<textarea` closes on the line holding any of `</script>`, `</pre>`, `</style>` or `</textarea>`; with `<?`, `<![CDATA[` or `<!` and a letter, on the line holding `?>`, `]]>` or `>`, which may be the line it opens on, `<?>` included. One opening with a tag of a block-level HTML element, or outside a paragraph with any single complete tag alone on its line other than one of those four, opening or closing, closes before the next blank line.

Blocks are read inside blockquotes and list items. A line continues a blockquote behind its `>` and a list item while indented to the item's content, or lazily, as a paragraph's continuation that starts no block. A blank line continues a list item only once the item holds something, so an item can begin with at most one blank line. A list item interrupts a paragraph in its own container only if it is not empty and, when numbered, starts at 1; a line that does not continue the paragraph's container starts a list item of any number, or an empty one. A block inside one ends with it.

Nothing inside a code block or an HTML block is prose. It holds no heading, link or `[[...]]` link, opens no comment, and is content. A `<!--` or `-->` inside an inline code span is literal, the span read across the lines of its paragraph, and a fence marker inside a comment is literal: whichever block opens first holds the lines until it closes. A code span closes on a backtick run of exactly the length that opened it, and may run across the lines of one paragraph.

## Masking

Before a body is searched, the text that must not match is masked: the inside of comments and code spans. The lines of code blocks and HTML blocks are not searched. Every masked byte becomes a space. The masked text is the same length as the text as written and differs from it only where a byte became a space: a position in one is the same position in the other. Masked text is never shown.
