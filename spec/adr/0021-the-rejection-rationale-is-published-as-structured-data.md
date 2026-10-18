---
id: ADR-0021
title: The rejection rationale is published as structured data
status: accepted
created: 2026-10-18
decided: 2026-10-18
depends: [ADR-0019]
updates: [ADR-0020]
obsoletes: []
---

# ADR-0021: The rejection rationale is published as structured data

## Context

Raised on 2026-10-17, while the gate on `reject` was being changed to judge a document with its rationale in place.

Under [ADR-0020](0020-structured-sections.md) a document's questions and its `Changelog` are published as structured data and left out of `contents`, because they record the document's process rather than its design, and archdoc.dev shows a document's contents as the document. Another section receives the same treatment only by a decision that names it, and the sections named leave `contents` in one release. [ADR-0020](0020-structured-sections.md) names `Rejection rationale` among the sections a rule for every process section would have left open.

`reject` appends `Rejection rationale` to a document, as `archdoc new` does to one it creates rejected, and L08 requires it on a rejected document and forbids it on any other RFC or ADR. It records why the proposal was turned down: the verdict, not the design. A required section that leaves `contents` is still named among the document's required sections, so under [ADR-0020](0020-structured-sections.md) its presence has to be recorded elsewhere, or the requirement lifted.

Under [ADR-0019](0019-export-stability.md), before ArchDoc 1.0, a change that removes a field, or changes a field's type or meaning, moves `schema` to the next integer, at most once between two tagged releases. `schema: 3`, which takes the question sections and the `Changelog` out of `contents`, has not been in a tagged release.

## Decision

`Rejection rationale` is added to the sections [ADR-0020](0020-structured-sections.md) names: it is published as structured data and left out of `contents`, in the release that takes the question sections and the `Changelog` out. L08's requirement of it on a rejected document stays, and the structured form records whether the section is present.

## Alternatives

- **Leaving it in `contents`.** The heading is fixed, so a consumer can find the section by its title. Lost because a consumer showing the design from `contents` would show the verdict as part of it.
- **Leaving `contents` in a later release, at `schema: 4`.** Lost because under [ADR-0019](0019-export-stability.md) a second break costs every consumer the change again, where joining `schema: 3` before it is released costs nothing beyond the break already made.
- **Lifting the requirement.** L08 would no longer require the section on a rejected document. Lost because a rejected document could then freeze with no reason for its verdict.

## Consequences

Easier:

- A consumer will show why a document was rejected without searching `contents` for its heading.

Harder:

- `Rejection rationale` will be the first required section absent from `contents`, so a consumer checking a rejected document's `required_sections` against `contents` will find it missing, and will have to read its structured form.
- A fix merged where the code emits `schema: 3` will not be released until the section's structured form is built, unless it is released from code still emitting `schema: 2`.

Constrained:

- No release will carry `schema: 3` until the section's structured form is built.
