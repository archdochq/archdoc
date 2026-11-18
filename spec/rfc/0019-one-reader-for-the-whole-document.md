---
id: RFC-0019
title: One reader for the whole document
status: proposed
created: 2026-11-13
decided:
depends: [ADR-0007, ADR-0009, ADR-0019, ADR-0023, ADR-0026]
updates: [RFC-0002, RFC-0006, RFC-0007, RFC-0008, RFC-0009, RFC-0011, RFC-0012, RFC-0016]
obsoletes: []
---

# RFC-0019: One reader for the whole document

## Abstract

A document's body is parsed once with goldmark into an outline of its blocks and inline pieces, each holding its position in the file, and every reader the tool has reads that outline: sections, headings and their anchors, links, `[[...]]` links, suggestions, questions, `Changelog` entries and term definitions. L20 and L21 stop reporting shapes they report only because the tool cannot read them, and a `Changelog` entry is any item of a list in the section. A body longer than 131,072 bytes is not read, and L23 reports it. The outline is checked against cmark-gfm's parse of recorded cases, and replaces the current readers in five stages.

## Motivation

The tool reads a body with several readers, each implementing part of CommonMark: a block walk that writes masked copies of each line, a link scanner over one of them, a renderer of heading text for anchors, a pattern-based masker for suggestions, and readers for `Changelog` entries, answers and term definitions, the first reading lines as written and the other two reading the walk's lines in ways of their own. Where one of them and GitHub disagree, L17 reports a link that works or passes one that does not, and `export`, `renumber`, `term remove` and `link` act on the same misreading. Where two of them disagree, one piece of text is a heading to one reader and prose to another.

Each of these was checked against cmark-gfm 0.29.0.gfm.13 and against the tool at [4cae902]:

| Body | cmark-gfm | The tool |
|---|---|---|
| `> ## Quoted`, then `[a](#quoted)` | a heading, to which GitHub gives the anchor `quoted`, and a link to it | no heading; L17 reports the link |
| `Underlined` / `---`, then `[b](#underlined)` | a level two heading, to which GitHub gives the anchor `underlined`, and a link to it | no heading; L17 reports the link |
| `\| a \|` / `\|---\|` / `\| b \| [c](gone.md) \|` | a table of one column; the second cell is not read | L17 reports `gone.md` |
| `` ` a `` / `[d](gone.md) <!--` / ``b` `` / `-->` | one code span holding the link and the comment | L17 reports `gone.md` |
| An answer whose blockquote holds a fenced code block | a blockquote holding a code block | L20 reports the fence |
| `1. 2026-11-13: Numbered entry.` in a `Changelog` | a list item | L21 reports a line belonging to no entry |

The last two are restrictions that exist only because the tool cannot read those shapes. A correction to one reader leaves the others as they were, and corrections to the inline readers have moved misreadings rather than removed them, because no order of blanking and matching reproduces the order in which CommonMark claims text. [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md) has the tool read a body with goldmark, against cmark-gfm's parse. This design is how it does so.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Parser | goldmark v2, the module `github.com/yuin/goldmark/v2`, at the version `go.mod` names, with its table, strikethrough, linkify and footnote extensions, and without its task list extension |
| Reference | cmark-gfm 0.29.0.gfm.13 with the extensions `table`, `strikethrough`, `autolink`, `tasklist` and `footnotes`. Its parse of a body is the reading the outline is to give. Its rendered HTML, and what GitHub's sanitiser does with that HTML, are not part of it |
| Known difference | A recorded case on which the outline and the reference's parse differ, listed with the reason, until the parser reads it as the reference does |
| Body | As now: everything after the closing delimiter of the front matter, or the whole file where there is none |
| Line | A body is divided into lines at each line feed, as now, and a line number counts them. A carriage return before a line feed belongs to the line ending. The parser reads a carriage return not followed by a line feed as a line ending, as the reference does, since it is given a copy with each such byte read as a line feed (What is parsed), so a block or piece after one is reported on the line holding it, and the masked texts are divided at line feeds alone. Where a reader takes a block's lines as written, as an entry's text and an answer's body are taken, a lone carriage return divides them as the parser does |
| Outline | The tree of blocks and inline pieces of one body, as the parser reads it, each with its position |
| Position | A byte offset into the body as written. Every block and every piece has a start and an end, the end exclusive |
| Content | A leaf block's text as the parser reads it: its lines with the markers and indentation of the blockquotes, list items and footnote definitions holding it removed; for a paragraph or heading, the leading spaces and tabs of each line removed; for a heading, its `#` markers and closing sequence, or its underline; and in a table cell, the backslash of each `\|`. Each line of content records where it starts in the file |
| Leaf block | A paragraph, a heading, a table cell, a thematic break, a code block, an HTML block or a link reference definition |
| Top level | A block held by no blockquote, list item or footnote definition |
| Comment block | An HTML block that holds nothing but HTML comments, spaces, tabs and line endings, and no comment left open in a block its end marker has closed. `<!-- c --> x` and `<!-- a --> <!-- b` are HTML blocks that are not one |
| Text | The text pieces of a leaf block, read together where they are adjacent on one line |
| Comment-masked text | Each line as written with every byte of an HTML comment in an HTML block replaced by a space. A comment within a paragraph shares it with text, so masking it would empty no section |
| ~~Masked text~~ | ~~Comment-masked text with every byte of a code span, its backticks included, replaced by a space too~~ No reader reads it once questions, entries and term definitions read the outline, so it is not made |
| Prose text | Each line as written with every byte replaced by a space but those of text that are prose, under the rules below |

### Components

