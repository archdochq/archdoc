---
title: Markdown
includes: [ADR-0007, ADR-0026, RFC-0008, RFC-0009, RFC-0011, RFC-0019]
---

# Markdown

The body of a document is everything after the closing `---` of the front matter. The tool reads it with goldmark into an outline of its blocks and inline pieces, each holding its position in the body as written, and reads every construct below from that outline. A carriage return before a line feed is part of the line ending, and a construct is recognised in a CRLF file exactly as in an LF one. A body longer than 131,072 bytes is not read: the tool finds no heading, section, anchor, link, question, entry or definition in it.

## Headings

Headings are read from the body's outline, as goldmark parses it: ATX headings, `#` through `######` at most three spaces in, and setext headings, the lines of a paragraph followed by a line of `=` or `-`. A heading of nothing but a closing sequence, such as `## #`, is empty. A heading's text is its content, without its markers, closing sequence or underline, a setext heading's lines joined by a line feed, with each HTML comment in it replaced by spaces and its ends trimmed. A line that is a heading only once a comment on it is masked, such as `##<!-- c --> Proposal`, is a paragraph. A top-level heading, one that no blockquote, list item or footnote definition holds, bounds a section and opens a `contents` entry; a heading inside one has an anchor and does neither. The body after a heading starts on the line after its last, a setext heading's underline.

The first top-level heading is an H1 of the form `<id>: <title>` for a numbered type, or `<title>` for a spec page, a term or a group.

## Anchors

Every heading has an anchor, computed as GitHub computes it, from the heading's pieces as goldmark reads them, rendered to text: a text with its backslash escapes and HTML entities resolved, a code span's content, a link's text, an autolink's address as written, an address the linkify extension finds included, a footnote reference's number, and nothing for an image or raw HTML, a comment included. Emphasis, strong emphasis and strikethrough render as their text, and a delimiter goldmark pairs with nothing is text, so `## The _id field` keeps its underscore. The text is lowercased, everything that is not a letter, mark, decimal digit, letter number, connector punctuation, hyphen or space is removed, and each space becomes a hyphen: `Ⅻ` is kept, and `²` and `½` are removed. `## <!-- note --> Scope` anchors as `-scope`, and `## The [RFC-0007](path) approach` as `the-rfc-0007-approach`. Anchors are assigned in the order GitHub renders the headings: each heading outside a footnote definition, in document order, then the headings of each footnote definition in the order its label is first referenced, a footnote reference numbered in the same order. A definition indented under another is a footnote of its own, its headings not the other's. A definition that nothing references is not rendered, and its headings have no anchor. A heading whose anchor is already taken within the document gains a `-1`, `-2` suffix. A heading in isolation has no anchor. An explicit anchor is the `id` or `name` of an `<a>` start tag in raw HTML or an HTML block, outside comments, its entities resolved; one in a footnote definition that nothing references is not read, though one in a referenced definition indented under it is. Where goldmark reads a heading otherwise than cmark-gfm, as for a known difference `testdata/commonmark/KNOWN` lists, the anchor is made from goldmark's reading.

## Links

Links are read from the body's outline, as goldmark reads them: an inline link `[text](destination)`, an image `![text](destination)`, and a link reference definition `[label]: destination`, wherever goldmark reads one, in blockquotes, list items, table cells and footnote definitions included, and never in a code span, a code block or an HTML block. Where goldmark reads a link otherwise than cmark-gfm, as for a known difference `testdata/commonmark/KNOWN` lists, the tool reads goldmark's link. A link's text is its bytes as written between its brackets, its lines joined by a line feed with any quote markers and indentation, and each HTML comment in it replaced by spaces. Its destination is the one goldmark gives, escapes and entities resolved, percent-decoded where it decodes. Its position is that of the destination as written, within any `<` and `>`, and the link is on the line holding it. An empty destination, as in `[text]()`, starts and ends after the `(`.

A label beginning `^` is a footnote, and a footnote definition is not a link reference definition. `[text][label]`, `[text][]` and `[text]` are reference links where the label, or the text for the last two, matches a definition as goldmark matches them. A reference link is not listed among a document's links; its definition is, and the reference is listed among the document's references, with its definition's destination, on the line holding its text's closing bracket. An autolink is not listed.

A `[[...]]` link is read from text as written, its pieces adjacent, so one holding emphasis, a code span or an entity is none: `[[`, a name holding no `[` or `]`, and `]]`, on one line, within text outside a link's or an image's text. A bracket escaped by a backslash ends none, so `\[[a]]` is no `[[...]]` link, and where the document defines the label inside, as in `[[RFC-0007]]` beside `[RFC-0007]: path`, goldmark reads a link within brackets and there is no `[[...]]` link. A `[[...]]` link in a heading is read, and marked as in one.

## Sections

A section is a top-level H2 and everything after it, up to the next top-level heading of equal or higher level. It includes its own subsections. A section is empty if it holds nothing but whitespace and HTML comments.

Section presence is checked by exact H2 text. The required sections by type:

