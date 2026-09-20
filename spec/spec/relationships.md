---
title: Relationships
includes: [ADR-0003, ADR-0014]
---

# Relationships

Front matter records forward relationships: `depends`, `updates` and `obsoletes` on an RFC or ADR, and `includes` on a spec page. Every reverse relationship is derived by reading the whole repository, and published in the index and the export.

## Derived

For each document:

- `updated_by`: every document listing it in `updates`.
- `obsoleted_by`: every document listing it in `obsoletes`.
- `depended_on_by`: every document listing it in `depends`.
- `included_in`: for an RFC or ADR, every spec page listing it in `includes`.

And three facts:

- A document is **effectively obsolete** when any document in its `obsoleted_by` is accepted. Obsolescence claimed by a draft, proposed, rejected or withdrawn document has no effect.
- An RFC or ADR is **implemented** when it is accepted and some spec page other than the glossary includes it.
- A spec page is **stale** when it includes a document that is effectively obsolete.

Nothing derived is written into any document.
