---
id: ADR-0020
title: Questions and the Changelog are published as structured data
status: proposed
created: 2026-10-14
decided:
depends: [ADR-0019]
updates: []
obsoletes: []
---

# ADR-0020: Questions and the Changelog are published as structured data

## Context

Raised on 2026-10-14, while a structured form for a document's questions in the export was being designed. The `Changelog` was named at the same time as sharing the treatment.

The export's `contents` carries every heading of a document and the prose beneath it. The export feeds archdoc.dev, which displays documents for their purpose rather than only rendering them, with smarter features than a rendered page. A document's questions and its `Changelog` record the document's process rather than its design. Questions are matters requiring action, and the `Changelog` is the history of edits made while proposed. Carried in `contents`, they sit among what a reader reads to understand the design, and a consumer that handles them separately has to find and remove them.

The glossary has the same shape. Under [RFC-0007](../rfc/0007-the-glossary-is-a-type-of-its-own.md) a term is published in `glossary` and nowhere else, so a consumer has one representation of it.

## Decision

A document's questions and its `Changelog` are published in the export as structured data of their own, and their sections, headings and prose are left out of `contents`. An option restores every heading to `contents`, for a consumer rendering a document as written. Another section receives this treatment only by a decision that names it.

## Alternatives

- **Carrying them in both `contents` and their structured form.** ~~Chosen first, because `contents` renders the document as written and the structured form serves review.~~ Reversed the same day, 2026-10-14. Lost because archdoc.dev shows a document's contents as the document and handles questions separately, as matters requiring action.
- **Their structured form by reference.** The structured form would carry anchors and lines, with the prose read from `contents`. Lost because the two serve different consumers.
- **Tagging their entries in `contents`.** Each entry under these sections would carry a field naming the section, and the shape would only gain a field. Lost because they would still be published twice, and every consumer would filter them.
- **A rule for every section that records a document's process.** Lost because it leaves open what else counts, such as `Rejection rationale` and `Sources`.

## Consequences

Easier:

- A consumer will show a document's design from `contents` with nothing to remove.
- A document's questions and edit history will be readable as data, across documents, without parsing prose.

Harder:

- `contents` will no longer hold every heading unless the option is given, so a consumer rendering a document as written will have to pass it or read `source`.
- Leaving headings out of `contents` will change what it means, which is a break under [ADR-0019](0019-export-stability.md).
- A section named here that a type requires will be listed among the required sections while absent from `contents`, so its presence will have to be recorded elsewhere, or the requirement lifted.

Constrained:

- Each section named here will need a structured form designed before it leaves `contents`, and the sections named will leave in one release, so the shape will break once.
- A section will be added to this list only by a decision naming it.
