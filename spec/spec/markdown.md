---
title: Markdown
includes: [ADR-0006, ADR-0007, RFC-0008, RFC-0009, RFC-0011]
---

# Markdown

The body of a document is everything after the closing `---` of the front matter. The tool reads headings, HTML comments, fenced code blocks and links throughout it, blockquotes within the question sections, and list items within a `Changelog` section. Everything else is text. A carriage return before a line feed is part of the line ending, and a construct is recognised in a CRLF file exactly as in an LF one.

## Headings

Headings are `#` through `######`. A line inside a ``` or ~~~ fence is not a heading, and neither is a `#` indented four or more spaces. This holds wherever headings are read: section parsing, anchors and link scanning. Indented code blocks are not otherwise recognised.

The first heading is an H1 of the form `<id>: <title>` for a numbered type, or `<title>` for a spec page, a term or a group.

## Anchors

Every heading has an anchor, computed as GitHub computes it. Link syntax is stripped first, so a heading is slugged from its rendered text. Everything that is not a letter, digit, underscore, hyphen or space is removed, the result is lowercased, and each space becomes a hyphen. A heading whose anchor is already taken within the document gains a `-1`, `-2` suffix. Anchors are assigned by walking a document's headings in order; a heading in isolation has no anchor. `## The [RFC-0007](path) approach` anchors as `the-rfc-0007-approach`.

## Sections

A section is an H2 and everything under it, up to the next heading of equal or higher level. It includes its own subsections. A section is empty if it holds nothing but whitespace and HTML comments.

Section presence is checked by exact H2 text. The required sections by type:

- RFC: `Abstract`, `Motivation`, `Proposal`, `Alternatives considered`, `Backwards compatibility`, in that order. `Rejection rationale` is required when the status is `rejected` and forbidden otherwise. `Sources` is required when `backfilled` is set. A document requiring either has a closing section that is its last H2: `Rejection rationale` when rejected, `Sources` otherwise. A document requiring neither may carry extra sections after the required ones.
- ADR: `Context`, `Decision`, `Alternatives`, `Consequences`, in that order. `Rejection rationale` and `Sources` as for an RFC.
- Ref: `Sources` is the last H2.
- Spec: no requirements.
- Term: no required sections. The definition is the prose under the H1, and L15 wants exactly one paragraph of it.
- Group: no required sections. Its description is the body after the H1.

Extra H2 sections are permitted between or after the required ones. `Open questions`, `Resolved questions` and `Changelog` are optional for every type, and are read as described below wherever they appear.

## Questions

The question sections are H2s titled `Open questions` and `Resolved questions`. A question is an H3 within one, and its text is the question. Its detail is the lines after it, to its answer or to the next heading. A document carries at most one `Resolved questions` section, and where it has an `Open questions` section, `Resolved questions` is the next H2 or H1 after one.

Within a question section, a blockquote is a run of consecutive lines each beginning with `>` at the start of the line, outside a fenced code block and an HTML comment. A question ends with a blockquote when its last one is followed by nothing but blank lines and comments before the next heading or the end of the document. In `Resolved questions`, that blockquote is the question's answer: its lines with the `>` and one column of the whitespace after it removed from each, a tab after the marker leaving two spaces. A link reference definition behind `>` in a question section is read as a link.

An `Open questions` section is empty or holds questions and nothing before the first. A `Resolved questions` section holds at least one question and nothing before the first. No question has a heading beneath it, and no blockquote in a question section holds a heading, a fenced code block or an HTML comment. An open question does not end with a blockquote. A resolved question ends with a non-empty one, with no other blockquote before it separated by only blank lines and comments. No question's heading produces an empty anchor, or the same anchor as another heading in the document, compared before any suffix is added.

## Changelog entries

A `Changelog` section runs from its H2 to the next H1 or H2. A carriage return before a line feed is removed before a line of it is read, and a blank line holds nothing but spaces and tabs.

An entry line begins `- ` at column 0 with a character other than a space or a tab after the space, and is not a line made only of three or more `-`, spaces and tabs. A block opener is a line beginning `#`, `>`, `<`, `-`, `*`, `+`, `_`, `=`, `|`, `[`, a backtick or `~`, or digits followed by `.` or `)`. After an entry line, each line in turn belongs to the entry while it is indented by two or more spaces with no tab before its first non-blank character; or blank, where a later line belongs to the entry; or a line at column 0 that is neither blank nor a block opener, directly after the entry line or another such line, where what follows the entry line's `- ` is not a block opener either. The first other line ends the entry.

Comment lines run from a line beginning `<!--` at column 0 outside every entry to the first line, starting with that one, that holds `-->`, where that line ends with `-->`. A line within them is not an entry line. Every line of the section that is not in an entry, blank or within comment lines belongs to no entry.

An entry's date is four digits, two and two, separated by `-`, directly after `- `, followed by `:` and then a space or the end of the line, that is a real calendar date. An entry without one is undated. Its text is its lines with `- ` and any date, its `:` and one space removed from the first, the first two spaces removed from each later line indented by two or more, and blank lines at either end dropped. A nested item is text within its entry.

Entries are read from every `Changelog` section, in document order. A level two heading differing from `Changelog` only in case, spacing or punctuation is not a `Changelog` section.

## Comments

Text inside an HTML comment is not content anywhere. It is passed over when finding headings and sections, when resolving `[[...]]` links, and when suggesting them. This holds whether the comment closes on the line it opened on or several lines later. A comment that is never closed runs to the end of the document. Text sharing a line with a comment, before its `<!--` or after its `-->`, is content.

## Code

A `<!--` or `-->` inside an inline code span or a fenced code block is literal text and opens or closes nothing. A fence marker inside a comment is likewise literal. A comment and a fenced block are both blocks: whichever opens first holds the lines until it closes.

A code span closes on a backtick run of exactly the length that opened it. A fence closes on a run at least as long as the one that opened it, carrying no info string.

## Masking

Before a body is searched, the text that must not match is masked: the inside of comments, code spans and fenced blocks. Every masked byte becomes a space. The masked text is the same length as the text as written and differs from it only where a byte became a space, so a position found in one is the same position in the other. Masked text is never shown.
