---
id: ADR-0009
title: A lint error names something the user may change
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0009: A lint error names something the user may change

## Context

Lint reports findings at two severities, and an error fails the build. Under [ADR-0001](0001-terminal-documents-are-frozen.md) a terminal document is never modified. A rule whose subject is the body of a frozen document can find a fault there that no permitted edit could clear: an empty required section, an unresolved wiki link, a link to a heading that no longer exists. Reported as an error, that fault fails the build forever, and the only remedy is the edit the freeze forbids.

## Decision

A lint error names something the user is permitted to change. A finding that no permitted edit to the named document could clear is a warning. This applies to the rules whose subject is the document's own body, which are L14, L16 and L17, when the document is frozen. It does not apply to a fault in a relationship between documents, such as L04's, which is cleared by editing the other document rather than the one named. It does not apply to L11 itself, whose finding is that a frozen document was edited, and whose remedy is to revert.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- A repository with a fault inside a frozen document can still pass, and the fault is still reported.
- An error is always a task: something the person reading it can do.

Harder:

- A warning on a frozen document is a permanent fixture of the output. It records something real that will never be fixed in place.

Constrained:

- Every rule will decide its severity by whether the document it names is frozen, so every rule needs to know.
- A rule about a relationship keeps its error severity even when one side is frozen, because the other side is not.

## Sources

- `docs/ARCHDOC.md` at [9654fe2a], the preamble to the lint rules, states the softening, the three rules it covers, and the two it does not. This sets `created` and `decided`.
- `docs/DECISIONS.md` at [9654fe2d], section The lifecycle and freezing, records the same rule.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
