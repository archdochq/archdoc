---
id: ADR-0004
title: The front matter schema only gains optional keys
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0004: The front matter schema only gains optional keys

## Context

Every document opens with a front matter block, and the tooling checks that block against a schema for the document's type. Under [ADR-0001](0001-terminal-documents-are-frozen.md) a terminal document is never modified. A key added to the schema as required would be absent from every document written before it, and the terminal ones among those could not be edited to add it. Every repository that existed would then fail the check with no permitted way to pass it.

## Decision

The front matter schema only ever gains optional keys. What a key may be and what must be present are two separate lists, so that a key can be added to the first without joining the second. `backfilled` is optional, and every key added after it is optional too.

## Alternatives

- **One list serving as both what is permitted and what is required.** This was the first form. It made it possible to add a key that every existing document lacked, at which point a new required key would put every frozen document in every existing repository in violation of the front matter check, and the freeze would forbid the edit that adds it.

## Consequences

Easier:

- A document valid under one version of the tooling stays valid under every later one.
- A repository can be upgraded without touching a frozen document.

Harder:

- A key that ought to be required in every new document can only be optional, and its absence can only be reported as a warning at most.

Constrained:

- The schema is append-only in effect. A required key can be removed but never added.
- Permitted keys and required keys will be kept as distinct lists in the tooling.

## Sources

- `internal/repo/parse.go` at [9654fe2p], the docblock on `RequiredKeys`, states the rule and its reason. This sets `created` and `decided`.
- `docs/DECISIONS.md` at [9654fe2d], section Documents and front matter, records the earlier single-list form and why it was replaced.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2p]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/repo/parse.go
[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
