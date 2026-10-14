---
id: ADR-0019
title: The export shape is held stable from 1.0
status: proposed
created: 2026-10-14
decided:
depends: []
updates: []
obsoletes: []
---

# ADR-0019: The export shape is held stable from 1.0

## Context

Raised on 2026-10-14, when a change was weighed that would leave the export's JSON structure as it is but change what one of its fields means.

[RFC-0004](../rfc/0004-exporting-the-repository-as-json.md) applied the principle of [ADR-0004](0004-the-front-matter-schema-only-gains-optional-keys.md) to the export: its shape only ever gains optional fields. [RFC-0006](../rfc/0006-the-export-publishes-what-it-parsed.md) broke it once, moving `schema` from `1` to `2`. It amended the rule so that the shape gains only optional fields from `schema: 2` onwards, and stated that a second break would belong in an ADR. The published account of the shape promises that "a field will not change meaning or type without the version changing", and the published `v2.json` describes itself as a shape that "only ever gains optional fields".

That promise has already been broken once without a change of number. Under [RFC-0007](../rfc/0007-the-glossary-is-a-type-of-its-own.md) the export stopped emitting the glossary page as a document, so a consumer reading terms from `documents` stopped seeing them, and `schema` stayed `2`. Two released shapes share that number.

ArchDoc is pre-1.0, and its README describes it as alpha, with a command surface that may still move. Tagged releases, pre-releases among them, have emitted `schema: 1`, from `v0.1.0-alpha.4` to `v0.1.0`, and `schema: 2`, from `v0.2.0`. The identifier each schema carries has moved with the repository: the pre-releases from `v0.1.0-alpha.4` to `v0.1.0-alpha.6` named a path that was not a file, `v0.1.0` named the file under the repository's first owner, and `v0.2.0` onwards name it under `archdochq`.

## Decision

Before ArchDoc 1.0, the export shape may break. A change that removes a field, or changes a field's type or meaning, moves `schema` to the next integer, at most once between two tagged releases, pre-releases included, however many such changes land between them. A number a tagged release has emitted is never given to another shape, and each released schema stays published. From 1.0, the shape only gains optional fields, starting from whichever number is current at 1.0.

## Alternatives

- **Squashing the number to `1` at the first stable release.** Lost because tagged releases have already emitted `1` and `2`, and their schemas are published. A later shape numbered `1` would give one number to two shapes, and a consumer holding output or a published schema could not tell which it has.
- **Treating a change of meaning with no change of structure as no break.** The way the change under [RFC-0007](../rfc/0007-the-glossary-is-a-type-of-its-own.md) was taken. Lost because a consumer of the earlier shape loses data without being told, which is what the number exists to signal.

## Consequences

Easier:

- A design before 1.0 will be able to change the shape it needs, at the cost of one number.
- Shapes released from this decision on will be told apart by `schema` alone. The two released under `2` before it will not.

Harder:

- A consumer before 1.0 will have to handle every number it can meet, including in output it stored under an earlier release.
- Each break before 1.0 will still cost every consumer the change it describes.

Constrained:

- From 1.0, the shape will only gain optional fields.
- The published account of the export's stability, including the description each published schema gives of itself, will say that the promise starts at 1.0.
