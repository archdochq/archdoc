---
id: ADR-0001
title: Terminal documents are frozen
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# ADR-0001: Terminal documents are frozen

## Context

A specification repository holds RFCs and ADRs as a record of what was proposed and decided. Each moves through a lifecycle that ends in one of three terminal statuses: accepted, rejected or withdrawn. The value of the record is that a reader can trust what a document said at the date its status was decided. A document that can be edited after that date cannot be read that way, because nothing distinguishes what stood at the time from what was changed afterwards.

## Decision

A document with a terminal status is never modified again. To change what an accepted document says, a new document is written that updates or obsoletes it. This holds for a mistake as small as a typo: the typo is corrected by a new document that updates the old one, or it is left standing, because the record is worth more than the spelling. A backfilled document, created directly in a terminal status, is frozen by the same rule as any other.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- A terminal document can be read as what stood on its decided date, without checking its history.
- The record cannot be revised without leaving a new document that says so.

Harder:

- A mistake in a terminal document is permanent. Correcting it costs a whole new document, and the mistaken one stays in the repository beside it.
- A document must be right before it reaches a terminal status, since that is the last point at which it can be changed.

Constrained:

- The tooling will have to tell a terminal document from an editable one, and refuse or report any change to the former.
- Nothing that happens to a document after it is frozen can be written into it. Anything the repository needs to know about a frozen document later will have to be derived from other documents.
- Any check the tooling applies to a frozen document will have to be one that no permitted edit could fail, or it will be reporting something nobody is allowed to fix.
- A document that a frozen document refers to will have to stay valid, since the frozen document cannot be edited to follow a change in it.

## Sources

- `PROCESS.md` at [9654fe2], section Lifecycle, states the decision and its reason as first written down. This sets `created` and `decided`.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/template/PROCESS.md
