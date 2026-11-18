---
id: ADR-0026
title: Documents are read with goldmark
status: accepted
created: 2026-11-13
decided: 2026-11-18
depends: [ADR-0005, ADR-0007, ADR-0025]
updates: [ADR-0005, ADR-0025]
obsoletes: [ADR-0006]
---

# ADR-0026: Documents are read with goldmark

## Context

Under [ADR-0006](0006-markdown-is-handled-with-the-standard-library.md) the tool recognises the constructs it needs with code written here and treats everything else as text, and a full parser is refused because it would have to round-trip everything it parsed. The set has grown one construct at a time, each read by code of its own: a block walk that blanks comments and code spans into a masked copy of each line, a link scanner over that copy, a second inline reader that renders heading text for anchors, a pattern-based masker for link suggestions, and separate readers for `Changelog` entries, the answers of resolved questions and the paragraphs of a term's definition, each of which reports what falls outside the shapes it reads. Masked text keeps every byte where it was written, under [ADR-0007](0007-masking-never-moves-a-byte.md).

GitHub supports GFM for user content on GitHub.com, as the [GFM specification][gfm] states, and [cmark-gfm][cmark-gfm] is the CommonMark reference implementation extended with the extensions that specification defines. CommonMark settles which construct claims a run of text by the order it reads in. Block structure is read first. Within a block's text, code spans, autolinks and raw HTML are claimed as they are met, from left to right, and a link's destination and title are claimed when its closing bracket is reached, before anything inside them is read as raw HTML. Blanking one construct and matching patterns against what remains reproduces that order in neither direction:

| Body | cmark-gfm | Reading |
|---|---|---|
| `<span title="[x](n2.md)">a</span>` | raw HTML; no link | The tool at [4cae902], which reads links before raw HTML, links `n2.md` |
| `[x](n9.md "<?") [x](n1.md) ?>` | links to `n9.md` and `n1.md` | A reader that finds raw HTML before links reads the `<?` in the title as opening HTML that runs to the `?>`, and loses `n1.md` |

