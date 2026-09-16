---
id: ADR-0007
title: Masking never moves a byte
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0006]
updates: []
obsoletes: []
---

# ADR-0007: Masking never moves a byte

## Context

Under [ADR-0006](0006-markdown-is-handled-with-the-standard-library.md) the tool rewrites a file by offset into it as written. Before it searches a body for a heading, a link or a glossary term, it has to set aside the text that must not match: the inside of an HTML comment, a code span, a fenced block. Whatever is done to that text, a match found afterwards has to land at the same place in the original file, or the edit spliced there lands somewhere else.

## Decision

Masking never moves a byte. Every masker writes a space over each byte it hides, so the masked text is the same length as the text as written and differs from it only where a byte became a space. A match found in the masked text is at the same offset in the file, and an edit spliced by that offset lands in the right column. The invariant is fuzzed rather than sampled.

## Alternatives

- **Removing masked text rather than blanking it.** Dropping a comment shortens the line, so every offset after it is wrong, and prose sharing a line with the comment is lost with it.

## Consequences

Easier:

- Every writer can search the masked text and edit the original by the same offsets, with no translation between the two.
- The property is stated in one sentence and can be checked by a fuzzer against any input.

Harder:

- Nothing masked can be normalised, collapsed or reflowed on the way through, because any of those moves a byte.

Constrained:

- Every masker, present and future, writes spaces in place and nothing else.
- A multi-byte character inside masked text is blanked byte by byte, so masked text is not guaranteed to be valid UTF-8 and is never shown to anyone.

## Sources

- `docs/DECISIONS.md` at [9654fe2d], section Markdown, states the invariant, the offset reasoning, and that comments are blanked in place rather than dropped. This sets `created` and `decided`.
- `internal/repo/repo_test.go` at [9654fe2t], `FuzzProseNeverMovesAByte`, is the fuzzed check that the text as written and the masked text are the same length and differ only by spaces.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
[9654fe2t]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/repo/repo_test.go
