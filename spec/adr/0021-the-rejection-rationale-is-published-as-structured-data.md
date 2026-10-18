---
id: ADR-0021
title: The rejection rationale is published as structured data
status: draft
created: 2026-10-18
decided:
depends: [ADR-0019]
updates: [ADR-0020]
obsoletes: []
---

# ADR-0021: The rejection rationale is published as structured data

## Context

Raised on 2026-10-17, while the gate on `reject` was being changed to judge the rationale it appends.

Under [ADR-0020](0020-structured-sections.md) a document's questions and its `Changelog` are published as structured data and left out of `contents`, because they record the document's process rather than its design, and archdoc.dev shows `contents` as the document. Another section receives the same treatment only by a decision that names it. [ADR-0020](0020-structured-sections.md) turned down a rule for every section that records a document's process, because it left open what else counts, and named `Rejection rationale` and `Sources` as examples.

`reject` appends `Rejection rationale` to a document, and L08 requires it on a rejected document and forbids it on any other RFC or ADR. It records why the proposal was turned down: the verdict, not the design. Carried in `contents`, it sits among the sections a consumer shows as the design.

Under [ADR-0019](0019-export-stability.md), a change that removes a field or changes its meaning moves `schema` to the next integer, at most once between two tagged releases however many such changes land between them. Schema 3, which leaves the question sections and the `Changelog` out of `contents`, has not been in a tagged release.

## Decision

Every section titled `Rejection rationale` is published in the export as `rejection_rationale`, whatever the document's status or type, and the section, its heading and every heading beneath it are left out of `contents`. `rejection_rationale` holds each such section in document order, with its heading's `anchor` and `line` and its `body` as written, subsections included, and is empty for a document that has none. `--all-headings` restores the section to `contents`, as it does the question sections and the `Changelog`, and `--no-bodies` and `index.json` omit its bodies, as they do those of questions and Changelog entries.

## Alternatives

- **Leaving it in `contents`.** The heading is fixed, so a consumer can find the section by its title. Lost because a consumer showing the design from `contents` would show the verdict as part of it, which is why [ADR-0020](0020-structured-sections.md) takes the question sections and the `Changelog` out.
- **Publishing it in both `contents` and its own field.** Lost because `contents` would still carry it, so it would be published twice and still shown as part of the design.
- **A single string, taken from the first section of that name.** Lost because a second section of the same title is legal Markdown, and its prose would appear nowhere but `source`.
- **Giving withdrawal the same treatment.** Lost because a withdrawal records no reason: no section is appended or required, so there is nothing to publish.

## Consequences

Easier:

- A consumer will show why a document was rejected without searching `contents` for a heading.
- A rejected document's `contents` will no longer carry the verdict among the design.

Harder:

- `contents` will no longer hold a rejected document's rationale unless `--all-headings` is given.
- `Rejection rationale` will be the first required section absent from `contents`, so a rejected document's `required_sections` will name a section a consumer finds in `rejection_rationale`.

Constrained:

- Landing before schema 3 is in a tagged release, the change will be part of the same break. Landing after, it will move `schema` to 4 under [ADR-0019](0019-export-stability.md).
- `Sources` will stay in `contents` until a decision names it.