The second reading is that of a branch written for the open task of [#69] to decide whether a `<!--` is inside a code span begun on an earlier line of its paragraph, where the tool at [4cae902] reads it as opening a comment. Working sessions, 2026-11-10 and 2026-11-11, not publicly available and first written down on 2026-11-13, took that branch through six rounds of changes, to comment, code span, raw HTML, autolink and anchor reading, each reviewed against GitHub's reading and from the second on against cmark-gfm's. Each review found new misreadings beside the changes before it, and the branch was stopped on 2026-11-11, unpublished, reading link titles as above where the code it was to replace reads them as cmark-gfm does.

The readers also disagree with one another, and with GitHub where one reads a shape the rest do not. A line that is a heading only once its comment is blanked opens a section but is text to the link scanner. The block walk follows blockquotes and list items and reads the blocks inside them, while a heading inside a blockquote, or on a list item's own line as in `- ## Heading`, has no anchor, a setext heading is not read, and a fence inside an answer's blockquote is reported rather than read. Several shapes have also been read in time quadratic in their length, each found and bounded on its own: [#67], [#70] and [#79], and [#84], which is still open.

The tool needs from a parser the reading cmark-gfm gives, the position in the file of each construct it reads, since a finding names a line and column and a writer edits at an offset, and a single binary built without cgo, as the release build is. Three parsers were measured against these in a working session, 2026-11-14, not publicly available and first written down on 2026-11-14, with the document bodies and cmark-gfm's times measured again on 2026-11-16: [goldmark][goldmark], a CommonMark parser written in Go with GFM's extensions, under the MIT licence and importing nothing outside the standard library, at v1.8.7 and at v2.1.16, each read with its table, strikethrough, linkify and footnote extensions; and cmark-gfm 0.29.0.gfm.13 with its table, strikethrough, autolink, task list and footnote extensions. goldmark v2 is a separate module, `github.com/yuin/goldmark/v2`, released at v2.0.0 on 2026-08-27 and at 19 later versions since, which its README describes as early in its release. Its nodes hold their start in the source and its values their ranges, among them the text of a link's destination, title and label, and of a definition's. The bodies were the 93 Markdown files under `spec/` and `testdata/repo` at [4cae902], without their front matter; ten bodies written for cases found while the branch above was worked on; the 728 examples of the GFM specification 0.29 and of cmark-gfm's extension and regression tests; and 10,000 bodies generated at random from brackets, backticks, comment and HTML delimiters, table rows, list and blockquote markers, headings, fences, emphasis, entities, images, addresses, link reference and footnote definitions, footnote references and `[[...]]` links.

Where goldmark v2's reading differs from cmark-gfm's, compared by the HTML each renders, in the destinations of links and images other than addresses, a raw HTML `<a href>` counted as a link; in the level and text of headings; and in the Markdown block elements, an HTML block not counted as one:

| Bodies | Links | Headings | Block structure |
|---|---|---|---|
| Document bodies and cases found on the branch, 103 | 0 | 0 | 0 |
| Specification and test examples, 728 | 0 | 0 | 3 |
| Generated, 10,000 | 73 | 36 | 348 |

Of the 73 generated bodies whose links differ, 46 differ in where an address is found. GFM finds an address written without `<` and `>` only at the start of a line, after whitespace, or after `*`, `_`, `~` or `(`. In 29 cmark-gfm's `autolink` extension finds one goldmark does not, or ends one later: in 26 elsewhere, as `http://x.io/p` after the `]` in `a]http://x.io/p[x](a.md)`, in one after a tab, where goldmark finds none, and in two past a `\`, as in `(www.y.io\![i](p.png)`, where goldmark ends it at the `\`. In 17 goldmark's linkify extension does, as `www.y.io` after the `)` in `[x](a.md)www.y.io[x](a.md)`; in each, the parser finding the address reads the link after it as part of the address. The other 27 include footnote definitions, which the two end at different lines; tables; and `<?>`, which goldmark reads as a whole processing instruction, the `?` of its `<?` counted again in its `?>`, where cmark-gfm and commonmark.js 0.31.2 read on to the next `?>`, so that in `a <?>[x](a.md) b ?>` goldmark finds a link and they find none. Of the 348 block differences, 293 are tables. Over all, 1,840 generated bodies differ somewhere in their HTML: about 420 in the three columns above, about 1,220 only in which addresses are found and where they end, and about 200 in other ways, footnotes among them, the exact split depending on how a heading's text is compared.

Of the 728 examples, 16 differ in their HTML: 5 only in that goldmark, without its task list extension, leaves a task's box as text; 3 in where an address is found, among them `<http://foo.bar/baz bim>`, in which cmark-gfm finds one; 2 in a `<meta>` tag, which goldmark lists among the tags that open an HTML block able to interrupt a paragraph and CommonMark does not; one each in closing tags such as `</script>` alone on a line, which goldmark reads as a paragraph, a lone `|` over a delimiter row, which goldmark reads as a table, strikethrough of unequal runs, and a footnote definition's indentation; and two in emphasis and strikethrough inside a footnote label. goldmark's task list extension reads `- [x](a.md)` as a completed task and loses the link, where GFM requires whitespace after a task's box, and with it the generated bodies whose links differ rise to 142. v1.8.7 differs on 105 of the generated bodies' links, and on one example's.

One difference no measured body holds was found on 2026-11-17: goldmark reads `<!` followed by any ASCII letter as a declaration, as CommonMark 0.31.2 does, opening an HTML block at the start of a line and raw HTML up to the next `>` within one. GFM 0.29 and cmark-gfm read a declaration only before an uppercase letter, and otherwise read the text as written, so in `[a`, `<!x>`, `](n.md)` and in `a <!x [b](c.md) >` cmark-gfm reads a link and goldmark reads none.

Where each places what it reads, in the generated bodies:

| Construct | goldmark v2 | cmark-gfm |
|---|---|---|
| Links and images | the start of each of 3,749, and the range of each destination but two left empty | 392 of 3,578 started wrongly, 26 not placed and 4 given a malformed position |
| Link reference definitions | the start, label and destination of each of 380 | none: the parse holds no definitions |
| Inline raw HTML | each of 13,274 | 1,423 of 12,652 wrongly and 119 not placed |
| HTML blocks | each of 3,053 | 1,280 of 3,076 ending before they start |
| Headings | each of 2,799 | each |

cmark-gfm's figures are of the 9,835 bodies whose XML parse is well-formed. goldmark v2 places a code span at its content, after the backticks before it, and an address its linkify extension finds at the byte before the address, while giving the address's own text its exact range; and it gives 5 of 57,006 texts no range. cmark-gfm counts columns on every line of a paragraph from the first line's indentation, and places everything after a link reference definition in the same paragraph on the wrong line. In the document bodies cmark-gfm places every link, but holds none of the 49 link reference definitions they define, ends both HTML comment blocks before they start, and misplaces three code spans in table cells; goldmark v2 places all of them as above but 15 code spans in table cells holding an escaped `|`, which it gives no position. v1.8.7 gives a link's destination no range, places 350 of 3,360 addresses its linkify extension finds one byte early, and places a heading indented by a tab inside a list item after its `#`.

The shapes timed were 22: 16 from the pathological tests of CommonMark and cmark-gfm, and six added for comments in headings, comments in code spans, pipes in a paragraph, table rows, addresses, and link reference definitions followed by a reference repeated. Each was timed at two sizes, the larger holding four times the repetitions of the smaller, on one machine. goldmark's time is that of its parse alone; cmark-gfm's is the best of five whole runs writing HTML, of which a run on an empty file takes 3 ms. Seven of them:

| Shape | goldmark v1.8.7 | goldmark v2.1.16 | cmark-gfm |
|---|---|---|---|
| `*a_ ` | 1.6 s, 152 s | 9 ms, 39 ms | 10 ms, 51 ms |
| `a**b` then `c* ` | 1.3 s, 87 s | 4 ms, 20 ms | 6.5 ms, 21 ms |
| `[a](b` | 3.7 s, 57 s | 10 ms, 38 ms | 14 ms, 45 ms |
| `[a](<b` | 0.96 s, 15 s | 7 ms, 26 ms | 10 ms, 30 ms |
| List items each nested a level deeper, 112 KB and 1.8 MB | 54 ms, 3.1 s | 45 ms, 2.6 s | 4 ms, 25 ms |
| Backtick runs of every length, 56 KB and 889 KB | 5 ms, 262 ms | 5 ms, 268 ms | 3.5 ms, 6 ms |
| Link reference definitions, then a reference repeated, 93 KB and 398 KB | 16 ms, 239 ms | 9 ms, 126 ms | 5.6 ms, 15 ms |

The last three take goldmark v2 time growing faster than their length: at 16 times the length, nested list items and backtick runs take 57 and 55 times as long, and definitions alone, without the references, take 15 times as long at 4.4 times the length. Each of the other 19 takes goldmark v2 about three to five times as long at four times the length.

Two more shapes, found on 2026-11-17 and 2026-11-18 in timing tests of the outline of the body goldmark parses, take goldmark v2.1.16 time growing faster than their length: brackets left open before a run of links, and a bracketed text closed on each line of a paragraph, 800 KB of which takes 19.3 seconds. v1.8.7 reads the first in linear time, 9 ms at 128 KiB and 37 ms at 512 KiB, where v2.1.16 takes 148 ms and 2.26 s.

## Decision

The tool reads a document's body with goldmark v2, with its table, strikethrough, linkify and footnote extensions and without its task list extension, in place of recognising a set of constructs and treating everything else as text as [ADR-0006](0006-markdown-is-handled-with-the-standard-library.md) has it, into a read-only outline of the body's blocks and inline pieces, each holding its position in the file as written, taken from goldmark's nodes and values and, where goldmark gives none or one that is not the construct's own, found from them in the bytes as written; every part of the tool that reads a body reads that outline, nothing is serialised from it, and a writer still edits the file as written at an offset. The reading the tool is to give is cmark-gfm's parse, with the extensions above and its task list extension, and not the HTML GitHub displays, whose sanitiser and heading anchors remain the tool's own to reproduce; a reading on which goldmark and cmark-gfm differ is a defect, corrected in goldmark where its maintainers take the correction, and otherwise around goldmark by code here or recorded as a known difference, and cmark-gfm is run only to record the parses the tool is tested against. goldmark v2 joins the dependency list of [ADR-0005](0005-the-dependency-list-is-closed.md) as [ADR-0025](0025-the-dependency-list-adds-pflag-and-x-text.md) leaves it, which is then cobra for the command tree, pflag for its flags, yaml.v3 for front matter, huh for prompts, `x/term` to detect a terminal, x/text for Unicode normalisation, goldmark v2 for Markdown, and goreleaser at build time only, and nothing else.

## Alternatives

- **Recognising named constructs, as [ADR-0006](0006-markdown-is-handled-with-the-standard-library.md) has it.** Each construct blanked or matched by code of its own, and a new construct added when something needs it. Lost because no order of blanking and matching reproduces the order CommonMark claims text in, as the two readings under Context show, so a correction moves a misreading rather than removing it.
- **Reading inline text as CommonMark does, keeping the block walk and the readers built on it.** Weighed in a working session, 2026-11-12, not publicly available and first written down on 2026-11-13. Lost because the `Changelog`, question and term readers keep the shapes they report, headings inside containers and setext headings stay unread, and the block walk has to change regardless: inline reading needs a paragraph's text with the markers of its blockquotes and list items removed, which the walk does not produce.
- **A CommonMark and GFM parser written here with the standard library.** Weighed in a working session, 2026-11-13, not publicly available and first written down on 2026-11-13. Lost because the whole of CommonMark's block and inline grammar and GFM's extensions would be written and maintained here, where goldmark reads every document body measured as cmark-gfm does and places every construct those bodies hold but 15 code spans in table cells, which can be found from the cell's bytes. Hand-writing CommonMark's list rules for `Changelog` entries was tried before, and read several shapes differently from how they render.
- **cmark-gfm itself, linked through cgo, compiled to WebAssembly and run in Go, or run as a command.** Weighed in a working session, 2026-11-14, not publicly available and first written down on 2026-11-14. Lost because its parse holds no link reference definitions and places pieces on continuation lines and after definitions wrongly, as measured under Context, so the tool would read definitions and correct positions with code of its own, in whichever way it ran cmark-gfm. Linking it through cgo would also end the build without cgo, and running it as a command would need it installed beside the tool. The WebAssembly build was not made.
- **goldmark v1, the module `github.com/yuin/goldmark`.** Weighed in a working session, 2026-11-14, not publicly available and first written down on 2026-11-14. Lost because it reads the first four shapes under Context in time growing with the square of their length or faster, and gives a link's destination no range, so the tool would find each destination itself, though it reads brackets left open before a run of links in linear time, where v2 does not. It places every code span at its backticks, those in table cells included, and gives every text a range, where v2 does not, but places 350 addresses and a tab-indented heading wrongly where v2 places them as written.

No other alternatives were weighed.

## Consequences

Easier:

- Sections, headings, anchors, links, `[[...]]` links, `Changelog` entries, answers and term definitions will be found by one reading, so no two of them can disagree about the same text.
- A `Changelog` entry will be read in any form CommonMark reads as a list item, an answer in any form it reads as a blockquote, and a term's definition in any form it reads as a paragraph, rather than in a subset of each.
- A reading on which the tool and cmark-gfm disagree will be a defect against a parser that can be run, rather than against rules written here.
- Link labels will be matched under Unicode's full case folding, as goldmark and cmark-gfm both match them, so that `[ẞ]` matches a definition of `[SS]`.

Harder:

- Readings on which goldmark and cmark-gfm differ, among them tables, footnote definitions, `<?>`, `<!` before a lowercase letter and where an address is found, will stay misreadings until each is corrected.
- The five shapes under Context that goldmark v2 reads in time growing faster than their length will take that time until goldmark reads them in linear time.
- A release of goldmark v2, which its README describes as early in its release and which has been released often, will change what the tool reads where it changes goldmark's parse, and a release of cmark-gfm what the tool should read. The tool will take a new goldmark only once the recorded parses are read again.
- A repository adopting the tool will audit eight names rather than the seven of [ADR-0025](0025-the-dependency-list-adds-pflag-and-x-text.md).

Constrained:

- The outline will be read-only, and writers will edit by offset into the file as written; nothing will be rendered or serialised from it.
- Masked text, where a reader still needs it, will be derived from the outline and will never move a byte, as [ADR-0007](0007-masking-never-moves-a-byte.md) requires.
- A construct CommonMark does not have, such as a `[[...]]` link, will be read from the text the outline leaves as text.
- A need that would take another library will still be met with the standard library or not met.

[4cae902]: https://github.com/archdochq/archdoc/commit/4cae90263e7b6a4b9a54120f87e032381558b50b
[#67]: https://github.com/archdochq/archdoc/issues/67
[#69]: https://github.com/archdochq/archdoc/issues/69
[#70]: https://github.com/archdochq/archdoc/issues/70
[#79]: https://github.com/archdochq/archdoc/issues/79
[#84]: https://github.com/archdochq/archdoc/issues/84
[gfm]: https://github.github.com/gfm/
[cmark-gfm]: https://github.com/github/cmark-gfm
[goldmark]: https://github.com/yuin/goldmark

## Changelog

- 2026-11-18: Record two more shapes goldmark v2 reads in more than linear time, found while the reader was built, one of which v1.8.7 reads in linear time, and count the examples differing in footnote labels as two.
