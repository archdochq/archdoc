---
id: ADR-0014
title: The glossary does not count as implementation
status: accepted
created: 2026-09-16
decided: 2026-09-16
backfilled: 2026-09-20
depends: []
updates: [ADR-0003]
obsoletes: []
---

# ADR-0014: The glossary does not count as implementation

## Context

Under [ADR-0003](0003-reverse-relationships-are-derived.md) a document is implemented when it is accepted and some spec page includes it. The glossary is a spec page, and its `includes` list is filled in by the same means as any other: the shipped glossary template tells an author that the RFC which named or renamed a term belongs there, and `archdoc term add --from` writes it.

Citing the RFC that named a term therefore marked that RFC as built. The tool reported as implemented whatever it had just instructed someone to cite, and the index named the glossary as the page where the work had been done.

## Decision

Inclusion by the glossary does not make a document implemented. A document is implemented when it is accepted and some spec page other than the glossary includes it, and the index's `Implemented in` column lists the same pages the fact is derived from.

The `included_in` list keeps the glossary as written. Which spec pages reference a document is a separate and factual question from whether it has been built.

## Alternatives

- **Counting the glossary like every other spec page.** This was the first form. It made `archdoc term add --from RFC-0001`, which the glossary template asks for, enough on its own to report RFC-0001 as built.

Whether anything other than the form it replaced was weighed is not recorded.

## Consequences

Easier:

- A document is reported as implemented only where a page describes the behaviour it specifies.
- Citing the RFC that named a term carries no consequence beyond the citation.

Harder:

- The glossary is a spec page under every other rule, so this exception has to be restated wherever implementation is described.

Constrained:

- The two readings of `includes` will be kept apart: the full list answers what references a document, and the filtered list answers whether it is built. Anything deriving one will have to say which it means.

## Sources

- [24f2f7a] makes the change and sets `created` and `decided`. Its message states that the glossary template asks for the RFC that named a term to go in `includes`, so the tool reported as built whatever it had just told someone to cite.
- `internal/repo/derive.go` at [24f2f7ad], the comment on `Implemented` and the docblock on `ImplementedIn`, state the reasoning and the separation of the two lists.
- `internal/index/index.go` at [24f2f7ai], the `Implemented in` column, states that the column has to agree with the fact derived from the same list.
- `internal/template/glossary.md` at [24f2f7ag] carries the instruction the reasoning rests on.

[24f2f7a]: https://github.com/ollieread/archdoc/commit/24f2f7a20642446270428a23a0a4fcc2f7e7cbad
[24f2f7ad]: https://github.com/ollieread/archdoc/blob/24f2f7a20642446270428a23a0a4fcc2f7e7cbad/internal/repo/derive.go
[24f2f7ai]: https://github.com/ollieread/archdoc/blob/24f2f7a20642446270428a23a0a4fcc2f7e7cbad/internal/index/index.go
[24f2f7ag]: https://github.com/ollieread/archdoc/blob/24f2f7a20642446270428a23a0a4fcc2f7e7cbad/internal/template/glossary.md
