---
id: RFC-0013
title: A ref can record an exchange
status: accepted
created: 2026-10-18
decided: 2026-10-19
depends: [ADR-0004, ADR-0019]
updates: [RFC-0004, RFC-0008]
obsoletes: []
---

# RFC-0013: A ref can record an exchange

## Abstract

A ref gains an optional `as_of` date, which marks it as a record of an exchange that took place on that date. L13 does not report a ref carrying it, L07 checks it is not after `verified`, `archdoc new` writes it on a ref from `--as-of` or a prompt, and the export publishes it beside `verified`.

## Motivation

A ref carries a `verified` date, the last time its content was checked against its sources, and L13 warns once that date is older than `ref_stale_days`, 180 by default. Research and comparisons go out of date, and the warning asks for them to be checked again.

A ref that records an exchange, such as a review round, a meeting or a discussion, describes what was said on one date, and does not go out of date. L13 cannot tell a record of an exchange from research, so every such record starts warning after `ref_stale_days`, asking for a check of something that cannot have changed. The edits that clear the warning are moving `verified` forward after that check, every `ref_stale_days`, or setting `ref_stale_days` to `0`, which disables the check for every ref in the repository.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| `as_of` | An optional front matter key on a ref: the date of the exchange the ref records |
| Record of an exchange | A ref carrying `as_of` |

### Components

| Component | Responsibility |
|---|---|
| `internal/repo` | `as_of` in the ref schema, as an optional key, parsed as a date as `verified` is |
| `internal/lint` | L01 reports `as_of` present and empty; L13 passes over a record of an exchange; L07 checks `as_of` against `verified` on a ref |
| `cmd/archdoc` | `new` takes `--as-of` for a ref and offers `as_of` in the form that asks for the type or title, and its help says so |
| `internal/export` | `as_of` on a document, and its definition in `schema: 3` |
| Published schemas | `v3.json`, which [RFC-0008](0008-questions.md) adds, gains `as_of` on a document |
| `internal/template` | `Data` gains the `as_of` date, and `ref.md` writes it where it is given. In `PROCESS.md`, the description of the ref type, its front matter, and the lint rules on dates and on refs. In the triage guide, the L07 and L13 rows. In the classifying guide, the description of the ref type. In the survey guide, a row classifying a record of an exchange found in a project's history as a ref carrying `as_of`. The copy of `PROCESS.md` in `spec/` follows from `archdoc update` |
| Specification pages | `documents.md`: the ref's front matter. `lint.md`: L07 and L13. `layout.md`: what `ref_stale_days` warns about. `commands.md`: `new`'s substitutions and options, the type prompt's filtering, and the prompt for `as_of` as an exception to the Interaction rule |
| Documentation | `docs/EXPORT.md`: the account of `config`, which a consumer reads to explain when a ref goes stale |

Where the rule registry carries the releases that changed each rule, as [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) proposes, the changes to L01, L07 and L13 are recorded there.

### Front matter

```yaml
---
id: REF-0003
title: Review round on the export shape
verified: 2026-10-12
as_of: 2026-10-09
---
```

| Key | Type | Rules |
|---|---|---|
| `as_of` | date | Optional. ISO 8601. The date of the exchange the ref records. Not empty where present. Not after `verified` |

A ref holds research, comparisons, background material, or the record of an exchange. `verified` stays required on every ref. On a record of an exchange it is the date the record was last checked against its sources, which is the date it was written or later. `as_of` on any other type is a key outside that type's schema, which L01 reports.

### Lint

| Rule | Change |
|---|---|
| L01 | `as_of` present with no value is an error, as a required key left empty is |
| L13 | Does not report a ref whose `as_of` is a date, whatever its `verified` date |
| L07 | Applies to refs as well as RFCs and ADRs: on a ref, where `as_of` is set, it is not after `verified`. An error |

### Export

| Field | Content |
|---|---|
| `as_of` | ISO 8601, refs only, absent where unset |

