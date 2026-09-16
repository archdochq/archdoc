---
id: ADR-0003
title: Reverse relationships are derived
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0003: Reverse relationships are derived

## Context

A document names the documents it rests on, amends or replaces, and a spec page names the documents it reflects. Every one of those relationships has a reverse: what rests on this document, what amends it, what replaced it, which pages reflect it. A reader wants both directions. Under [ADR-0001](0001-terminal-documents-are-frozen.md) a terminal document is never modified, so the reverse of a relationship formed after a document froze cannot be written into it.

## Decision

Front matter records forward relationships only: `depends`, `updates` and `obsoletes` on an RFC or ADR, and `includes` on a spec page. Every reverse relationship is derived by the tooling from the forward ones across the whole repository and published in the generated index. Nothing derived is ever written into a document. The same derivation yields three facts about a document: it is effectively obsolete when an accepted document obsoletes it, it is implemented when it is accepted and some spec page includes it, and a spec page is stale when it includes something effectively obsolete.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- Recording a new relationship touches one document, the one making the claim.
- A frozen document acquires reverse relationships for as long as the repository lives, with no edit to it.

Harder:

- A document read on its own shows only what it knew when written. What later referred to it is elsewhere.

Constrained:

- The tooling will have to read every document to answer a question about any one of them.
- The derived relationships will need somewhere to be published, since they live in no document.
- Whether a document is obsolete, implemented or stale will be computed from the graph, not stated in front matter.

## Sources

- `PROCESS.md` at [9654fe2], section Front matter, states that only forward relationships are recorded and gives the reason: a frozen document cannot be edited to record what happened to it later. This sets `created` and `decided`.
- `docs/ARCHDOC.md` at [9654fe2a], section Relationships and derivation, lists what is derived and defines effectively obsolete, implemented and stale.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/template/PROCESS.md
[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