| Component | Responsibility |
|---|---|
| `go.mod` | Requires `github.com/yuin/goldmark/v2` at a fixed version |
| `internal/markdown` | New. Parses a body with the parser and builds its outline: the blocks, the link reference and footnote definitions, and the inline pieces of each leaf block, with the positions the parser gives and those found from them. Knows nothing of documents, sections or rules, and imports goldmark and the standard library alone. No other package imports goldmark |
| `internal/repo` | Reads each document's outline once and answers every question about the body from it: headings, sections, anchors, explicit anchors, links, references, `[[...]]` links, comment-masked text and prose text, questions, `Changelog` entries, a term's definition, whether a block is left open, whether a title survives as a heading, and whether the body was read at all. `renumber`'s search for `[[...]]` links reads it too, and `renumber` refuses while any body is not read. The block walk, the maskers, the link scanner and the anchor renderer's own reading of links and code spans are removed |
| `internal/link` | `Resolve` reads `[[...]]` links and headings from the outline in place of its own pattern. Suggestions are offered within prose text, and `maskLinks` and its patterns are removed |
| `internal/lifecycle` | Reads sections, their emptiness and whether a block is left open from the outline, through `internal/repo`. `accept` and `reject` refuse a document whose body is not read |
| `internal/glossary` | `term rename` retitles a term's H1 written as either an ATX or a setext heading, and refuses a term whose body is not read. `term remove` refuses while any body is not read |
| `internal/lint` | L15, L20 and L21 read the outline. L23, new, reports a body that is not read, and L08, L09, L10, L14, L15, L16, L17, L20 and L21 pass over a document whose body is not read. L20 and L21 change as described under Questions and `Changelog` entries. In the rule registry, the summaries of L20 and L21, and a change recorded in the release this lands in for each rule shipped before it whose findings change under Backwards compatibility: L08, L09, L10, L14, L15, L16 and L17. L20, L21 and L22 are first shipped in v0.6.0, so where this lands before it they record no change |
| `internal/export` | Unchanged in shape. Answers, entries and links are what the outline gives. In `schema.json`, the descriptions of `contents` and of a `contents` entry's body, of a `Changelog` entry and of its `body`, of an answer's `body`, of where a `Rejection rationale` section's body begins, of a group's `description`, and of what a document whose body is not read exports; `schema/export/v3.json` is regenerated from it |
| `cmd/archdoc` | `new` asks `internal/repo` whether a title survives as a heading, as now, answered from the outline |
| `internal/template` | In `PROCESS.md`, the `Changelog` entry form and the list of what lint enforces, L23 added to it; in the authoring guide, the entry form and the sentence forbidding a heading, a fence or a comment in an answer's blockquote; in the triage guide, the L20 and L21 rows, a row for L23, and L23 among the rules softened on a frozen document; in `PROCESS.md`, what `accept`, `reject` and `renumber` refuse. The copies `archdoc update` writes, and every published copy of the guides, follow from these |
| `testdata/commonmark` | New. The reference cases, their recorded parses, and the known differences, which `.gitattributes` keeps as written, line endings included |
| Specification pages | `markdown.md`: a body is read as the parser reads it, the length past which it is not read, the corrections the outline makes to that reading, and the rules the tool adds to it. `lint.md`: L15, L20, L21 and L23. `links.md`: what suggestions are offered within. `export.md`: which headings `contents` holds, where an entry's body starts, what an answer's body holds, and what a document whose body is not read exports. `lifecycle.md`, `commands.md` and `links.md`: what `accept`, `reject`, `renumber`, `term rename` and `term remove` refuse. `architecture.md`: `internal/markdown` in the package tree, the paragraph on how a body is read, and goldmark among the dependencies, in place of the sentence that Markdown is handled with the standard library |
| Documentation | `docs/DECISIONS.md`: each entry under Markdown, Writers, and Links and the glossary, that the outline makes obsolete is struck, noting what replaced it, the entry that Markdown is handled with the standard library is struck, noting goldmark, and the outline's own entries are added. `docs/TESTING.md`: the reference cases, how they are recorded, and the known differences. `docs/EXPORT.md`: what `contents` and a `Changelog` entry hold, and what a document whose body is not read exports |

### The outline

The outline holds a block or piece for each node of the parser's tree, of the type the reference's parse gives the same construct.

| Block | Parser's node | Reference's node | Holds |
|---|---|---|---|
| Blockquote | `Blockquote` | `block_quote` | Blocks |
| List | `List`: bullet or ordered, with its start and delimiter, tight or loose | `list` | List items |
| List item | `ListItem` | `item`, or `tasklist` with `completed` where the reference reads a task | Blocks |
| Footnote definition | `FootnoteDefinition` | Written by the reference's XML as an element it cannot name; see Testing | Blocks |
| Paragraph | `Paragraph` | `paragraph` | Inline pieces |
| Heading | `Heading`, of level 1 to 6, ATX or setext | `heading` | Inline pieces |
| Thematic break | `ThematicBreak` | `thematic_break` | Nothing |
| Code block | `CodeBlock`, fenced with its info string, or indented | `code_block` | Its content, read as nothing else |
| HTML block | `HTMLBlock` | `html_block` | Its content as written, read as nothing else |
| Table | `Table`, a `TableHeader` and a `TableBody` of `TableRow`s, each holding `TableCell`s with their alignment | `table`, a `table_header` and `table_row`s, each holding `table_cell`s | Rows of cells, each cell holding inline pieces |
| Link reference definition | `LinkReferenceDefinition` | None: the reference consumes it and leaves nothing in its parse | Its label, destination and title |