- RFC: `Abstract`, `Motivation`, `Proposal`, `Alternatives considered`, `Backwards compatibility`, in that order. `Rejection rationale` is required when the status is `rejected` and forbidden otherwise. `Sources` is required when `backfilled` is set. A document requiring either has a closing section that is its last H2: `Rejection rationale` when rejected, `Sources` otherwise. A document requiring neither may carry extra sections after the required ones.
- ADR: `Context`, `Decision`, `Alternatives`, `Consequences`, in that order. `Rejection rationale` and `Sources` as for an RFC.
- Ref: `Sources` is the last H2.
- Spec: no requirements.
- Term: no required sections. The definition is everything after the H1 to the end of the file, and L15 wants it to be exactly one paragraph.
- Group: no required sections. Its description is the body after the H1.

Extra H2 sections are permitted between or after the required ones. `Open questions`, `Resolved questions` and `Changelog` are optional for every type, and are read as described below wherever they appear.

## Questions

The question sections are H2s titled `Open questions` and `Resolved questions`. A question is an H3 within one, and its text is the question. Its detail is the lines after it, to its answer or to the next top-level heading. A document carries at most one `Resolved questions` section, and where it has an `Open questions` section, `Resolved questions` is the next H2 or H1 after one.

A question ends with a blockquote when its last top-level block before the next top-level heading, passing over comment blocks and link reference definitions, is a blockquote, as goldmark reads one: its `>` indented by up to three spaces, its lazy lines included, holding any blocks. In `Resolved questions`, that blockquote is the question's answer: its lines with any indentation before the `>`, the `>` and one column of the whitespace after it removed from each, a tab after the marker leaving as spaces the columns it spans past the one taken, and a lazy line as written. A heading in a blockquote is part of it, neither a question nor the end of one.

An `Open questions` section is empty or holds questions and nothing before the first. A `Resolved questions` section holds at least one question and nothing before the first. No question has a heading beneath it. An open question does not end with a blockquote. A resolved question ends with one holding something other than comment blocks, with no other blockquote as the block before it, passing over comment blocks and link reference definitions. No question's heading produces an empty anchor, or the same anchor as another heading in the document, one inside a container included, compared before any suffix is added.

## Changelog entries

A `Changelog` section runs from the line after its H2 to the next top-level H1 or H2. A carriage return before a line feed is removed before a line of it is read, and a blank line holds nothing but spaces and tabs.

An entry is an item of a top-level list in the section, as goldmark reads it, whatever its marker: `-`, `*`, `+`, or a number followed by `.` or `)`. A section may hold more than one list, and its entries are the items of each, in document order. Comment blocks and link reference definitions at the top level of the section are passed over, and any other top-level block belongs to no entry.

An entry's text is its lines as written, with the item's marker and the spaces or tabs after it removed from the first, or one column of them where five or more follow it, the rest being an indented code block, the item's content indentation removed from each later line that has it, a tab spanning past it leaving the rest as spaces, a lazy line as written, and blank lines at either end dropped. Its date is four digits, two and two, separated by `-`, at the start of that text, followed by `:` and then a space or the end of the line, that is a real calendar date; the date, its `:` and one space are removed from the text. An entry without one is undated. A nested item is text within its entry.

Entries are read from every `Changelog` section, in document order. A level two heading differing from `Changelog` only in case, spacing or punctuation is not a `Changelog` section.

## Comments

An HTML comment is read where goldmark reads one: a comment opening a line at most three columns in, past the markers of the blockquotes and list items holding it, opens an HTML block that runs to the line holding `-->`, or to where its container ends, or to the end of the document; and within a paragraph, a heading or a table cell, a `<!--` opens a comment only where a `-->` closes it within the same paragraph, heading or table cell, and is text otherwise. `<!-->` and `<!--->` are complete comments. Text inside a comment is not content: it holds no heading, link or `[[...]]` link, and is not prose. A comment block is an HTML block holding nothing but comments and whitespace. Where goldmark reads a comment otherwise than cmark-gfm, as for a known difference `testdata/commonmark/KNOWN` lists, the tool reads goldmark's.

## Code and HTML blocks

Code blocks, fenced or indented, and HTML blocks are read as goldmark reads them, in blockquotes, list items and footnote definitions too. Nothing inside one is prose: it holds no heading, link or `[[...]]` link, and opens no comment. A block is left open when the body ends inside a top-level fenced code block, or a top-level HTML block that only its end marker closes, one opened by `<script`, `<pre`, `<style` or `<textarea`, a comment, a processing instruction, a declaration or `<![CDATA[`; `reject` refuses such a document, as the section it appends would be read inside the block.

## Masking

A body is read through masked copies of its lines, each the same length as the line as written and differing from it only where a byte became a space, so a position in one is the same position in the other. Masked text is never shown.

- Comment-masked text blanks each HTML comment in an HTML block, one left open running to the block's end. A section's emptiness is read from it.
- Prose text blanks every byte but those of text that is prose, and holds no line of a code block or an HTML block. Text is prose where it is not in a heading, not within a link's or an image's text, not between an `<a` start tag and its `</a>` in the same paragraph or table cell, not part of an address with a scheme or beginning `www.`, from its start to the next space, and not between a `[` and the `]` closing it on its line. A code span, raw HTML and a link reference definition are not text. Suggestions are offered within prose text.
