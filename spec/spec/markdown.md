---
title: Markdown
includes: [ADR-0006, ADR-0007]
---

# Markdown

The body of a document is everything after the closing `---` of the front matter. The tool reads four constructs from it: headings, HTML comments, fenced code blocks and links. Everything else is text.

## Headings

Headings are `#` through `######`. A line inside a ``` or ~~~ fence is not a heading, and neither is a `#` indented four or more spaces. This holds wherever headings are read: section parsing, anchors and link scanning. Indented code blocks are not otherwise recognised.

The first heading is an H1 of the form `<id>: <title>` for a numbered type, or `<title>` for a spec page.

## Anchors

Every heading has an anchor, computed as GitHub computes it. Link syntax is stripped first, so a heading is slugged from its rendered text. Everything that is not a letter, digit, underscore, hyphen or space is removed, the result is lowercased, and each space becomes a hyphen. A heading whose anchor is already taken within the document gains a `-1`, `-2` suffix. Anchors are assigned by walking a document's headings in order; a heading in isolation has no anchor. `## The [RFC-0007](path) approach` anchors as `the-rfc-0007-approach`.

## Sections

A section is an H2 and everything under it, up to the next heading of equal or higher level. It includes its own subsections. A section is empty if it holds nothing but whitespace and HTML comments.

Section presence is checked by exact H2 text. The required sections by type:

- RFC: `Abstract`, `Motivation`, `Proposal`, `Alternatives considered`, `Backwards compatibility`, `Open questions`, `Changelog`, in that order. `Rejection rationale` is required when the status is `rejected` and forbidden otherwise. `Sources` is required when `backfilled` is set. A document requiring either has a closing section that is its last H2: `Rejection rationale` when rejected, `Sources` otherwise. A document requiring neither may carry extra sections after the required ones.
- ADR: `Context`, `Decision`, `Alternatives`, `Consequences`, in that order. `Rejection rationale` and `Sources` as for an RFC.
- Ref: `Sources` is the last H2.
- Spec: no requirements, except the glossary.

Extra H2 sections are permitted between or after the required ones.

## Comments

Text inside an HTML comment is not content anywhere. It is passed over when finding headings and sections, when resolving `[[...]]` links, and when suggesting them. This holds whether the comment closes on the line it opened on or several lines later. A comment that is never closed runs to the end of the document. Text sharing a line with a comment, before its `<!--` or after its `-->`, is content.

## Code

A `<!--` or `-->` inside an inline code span or a fenced code block is literal text and opens or closes nothing. A fence marker inside a comment is likewise literal. A comment and a fenced block are both blocks: whichever opens first holds the lines until it closes.

A code span closes on a backtick run of exactly the length that opened it. A fence closes on a run at least as long as the one that opened it, carrying no info string.

## Masking

Before a body is searched, the text that must not match is masked: the inside of comments, code spans and fenced blocks. Every masked byte becomes a space. The masked text is the same length as the text as written and differs from it only where a byte became a space, so a position found in one is the same position in the other. Masked text is never shown.