| Piece | Parser's node | Reference's node | Holds |
|---|---|---|---|
| Text | `Text` | `text` | Its text, with backslash escapes and entities resolved |
| Line break | A `Text` ending in a soft or hard line break | `softbreak` or `linebreak` | Nothing |
| Code span | `CodeSpan` | `code` | Its content |
| Emphasis, strong emphasis, strikethrough | `Emphasis`, `Strong`, `Strikethrough` | `emph`, `strong`, `strikethrough` | Pieces |
| Link | `Link`, inline or by reference with its label, with its destination and title; or `AutoLink`, written within `<...>` or found by the linkify extension, with its destination | `link` | The pieces of its text |
| Image | `Image`, inline or by reference, with its destination and title | `image` | The pieces of its description |
| Raw HTML | `RawHTML` | `html_inline` | Its text as written |
| Footnote reference | `FootnoteReference`, with its label | Written by the reference's XML as an element it cannot name; see Testing | Its label |

| Rule | Detail |
|---|---|
| What is parsed | The parser reads a copy of the body in which each carriage return not followed by a line feed is a line feed, as the reference reads it. The copy is the body's length and differs from it only at those bytes, so every position in it is the same position in the body |
| Blocks | A container starts at its marker: a blockquote at its `>`, a list item at its list marker, and a table cell after the pipe opening it, or at the first byte of its row's content where none does. A leaf block starts at the first byte of its first line that is neither a container's marker nor indentation. A block ends after the last byte of its last line that is not blank, before the line ending, a fenced code block's closing fence, a setext heading's underline and an ATX heading's closing sequence included |
| Pieces | A piece starts and ends at the bytes it was read from, delimiters included: a code span's backticks, a link's brackets and parentheses, an autolink's `<` and `>` |
| Positions given | The start of each block and piece is the offset the parser gives its node, except as found below. The ranges of a block's lines, of a text, of a raw HTML piece, of a code span's content, of an autolink's text, of a link's or image's destination and title, of a reference link's label, and of a definition's label, destination and title are the ranges the parser gives their values. A piece's end follows its last child's, past its closing delimiters as written |
| Positions found | Where the parser gives a construct no position, or one that is not the construct's own, the outline finds it in the bytes as written, from the positions the parser does give. A code span, which the parser places at its content, starts at the backticks before the content and ends after the backticks closing it. Every code span in a table cell in which a backtick comes before an escaped `\|`, which the parser gives no position, whether or not the span holds the `\|`, is found among the cell's bytes. An autolink starts and ends where the range of its text does, as the parser can place an address the linkify extension finds at the byte before it, after a space or a `(`. A fenced code block's closing fence and a setext heading's underline, which the parser leaves out of the block's content and lines, are the line after the last line of content, or after the opening fence where there is none, where that line closes the block; an ATX heading's closing sequence, and the `#` markers of a heading with no text, are found on its line. A thematic break ends at the end of its line, and a link reference definition after its title, or its destination where it has none. An empty destination written `()`, which the parser gives no range, starts and ends after the `(`, and an empty title written `""`, `''` or `()`, which it gives none either, after its first byte. A setext heading whose line after its content is no underline, which the parser gives where a table has taken the heading's last line as its header, is the paragraph it was, and the line the parser takes as its underline lies in no block. The extent of a hard line break, which the parser records only as a flag on the text before it, is the bytes from that text's end to the line ending, and the empty text the parser gives before a hard line break that opens a paragraph, with no range, starts and ends at its backslash. Whether a fenced code block or an HTML block is closed, which the parser does not record, is read from the line after its last for a fence, and from its last line for an HTML block that only its end marker closes |
| Parts | A link and an image also hold the positions of their text, destination and title, and a link reference definition those of its label, destination and title. A destination's position is that of the destination as written, within any `<` and `>`, as now, so a writer replaces the destination alone, and its bytes, with escapes and entities resolved, are the destination the parser gives. An empty destination starts and ends where it would begin. An autolink's destination is the address as written, without its `<` and `>` and without any scheme the parser adds to it. A reference link has no destination of its own: its definition's is the one written |
| Footnote definitions | The parser keeps each footnote definition where it stands, one that nothing references included, and the outline keeps them as it does. The reference's parse moves the definitions that are referenced to its end and leaves out the rest; see Testing |
| Content | A byte of content maps to the byte of the file it was read from. A column of a tab that a container's marker covers only in part is a space in content, mapped to the tab |
| Line | A block or piece is on the line holding its start. A link, an image and a link reference definition are on the line holding the start of the destination. A reference link is on the line holding its label |
| Order | A block's children are in document order and do not overlap, and every piece lies within its block's content |

### Reading

1. The front matter is split off as now, and the body divided into lines.
2. The parser reads the body, its lone carriage returns read as line feeds.
3. The outline is built from the parser's tree: each node's position as given, or found where the parser gives none, and the parts of each link, image and definition.
4. Labels are matched as the parser matches them: under Unicode's full case folding, each run of whitespace one space, the first definition of a label being the one that applies.
5. `[[...]]` links are read under the rule below.

Every reading of a body takes this one outline. No reader reads the body's bytes for a construct the outline holds.

### What reads the outline

| Reader | Reads |
|---|---|
| Headings | Every heading, ATX or setext, wherever it stands. A heading's text is its content, the lines of a setext heading joined by a line feed as the parser joins them, with every HTML comment in it replaced by spaces and its ends trimmed, as now |
| Sections | Top-level headings. A heading inside a blockquote, a list item or a footnote definition bounds no section and opens no entry of the export's `contents` |
| A heading's body | The lines after the heading's last line, its underline for a setext heading, to the heading that ends it. A section's text, a `contents` entry's body, a term's definition after its H1 and a group's description are each read from there, where today each starts on the line after the heading's first |
| The H1 | A document's H1, which L10 reads, `renumber` and `term rename` retitle, and a term's definition and a group's description follow, is its first top-level heading, where that heading is of level one, as today it is its first heading where that is of level one |
| Heading anchors | Every heading, in the order the reference's parse holds them, so a heading inside a container has an anchor and takes a suffix where its anchor is already taken: the headings of referenced footnote definitions after every other heading, in the order the definitions are first referenced, and no heading of a definition that nothing references, which has no anchor. Explicit anchors in a definition that nothing references are not read either. The anchor's rendered text is made from the heading's pieces under the rules `markdown.md` gives under Anchors, and the slug and its suffixes are computed as now |
| Explicit anchors | The `<a>` start tags carrying `id` or `name` in raw HTML pieces and HTML blocks. A comment within an HTML block is removed before an anchor is read from it, as now |
| Links | Inline links and images, and link reference definitions. A reference link is listed among the document's references, as now |
| `[[...]]` links | Text, under the rule below |
| Suggestions | Prose text |
| Section emptiness | Comment-masked text, as now |
| Questions | The top-level blocks of each question section |
| `Changelog` entries | The top-level blocks of each `Changelog` section |
| A term's definition | The top-level blocks after its H1 |
| A block left open | A top-level fenced code block, comment block, or HTML block that only its end marker closes, still open when the body ends. One inside a container is not: the blank line and unindented heading `reject` appends end the container. `reject` refuses such a document, as now |

