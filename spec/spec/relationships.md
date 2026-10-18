---
title: Relationships
includes: [ADR-0003, ADR-0014, RFC-0011]
---

# Relationships

Front matter records forward relationships: `depends`, `updates` and `obsoletes` on an RFC or ADR, `includes` on a spec page, and `documents` on a group. Every reverse relationship is derived by reading the whole repository, and published in the export. `updated_by` and `obsoleted_by` are also published in the index, and `included_in`, less the glossary, as `Implemented in`.

## Derived

For each document:

- `updated_by`: every document listing it in `updates`.
- `obsoleted_by`: every document listing it in `obsoletes`.
- `depended_on_by`: every document listing it in `depends`.
- `included_in`: for an RFC or ADR, every spec page listing it in `includes`.
- `groups`: for an RFC, ADR, ref or spec page, the slug of every group listing it in `documents`, in order of slug. A group listing a member twice counts once.

And three facts:

- A document is **effectively obsolete** when any document in its `obsoleted_by` is accepted. Obsolescence claimed by a draft, proposed, rejected or withdrawn document has no effect.
- An RFC or ADR is **implemented** when it is accepted and some spec page includes it. A term names the document that introduced it in `named_by`, which is not an inclusion and contributes nothing here.
- A spec page is **stale** when it includes a document that is effectively obsolete.

Nothing derived is written into any document.
