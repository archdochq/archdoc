---
id: ADR-0002
title: Identity is the filename number
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# ADR-0002: Identity is the filename number

## Context

RFCs, ADRs and refs are referred to from other documents, from the index and from links in prose, and those references have to name something that does not change. A document carries a title, a filename and a front matter block, and any of the three could serve as its identity. A title changes as a document is revised. A front matter value can be edited to say anything. The filename is what git tracks and what a directory listing shows.

## Decision

A numbered document's identity is the number in its filename. Each type has its own sequence, starting at 1 and zero-padded to four digits, and the identifier is the uppercased type joined to that number: `RFC-0001`, `ADR-0001`, `REF-0001`. The filename is the number followed by a slug derived from the title, `rfc/0001-database.md`, and the slug is for people; the number is the identity. The `id` in the front matter is data to be checked against the identity derived from the filename, not the source of it. A filename that does not fit the grammar yields no identity at all. The tool never renames a file: if the title changes, the slug in the filename stays as it was. Spec pages are not numbered and are identified by their path below `spec/` without the extension.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- A document's identity is visible in a directory listing and in every link to it, with nothing to open.
- A front matter `id` that disagrees with the filename is a detectable fault rather than a silent redefinition.
- References between documents name the identifier and survive a change of title.

Harder:

- Renaming a file changes a document's identity, so the tool leaves filenames alone and a slug can drift from the title it was made from.
- Two files carrying the same number claim the same identity, and nothing in either file says which is right.

Constrained:

- The filename grammar is fixed: four digits, a hyphen, a slug, `.md`. Anything else is not a document of that type.
- The tooling will have to check every front matter `id` against the identity the filename gives.
- Numbering is bounded at 9999, since a fifth digit would produce a filename the grammar cannot read back.

## Sources

- `PROCESS.md` at [9654fe2], section Identity, states that the number is the identity and the slug is for people, and that references use the identifier and never the filename. This sets `created` and `decided`.
- `docs/ARCHDOC.md` at [9654fe2a], section Identifiers, gives the identifier form, the filename grammar, the sequence rule and the 9999 bound.
- `docs/DECISIONS.md` at [9654fe2d], section Documents and front matter, records that the front matter `id` is checked against the derived identity rather than defining it.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/template/PROCESS.md
[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