A `[[...]]` link is read from the bytes of the file that text spans, as written: `[[`, a name holding no `[` or `]`, and `]]`, on one line, all within text that is outside the text of a link or an image. An escaped bracket is not text as written, so `\[[a]]` is no `[[...]]` link. Where the document defines the label inside, as in `[[RFC-0007]]` beside `[RFC-0007]: path`, the parser reads a link within brackets, and there is no `[[...]]` link. A `[[...]]` link in a heading is read, and L16 reports it, as now.

Text is prose where it is not in a heading, not within the text of a link or an image, not between an `<a` start tag and its `</a>` in the same leaf block, not part of an address with a scheme or beginning `www.`, from its start to the next space or line ending, and not between a `[` and a `]` on its line. `link --suggest` and `link --apply` offer and apply a suggestion only where its whole match lies within prose. A line holding raw HTML or a bracket left unmatched is no longer withheld as a whole: only the bytes outside prose are.

### Questions

This amends [RFC-0008](0008-questions.md). A question section, a question and its detail are as that design has them, with headings read from the outline: a question is a top-level level three heading, and its detail runs to the next top-level heading. The blockquotes it reads change:

| Rule | Detail |
|---|---|
| Blockquote | A top-level blockquote in a question section, as the parser reads it: its `>` indented by up to three spaces, its lazy continuation lines included, holding any blocks |
| Ends with a blockquote | A question ends with a blockquote when its last top-level block before the next top-level heading, passing over comment blocks and link reference definitions, is a blockquote |
| Answer | The blockquote a question in `Resolved questions` ends with. Its body is its lines with any indentation before the `>`, the `>`, and one column after it removed, a tab after the marker leaving as spaces the columns it spans past the one taken, and a lazy line as written |
| A heading in a blockquote | Part of the blockquote. It is not a question and does not end the question holding it |
| Anchors | A question's anchor is compared with every heading's, a heading inside a container included |

L20 no longer reports a heading, a fenced code block or an HTML comment in a blockquote of a question section. Its other findings stand, read through the rules above.

### `Changelog` entries

This amends [RFC-0009](0009-changelog.md). A `Changelog` section runs from its level two heading to the next top-level heading of level one or two. Entries are read from it as follows:

| Rule | Detail |
|---|---|
| Entry | An item of a top-level list in the section, whatever its marker: `-`, `*`, `+`, or a number followed by `.` or `)`. A section may hold more than one list, and its entries are the items of each, in document order |
| Nested item | A list item within an entry is text of that entry, as now |
| Date | Read from the start of the entry's text, where the item opens with a paragraph, by the rules [RFC-0009](0009-changelog.md) gives. An entry opening with any other block, five or more columns past its marker making an indented code block among them, is undated |
| Text | The entry's lines as written, with the item's marker and the spaces after it removed from the first, the item's content indentation removed from each later line that has it, a lazy line as written, the date with its `:` and one space removed from the first line, and blank lines at either end dropped |
| Passed over | Comment blocks and link reference definitions at the top level of the section |

| Finding | Line |
|---|---|
| A top-level block in the section that is not a list, a comment block or a link reference definition | The block's first line |

That finding replaces [RFC-0009](0009-changelog.md)'s finding of lines belonging to no entry. Its finding of a heading opening or ending the section that is indented by two or three spaces, or is a heading only once comments are masked, is removed: the section is bounded by top-level headings as the parser reads them. Its findings on dates, their order, a second `Changelog` section, and a heading differing from `Changelog` only in case, spacing or punctuation stand.

`PROCESS.md` gives an entry as a dated list item, beginning with its date in the form `YYYY-MM-DD: `, and the template's comment shows `- ` as before.

### Term definitions

A term's definition is the top-level blocks after its H1, subsections included. L15 wants exactly one paragraph, passing over comment blocks and link reference definitions. Its findings keep their form.

### Masked texts and prose text

Comment-masked text and prose text are derived from the outline and hold to [ADR-0007](../adr/0007-masking-never-moves-a-byte.md): each line is the same length as written and differs from it only where a byte became a space. The lines of code blocks and HTML blocks are not prose lines, as now.

### Time

Building the outline from the parser's tree, and every reading of the outline, takes time linear in the body's length. The parser's own time is goldmark's. Each of the 22 shapes [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md) times, and each shape the current readers' timing tests hold, is a timing test of reading a body. A timing test of a shape goldmark reads in more than linear time is skipped, naming the shape, until goldmark reads it in linear time. At v2.1.16 those are three of the shapes [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md) gives times for, list items nested a level deeper on each line, backtick runs of every length, and link reference definitions; one the current readers' timing tests hold, brackets left open before a run of links; and a bracketed text closed on each line of a paragraph. Building the outline is timed on every shape, those included, and fails where its time grows more than twice as fast as the body's length.