`as_of` is an optional field of a document. Under [ADR-0019](../adr/0019-export-stability.md) an added optional field does not move `schema`, so `as_of` joins `schema: 3` whether or not a tagged release has emitted it by the time this is built. This amends the document object [RFC-0004](0004-exporting-the-repository-as-json.md) describes and the `v3.json` [RFC-0008](0008-questions.md) introduces, which gain `as_of`. The export does not report a ref's staleness itself; a consumer computing it from `verified` and `ref_stale_days` passes over a document carrying `as_of`.

### `archdoc new`

| Flag | Effect |
|---|---|
| `--as-of <date>` | Writes `as_of` with the date given into the ref `new` creates, as in the front matter above |

| Rule | Detail |
|---|---|
| Type | Refused for an RFC or ADR, which have no `as_of`. Where the type is prompted for, `--as-of` leaves only a ref to choose |
| `--backfill` | Refused with `--as-of`, since a ref cannot be backfilled |
| Date | Refused unless an ISO 8601 date, and refused after today, compared as a date against today's date, since `new` writes today's date to `verified` and L07 reports an `as_of` after it |
| When | Checked before anything is prompted for, as every other flag of `new` is |
| Prompts | Where `--as-of` is not given and `new` prompts for the type or the title, `as_of` is a field of the same form, after them, pre-filled empty, and shown only once the type is a ref. Left empty, the ref is created without `as_of`. A date given there is refused as the flag's is. It does not open a form of its own: given the type and title, `new` creates the ref without `as_of` unless `--as-of` is given. This is an exception to the rule that a command prompts for every optional flag whose value is written to a file, because `as_of` is optional and most refs are not records of an exchange |

### Index

`INDEX.md`'s Refs table is unchanged, as `ID | Title | Verified`.

## Alternatives considered

The design rests on [ADR-0004](../adr/0004-the-front-matter-schema-only-gains-optional-keys.md), under which the ref schema gains `as_of` as an optional key, and on [ADR-0019](../adr/0019-export-stability.md), under which the field joins `schema: 3`.

- **Leaving L13 as it is.** Lost because every record of an exchange would warn once older than `ref_stale_days`, and clearing it means checking again, every `ref_stale_days`, a record of something that cannot have changed. A warning that is always present on such records comes to be passed over on the refs it exists for.
- **Setting `ref_stale_days` to `0`.** Already possible. Lost because it disables the check for every ref in the repository, including those that go out of date.
- **Never prompting for `as_of`.** Lost because a form already asking for the type or title costs nothing more for the field, and a record of an exchange created through it would otherwise need `new` run again with `--as-of`.
- **Prompting for `as_of` whenever `--as-of` is not given.** Lost because `as_of` is optional and most refs are not records of an exchange, so `new ref` with a title would open a form for a field almost always left empty.
- **A marker, `kind: record`.** Lost because it marks the ref without carrying the date the record describes, and a key named `kind` beside a document's type, which its directory gives, reads as a second type.

## Backwards compatibility

A ref without `as_of` is unaffected, and L13 reports it as before. Under any earlier release a ref carrying `as_of` fails L01, as a key outside the ref schema, so a repository can carry one only once the release it pins carries this design.

A consumer validating against `v3.json` as it stood, which allows no field it does not name, rejects a document carrying `as_of`, whether that `v3.json` was released or not. `v3.json` is regenerated from the amended embedded schema, and the test comparing the schema's declared properties against the exported types finds the field on both sides.

`new` without `--as-of` writes a ref as before. Where it prompts for the type or title, the form also asks for `as_of`, and left empty writes the ref as before. The lint fixtures gain a record of an exchange, a ref whose `as_of` is after its `verified`, and a ref with `as_of` present and empty, and the tests of `new` gain `--as-of` on a ref, on an RFC, with `--backfill`, with a date that is not ISO 8601, and with a date after today. No ref in this repository carries `as_of`, so its export is unchanged.

## Open questions

## Changelog

- 2026-10-19: Motivation states the need without naming a tool that writes records.
- 2026-10-19: `new` asks for `as_of` in the form that asks for the type or title, a ref carrying it is called a record of an exchange, and the survey guide and the rule registry [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) proposes are named among what changes.
