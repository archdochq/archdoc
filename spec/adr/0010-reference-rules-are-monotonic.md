---
id: ADR-0010
title: Reference rules are monotonic
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0010: Reference rules are monotonic

## Context

A document names other documents in `depends`, `updates`, `obsoletes` and `includes`, and lint checks what it names. Under [ADR-0001](0001-terminal-documents-are-frozen.md) the naming document may be frozen. If a rule could be satisfied when the document froze and violated later because the named document changed status, a frozen document would start failing lint through no edit of its own, with no permitted way to stop it.

## Decision

Every reference rule is monotonic: a reference that is valid stays valid. `includes`, `updates` and `obsoletes` may name only accepted documents, and accepted is a terminal status, so what they name never changes status again. No document can be put in violation by another author's later action.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- A frozen document that passed lint when it froze passes lint for as long as the repository lives.
- Lint output for a repository changes only when a document in it changes.

Harder:

- A reference to a document that is not yet accepted cannot be made through `updates`, `obsoletes` or `includes`. A design that builds on a proposed document names it in `depends` instead.

Constrained:

- Any rule added later that checks one document against another will have to hold once satisfied, or it breaks the property for every frozen document at once.

## Sources

- `docs/DECISIONS.md` at [9654fe2d], section Linting, states the property and the reasoning. This sets `created` and `decided`.
- `PROCESS.md` at [9654fe2], section Relationships, states that `updates` and `obsoletes` may reference only accepted documents, and that a spec page may only include accepted documents.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
[9654fe2]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/template/PROCESS.md