A body longer than 131,072 bytes (128 KiB) is not given to the parser. Its outline holds the document node alone, so every reader finds nothing in it: no heading, section, anchor, link, question, entry or definition. L23 reports such a document at the first line of its body, an error or, in a frozen document, a warning under [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md). The rules that read a body, L08, L09, L10, L14, L15, L16, L17, L20 and L21, pass over a document whose body is not read, and L17 checks a link into one without its fragment, as no anchor in it is known. The rules that read front matter, and L11, read it as any other. `accept`, which reads its open questions, `reject`, which appends a section after the body, and `term rename`, which retitles its H1, refuse a document whose body is not read; `renumber` and `term remove`, which read the links of every document, refuse while any body is not read. `link` finds nothing to change in it, and `propose` and `withdraw`, which change front matter alone, act on it as on any other. `export` and `index` read such a body as holding nothing, and the export schema's descriptions say so. The limit is fixed. At v2.1.16 goldmark reads 128 KiB of the slowest of the five shapes in 0.55 seconds and 256 KiB in 2.1, and at [f9a2f3f] the longest body in this repository's `spec/`, this RFC's, is 49,131 bytes.

### Testing

The outline is compared with the reference's own parse of recorded cases.

```
testdata/commonmark/
  REFERENCE                     cmark-gfm 0.29.0.gfm.13 table strikethrough autolink tasklist footnotes
  ATTRIBUTION.md                the licence and source of the spec, extension and regression examples
  KNOWN                         each known difference: its case, the first node that differs, and why
  spec/0001.md                  an example's input
  spec/0001.xml                 the reference's parse of it
  extensions/0001.md
  extensions/0001.xml
  regression/0001.md
  regression/0001.xml
  archdoc/table-cell-past-header.md
  archdoc/table-cell-past-header.xml
```

```xml
<table sourcepos="1:1-3:20">
  <table_header sourcepos="1:1-1:5">
    <table_cell sourcepos="1:2-1:4">
      <text sourcepos="1:3-1:3" xml:space="preserve">a</text>
    </table_cell>
  </table_header>
  <table_row sourcepos="3:1-3:20">
    <table_cell sourcepos="3:2-3:4">
      <text sourcepos="3:3-3:3" xml:space="preserve">b</text>
    </table_cell>
  </table_row>
</table>
```

1. The cases are the 672 examples of the GFM specification 0.29, the 30 of the reference's extension tests and the 26 of its regression tests, as the reference's repository holds them at its tag, the first two published under CC-BY-SA 4.0 and the third under the licence of the repository's `COPYING`, attributed in `ATTRIBUTION.md`, each with the `→` the specification writes for a tab turned into one; and the tool's own cases, under `archdoc/`, holding every reading the current readers' tests assert that is still true of the reference, each misreading found later, and each difference between the parser and the reference found later.
2. Each case's parse is recorded beside it, as `cmark-gfm -t xml --sourcepos` with the five extensions writes it.
3. A test reads each case into an outline and compares it with the recorded parse, read with `encoding/xml`: the same tree of blocks and pieces, passing over the outline's link reference definitions, which the parse does not hold, the same types, the attributes the parse carries, such as a heading's level, a list's type, start, delimiter and tightness, a code block's info string, a header cell's `align`, and a link's destination and title, and the same text. A `tasklist` element is compared as a list item whose content begins with its box as written, `[ ]` for one not completed and `[x]` or `[X]` for one completed.
4. The reference's XML writes a footnote reference and a footnote definition as `<unknown>` tags that are not well-formed XML, with no label, which `encoding/xml` does not accept, and it moves each definition to the end of the document, in the order first referenced, dropping any that nothing references. The comparison reads the empty element as a footnote reference and the other as a footnote definition, and compares the outline's referenced definitions with them, in the order first referenced. Footnote labels, and the place of a definition, are checked by hand-written cases.
5. A case listed in `KNOWN` is expected to differ at the node it names. The test fails where it differs elsewhere, and where it no longer differs, so that the entry is removed when a release of goldmark reads the case as the reference does. `KNOWN` begins with the differences found when the parser was chosen: among the examples, a `<meta>` tag interrupting a paragraph, as goldmark counts `meta` among the tags that open an HTML block of the sixth type and CommonMark does not, a closing `</script>` tag alone on a line, a closing `</pre>` tag at the start of a line, a one-column table whose header row holds no text, emphasis or strikethrough inside a footnote label, strikethrough of unequal runs, which addresses the `autolink` extension finds and where they end, and the indentation of a footnote definition's first line; and the cases the tool adds for the differences found among the generated bodies and by the fuzz test below: where an address written without `<` and `>` is found and where it ends, among them one after a `]` or a `)` with a link after it, the beginning and end of a table, in a list item, a blockquote or a footnote definition too, `<!` and a lowercase letter, which goldmark reads as a declaration, opening an HTML block of the fourth type at the start of a line and raw HTML within one, as CommonMark 0.31.2 does, where GFM 0.29 and the reference read a declaration only before an uppercase letter, `<?>`, which goldmark reads as a whole processing instruction where the reference reads on to the next `?>`, whether a list is tight, where a list item ends after an empty item nested in it, emphasis and strikethrough beside a lone or escaped `~`, emphasis beside a currency symbol, which goldmark reads as punctuation, as CommonMark 0.31.2 does, a `-` or `---` after a link reference definition, the leading spaces of a paragraph after one in a blockquote, which the reference keeps and commonmark.js does not, a footnote reference followed by `()`, or after `!` by a destination, `!` and any character before `^1]`, which goldmark reads as a footnote reference, footnote labels, which goldmark matches only where their case agrees and reads where they hold a space, an escaped `]`, or a `]` after an escaped backslash, the space after a footnote definition's colon before an HTML block, the leading space of a lazy line within a code span, which the reference keeps and goldmark and commonmark.js remove, a space written as an entity at the end of a fence's info string, which the reference drops, a fence's info string of one character where the body ends on its line, a line of spaces in an indented fence, a tab after a container's marker or indenting a line inside a container, a title written directly after a destination within `<` and `>`, a label longer than the 999 characters CommonMark allows, the backslash of an escaped backslash before a pipe in a table cell, and a line that would open an HTML block of the seventh type after a quoted or listed paragraph, at which the reference ends the paragraph where commonmark.js does not.
6. Positions are not compared with the reference's, as the reference's are not exact: in 0.29.0.gfm.13, among others, a code span is placed at its content without its backticks, an address the `autolink` extension finds starts a column early and an email address it finds has none, an HTML block that its end marker closes ends before it starts where it is one line, and a line early where it is longer, every line of a paragraph is placed from the first line's indentation, and pieces after a link reference definition are placed on the wrong line. Positions are checked by the rules under The outline, which a test asserts of every case: every block and piece starts at the first byte of its construct, every piece lies within its block, siblings are in order and do not overlap, a text piece's bytes in the file are its text where it holds no escape or entity, and a destination's bytes, with escapes and entities resolved, are its destination. The offset invariant of [ADR-0007](../adr/0007-masking-never-moves-a-byte.md) is fuzzed as now.
7. ATX and setext headings, and the forms of a link, are not attributes of the parse, and are checked by hand-written cases.
8. `go test ./internal/markdown -update-commonmark` records every parse again. It fails where `cmark-gfm` is not on the path, or reports a version other than the one `REFERENCE` names.
9. `FuzzOutlineMatchesTheReference` compares the outline with the reference's parse of generated input where `cmark-gfm` of the version `REFERENCE` names is on the path, and checks the position rules on every input whether or not it is. Its input is valid UTF-8 holding no control character but a tab, a line feed or a carriage return, as the reference writes others into its XML unescaped. A difference it finds becomes a case, and a known difference where the parser is not corrected.
10. A body of 131,072 bytes is read and one of 131,073 is not, and `testdata/lint` holds a document whose body is longer, which L23 reports.

