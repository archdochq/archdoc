---
id: ADR-0006
title: Markdown is handled with the standard library
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0005]
updates: []
obsoletes: []
---

# ADR-0006: Markdown is handled with the standard library

## Context

The tool reads Markdown to find headings, HTML comments, fenced code blocks and links, and it rewrites Markdown in place: a status in front matter, a heading, a wiki link expanded, a glossary entry inserted or removed. It needs nothing else from the format. Under [ADR-0005](0005-the-dependency-list-is-closed.md) a library for the purpose would have to earn a place on a closed list.

## Decision

Markdown is handled with the standard library. The tool recognises the four constructs it needs and treats everything else as text.

## Alternatives

- **A full Markdown parser.** It would be a liability when rewriting files in place, because it would have to round-trip everything it parsed, and the tool has no use for anything beyond the four constructs.

## Consequences

Easier:

- A rewrite touches only the bytes it means to. Nothing else in the file is parsed, so nothing else can be reformatted.
- The dependency list stays at five.

Harder:

- Every construct the tool does recognise is recognised by code written here, including the parts of CommonMark that are awkward: a code span closing on a backtick run of the same length, a fence closing on a run at least as long, a comment that never closes.
- A construct the tool does not recognise is text. A heading inside an HTML block, or a link written in a form the tool does not read, is invisible to it.

Constrained:

- The set of recognised constructs is headings, HTML comments, fenced code blocks and links, and grows only when something needs more.
- Rewriting will have to work by offset into the file as written, since there is no parsed tree to serialise back.

## Sources

- `docs/ARCHDOC.md` at [9654fe2a], section Dependencies, states that Markdown is handled with the standard library and why a full parser would be a liability. This sets `created` and `decided`.
- `docs/DECISIONS.md` at [9654fe2d], section Markdown, states the four constructs needed and that a full parser would have to round-trip everything it parsed.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