### Delivery

The current readers are replaced in five stages. Each leaves `go test ./...`, `archdoc lint` and `archdoc index --check` passing, and `markdown.md` and `architecture.md` describing the code as the stage leaves it.

1. goldmark in `go.mod`, `internal/markdown`, and the reference cases with their known differences, read by nothing else.
2. Headings, sections and heading anchors read the outline: setext headings are read, a heading inside a container has an anchor, only top-level headings bound sections, and the anchor renderer's own reading of links and code spans is removed. A body longer than the limit is not read, by the outline or by any current reader not yet replaced, and L23, the rules passing it over and the commands refusing it follow. L21 stops reporting a heading opening or ending its section for its indentation or for a comment on its line, as neither line is any longer read as the section's heading.
3. Links, references, `[[...]]` links, explicit anchors, comment-masked text and prose text read the outline, and suggestions read prose text. The link scanner, the inline maskers and `maskLinks` are removed. ~~Masked text, which only the readers stage 4 replaces read, is made with them.~~ Those readers read the outline's blocks, so masked text is not made.
4. Questions, `Changelog` entries and term definitions read the outline, and L15, L20 and L21 change as described above, with `PROCESS.md`, the guides and the export schema's descriptions.
5. The block walk, and whatever else remains of the current readers, is removed.

### Errors

| Condition | Result |
|---|---|
| A top-level block in a `Changelog` section that is not a list, a comment block or a link reference definition | L21, at the block's first line |
| A body longer than 131,072 bytes | L23, at the first line of the body |
| `-update-commonmark` where `cmark-gfm` is not on the path | The test fails, naming the command it needs |
| `-update-commonmark` where `cmark-gfm` reports another version | The test fails, naming both versions |
| A case whose outline differs from its recorded parse, other than at the node `KNOWN` names for it | The test fails, naming the case and the first node that differs |
| A case listed in `KNOWN` whose outline no longer differs from its recorded parse | The test fails, naming the case, until its entry is removed |

### Out of scope

- **What GitHub's sanitiser does after parsing.** The reference's output can hold a comment its parse leaves open, which GitHub's sanitiser reads as hiding the rest of the page. That is GitHub's handling of HTML, not the reference's parse, and stays a separate question.
- **Writing from the outline.** Writers edit the file as written at an offset, under [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md). Nothing is rendered or serialised from the outline.

## Alternatives considered

The design rests on [ADR-0007](../adr/0007-masking-never-moves-a-byte.md), which the masked texts keep to; on [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md) and [ADR-0023](../adr/0023-every-finding-no-permitted-edit-can-clear-is-a-warning.md), under which a finding the outline adds to a frozen document is a warning; on [ADR-0019](../adr/0019-export-stability.md), under which the export keeps `schema: 3`; and on [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md), which makes goldmark the parser and cmark-gfm's parse the reading, and records the parsers weighed against goldmark and the reading of inline text alone with the block walk kept.

- **Masked text as the reader's only output.** One or two masked copies of each line, more accurate than today's, with every reader matching patterns against them as now. Raised in a working session, 2026-11-12, not publicly available and first written down on 2026-11-13. Lost because a masked line cannot say that it belongs to a list item or a blockquote, which the `Changelog` and question readers need.
- **goldmark's task list extension, beside the reference's `tasklist`.** Weighed in a working session, 2026-11-14, not publicly available and first written down on 2026-11-14. Lost because it reads `- [x](a.md)` as a completed task and loses the link, which the reference reads, while the tool reads nothing from a task's box.
- **Lone carriage returns passed to the parser as written.** Weighed in a working session, 2026-11-14, not publicly available and first written down on 2026-11-14. Lost because goldmark reads the text after one as the same line, so `# A`, a lone carriage return, `## B` is one heading where the reference reads two.
- **Keeping `-` as the only `Changelog` marker, as [RFC-0009](0009-changelog.md) has it.** Weighed in a working session, 2026-11-16, not publicly available and first written down on 2026-11-16. Lost because a list item written with another marker renders as one entry of the list, and the restriction existed so that the subset could be read.
- **Hand-written expected readings alone.** Each test stating which links, headings and blocks a body holds, as the current tests do. Weighed in a working session, 2026-11-13, not publicly available and first written down on 2026-11-13. Lost because such tests hold only the cases someone thought of: six rounds of changes to comment, code span, raw HTML, autolink and anchor reading, each reviewed against GitHub's reading, each found cases the tests did not hold.
- **GitHub's API as the reference.** Comparing with `gh api markdown`. Weighed in a working session, 2026-11-13, not publicly available and first written down on 2026-11-13. Lost because every test run would need the network and credentials, and its output is HTML after the sanitiser rather than the parse [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md) names.
- **Comparing the reference's positions.** Weighed in a working session, 2026-11-16, not publicly available and first written down on 2026-11-16. Lost because they are not exact, as listed under Testing.
- **One change in place of stages.** Weighed in a working session, 2026-11-13, not publicly available and first written down on 2026-11-13. Lost because a regression in a change replacing every reader at once could come from any of them, where a stage changes what lint reports in one area.
- **Correcting `<!` before a lowercase letter to the reference's reading.** Weighed in a working session, 2026-11-17, not publicly available and first written down on 2026-11-17. Lost because the shape is rare in a specification document, so the outline reads it as goldmark does.

No other alternatives were weighed.

## Backwards compatibility

In this repository's `spec/`, 55 documents, and in `testdata/repo`, 18, terms and groups aside, the reference reads the same set of relative link destinations in each document as the tool at [4cae902], and neither holds a setext heading or a heading inside a blockquote or list item. Each answer there is a blockquote the reference reads over the same lines, and each `Changelog` entry starts a list item the reference reads. goldmark reads the body of each of the 93 Markdown files under the two directories at [4cae902] as the reference does, compared by the HTML each renders.

In a repository that upgrades:

- Wherever the tool's reading of a link differs from the parser's, L17 checks the link the parser reads and stops checking the one it does not, and `export`, `renumber`, `term remove` and `link` act on the same reading. Where the parser's reading differs from the reference's, as for the known differences, the tool reads the parser's. `<!` followed by a lowercase letter, as in `<!doctype html>`, is a declaration, where GitHub and the tool today read it as text: at the start of a line it opens an HTML block, ending the paragraph before it, and within a line it is raw HTML up to the next `>`. A link it divides or holds, as in `[a`, `<!x>`, `](n.md)` and in `a <!x [b](c.md) >`, is no longer read, and L17 no longer checks it.
- A line of text followed by a line of `=` or `-` is a heading. Where the underline is `-` and the author meant a thematic break, the document now has a heading where GitHub renders one, with an anchor, and a section where the heading is at the top level, which L08 reads like any other: a setext `Proposal` satisfies it. The heading's body starts after its underline, and a link suggestion is no longer offered in its text.
- A line that is a heading only once a comment on it is masked, such as `##<!-- c --> Proposal`, is a paragraph, as the parser reads it. It no longer opens a section or satisfies L08, and its anchor goes.
- A heading inside a blockquote, and a heading on a list item's own line, such as `- ## Marker`, have an anchor, and a later heading with the same anchor takes the next suffix, as on GitHub. A link written to the suffix the tool gave before points to the heading GitHub gives it. A heading within a list item, indented by up to three spaces, which has an anchor today, keeps it, but no longer bounds a section, opens an entry of `contents`, is a question in a question section, or is taken as a document's first heading by L10, `renumber`, `term rename`, a term's definition or a group's description. A link suggestion is no longer offered in a heading inside a blockquote or on a list item's own line, as one is today; a heading within a list item gets none today either.
- A heading's anchor is made from its pieces as the parser reads them, so a heading holding a link, a code span or a comment that the anchor renderer reads differently today has the anchor GitHub gives it.
- A table cell past the header row's count is not read.
- `link --suggest` offers terms in text it withholds today: text sharing a line with raw HTML outside an `<a>` element, and text on a line whose brackets are unbalanced, outside the brackets.
- L20 no longer reports a heading, a fence or a comment in a blockquote of a question section. A blockquote indented by one to three spaces, and a lazy line after a blockquote, are read as the parser reads them: a resolved question ending in one gains its answer, in the export too, where L20 reports today that it does not end with a blockquote, and an open question ending in one is reported. A resolved question whose blockquote is followed by a top-level link reference definition, reported today as not ending with a blockquote, gains its answer. A question sharing an anchor with a heading inside a container is reported.
- L21 reads entries written with `*`, `+` or a number, which are reported today and left out of `changelog`, and publishes them, in frozen documents too. An entry whose marker is indented by one to three spaces, or followed by a tab, is reported today and is an entry. A continuation line indented with a tab is part of the entry rather than reported. A line at column 0 the parser reads as a lazy line of the entry is part of the entry's text: `[x]: y.md` directly after an entry, reported today as belonging to no entry and not read as a definition, is part of the entry's text and still not a definition. A top-level link reference definition in the section, reported today, is passed over. A lone carriage return ends a line, so `- 2026-11-14: A`, a lone carriage return, `- 2026-11-14: B` is two entries where today it is one. An indented heading opening or ending the section is no longer reported.
- L15 no longer counts a link reference definition in a term's definition as a second paragraph, which it reports today.
- In a frozen document, L08, L09, L10, L14, L16, L17, L20 and L21 warnings can appear or disappear under these readings: L10, for example, reads a setext H1 and no longer reads `#<!-- c --> Title`, and L16 no longer reads `\[[a]]`, which it reports today. Where the last warning of a rule disappears from a document acknowledged for that rule, L22 reports the acknowledgement, and a new warning counts under `--strict-warnings`.
- A body longer than 131,072 bytes, read today, is not read: L23 reports it, the rules that read a body pass it over, `accept`, `reject` and `term rename` refuse it, `renumber` and `term remove` refuse to run, and its export holds nothing read from the body. A body within the limit holding one of the five shapes goldmark reads in more than linear time takes that time to read, up to 0.55 seconds.
- A guide under `docs/guides` is read as a page, under [RFC-0016](0016-a-release-carries-its-documentation-and-an-install-script.md), so its links are the parser's too, and the release build refuses a guide longer than 131,072 bytes, whose links it cannot check.
- A term's H1 is the document's, its first top-level heading where that is of level one, as for every other type, where today it is the first level one heading anywhere in the file. A term opening with a heading of another level, which L10 reports today, has no H1, so no definition: L15 reports no paragraph, the definition is empty in `GLOSSARY.md`, `term show` and the export, and `term rename` refuses it.
- `export` keeps `schema: 3`, which no tagged release has emitted, as v0.5.1 emits `schema: 2`; under [ADR-0019](../adr/0019-export-stability.md) every change between two tagged releases shares one number, so this holds while the change lands before the next tag. No field is added or removed; the values above change, and the schema's descriptions named under Components are corrected.

The tests asserting what changes are rewritten in the stage that changes it, among them: the L20 tests on headings, fences and comments in a blockquote, in `internal/repo/questions_test.go`; the L21 tests on indented and comment-masked headings and on lines outside the subset, in `internal/repo/changelog_test.go` and `internal/lint/lint_test.go`; the `TermEntry` cases on setext underlines and on a comment that is a heading only once masked, in `internal/repo/repo_test.go`; and the suggestion tests on raw HTML and bracketed text, in `internal/link`. The L21 case in `testdata/lint`, an entry written with `*`, becomes an entry, so the fixture's L21 case moves to a paragraph in the section.

[4cae902]: https://github.com/archdochq/archdoc/commit/4cae90263e7b6a4b9a54120f87e032381558b50b
[f9a2f3f]: https://github.com/archdochq/archdoc/commit/f9a2f3f776622f1d57141485b6c887e40d088beb

## Resolved questions

### How is a body that goldmark reads in more than linear time bounded?

goldmark v2.1.16 reads three of the shapes [ADR-0026](../adr/0026-documents-are-read-with-goldmark.md) lists, one the current readers' timing tests hold, and a bracketed text closed on each line of a paragraph, in more than linear time, the longest measured, 800 KB of the last, in 19.3 seconds, where the current readers were made linear in each such shape as it was found, one of them still open. The bound could be a limit on a body's length, past which the tool reports the document rather than reading it; a limit on the time spent reading one body; a correction to goldmark made upstream before the reader is released; or none, until goldmark reads the shapes in linear time.

> By a limit on a body's length: a body longer than 131,072 bytes is not read, and L23 reports it, as Time describes. A limit on time would make whether a document is read depend on the machine reading it, a correction upstream would leave the release waiting on goldmark, and no bound leaves a body read in 19.3 seconds.

## Changelog

- 2026-11-17: Add `<!` followed by a lowercase letter, and `<?>`, to the differences `KNOWN` begins with. Record under Backwards compatibility the links the first hides, and under Alternatives the correction weighed for it.
- 2026-11-17: Replace a footnote definition inside a container, which goldmark and the reference read alike, among the differences `KNOWN` begins with, by a table begun inside a footnote definition, which the generated bodies that differ there hold, and add the differences found when the outline was first compared with the reference and with the bodies the current readers' tests read.
- 2026-11-17: Add brackets left open before a run of links, a shape the current readers' timing tests hold, to the shapes goldmark reads in more than linear time.
- 2026-11-17: Add to the differences `KNOWN` begins with `---` after a link reference definition, a footnote reference after `!` followed by a destination, footnote labels whose case differs or holding a space or an escaped `]`, and the space after a footnote definition's colon before an HTML block.
- 2026-11-18: Find an empty title's position, and read a setext heading whose underline follows a table as the paragraph it was. Add a bracketed text closed on each line of a paragraph to the shapes goldmark reads in more than linear time, and time building the outline on every shape. Add to the differences `KNOWN` begins with a footnote label holding a `]` after an escaped backslash, the leading space of a lazy line within a code span, and a space written as an entity at the end of a fence's info string.
- 2026-11-18: Resolve how a body goldmark reads in more than linear time is bounded: a body longer than 131,072 bytes is not read, L23, new, reports it, the rules, commands and export that read a body treat it as holding nothing, and the commands that need a body refuse it. Correct the claim that writers refuse a document holding a block left open today, which only `reject` does.
- 2026-11-18: Add emphasis beside a currency symbol to the differences `KNOWN` begins with, and deliver in stage 2 the end of L21's reports on a heading indented or masked into opening or ending its section.
- 2026-11-18: Make masked text in stage 4 with the readers that read it, rather than in stage 3.
- 2026-11-18: Count an address beginning `www.` with one carrying a scheme as not prose, as goldmark leaves some that cmark-gfm finds unlinked.
- 2026-11-18: Make no masked text, which no reader reads once questions, entries and term definitions read the outline's blocks.
- 2026-11-18: Correct, from an audit before acceptance, what the design says of comment blocks, comment-masked text, a block left open, an entry's date, the fuzz test and the known differences, name the remaining documents and files it changes, and add the guides, which [RFC-0016](0016-a-release-carries-its-documentation-and-an-install-script.md) reads as pages, to what it updates.
