---
id: RFC-0012
title: The rejection rationale is structured data
status: accepted
created: 2026-10-18
decided: 2026-10-18
depends: [ADR-0019, ADR-0020, ADR-0021]
updates: [RFC-0004, RFC-0006, RFC-0008, RFC-0009]
obsoletes: []
---

# RFC-0012: The rejection rationale is structured data

## Abstract

Every level two section titled `Rejection rationale` is published in the export as `rejection_rationale`, each with its heading's anchor and line and its prose as written, and is left out of the document's contents. `--all-headings` restores it to `contents` with the question sections and the `Changelog`, and the change joins the break `schema: 3` already makes.

## Motivation

A rejected document ends with `Rejection rationale`, which records why the proposal was turned down. In the export it is a heading in `contents` like any other, among the sections a consumer shows as the design. Under [ADR-0021](../adr/0021-the-rejection-rationale-is-published-as-structured-data.md) the section is published as structured data and leaves `contents`, in the same release as the question sections and the `Changelog`.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| `Rejection rationale` section | A level two heading `Rejection rationale` and every line up to the next heading of level one or two, as the tool reads headings. A heading of level three to six within it is part of it |
| Body | The section's lines after its heading, as written, subsections included |

### Components

| Component | Responsibility |
|---|---|
| `internal/export` | The `rejection_rationale` array on each document, read from the document's sections of that title, `contents` without those sections unless every heading is asked for, and its definition in `schema: 3`, with the schema's and the types' own descriptions of the document, `contents` and `required_sections`, and of the options omitting bodies and restoring headings |
| `cmd/archdoc` | `--all-headings` restores the section with the question sections and the `Changelog`, and its help says so. `index.json` under `--out` drops each section's body, copying the array before it does, as it copies `changelog` |
| Published schemas | `v3.json`, which [RFC-0008](0008-questions.md) adds, gains `rejection_rationale_section` and `rejection_rationale` on a document, and its descriptions of the document, `contents` and `required_sections` name the section |
| Specification pages | `export.md`: the flags, the shape, and what `index.json` omits under `--out` |
| Documentation | `docs/EXPORT.md`: the flags and the shape; the account of `required_sections`, whose example renders a section missing from `contents` as not written, and which now has to read a rejected document's rationale from `rejection_rationale`; and the account of version 3 |

### Export

```json
"rejection_rationale": [
  {
    "anchor": "rejection-rationale",
    "line": 41,
    "body": "\nA second index of the same documents would drift from `INDEX.md`, which already lists them.\n"
  }
]
```

| Rule | Detail |
|---|---|
| `contents` | Leaves out every `Rejection rationale` section: its heading and every heading beneath it, with their bodies, whatever the document's status or type |
| `--all-headings` | `contents` carries the section as written, with every other heading. `rejection_rationale` is unchanged |
| Presence | `rejection_rationale` is written on every document, empty where there is no such section. A rejected document's `required_sections` still names `Rejection rationale`, with the anchor of its heading where there is one, and the section is present exactly where `rejection_rationale` is not empty |
| Sections | `rejection_rationale` holds every such section, in document order |
| Bodies | `body` holds the section's body, so `line` plus one is its first line. It is absent wherever contents bodies are: under `--no-bodies` and in `index.json` |
| What it holds | What was parsed, whatever L08 and L14 report, including the prompt `reject` appends where it has not been replaced |
| Schema | One definition, `rejection_rationale_section`, is added to `schema: 3`, requiring `anchor` and `line`, with `body` optional. A document requires `rejection_rationale`, an array of `rejection_rationale_section` |

No tagged release has emitted `schema: 3`, and under [ADR-0021](../adr/0021-the-rejection-rationale-is-published-as-structured-data.md) the section leaves `contents` in the release that carries it, so under [ADR-0019](../adr/0019-export-stability.md) the change is part of the break [RFC-0008](0008-questions.md) and [RFC-0009](0009-changelog.md) already make, and `schema` stays at `3`.

This amends [RFC-0006](0006-the-export-publishes-what-it-parsed.md), [RFC-0008](0008-questions.md) and [RFC-0009](0009-changelog.md), under which `contents` is every heading outside the question sections and the `Changelog`: it is every heading outside those and `Rejection rationale`, and every heading under `--all-headings`. It amends the document object [RFC-0004](0004-exporting-the-repository-as-json.md) describes, which gains `rejection_rationale`.

### Out of scope

- **`Sources`.** [ADR-0020](../adr/0020-structured-sections.md) names it beside `Rejection rationale`, and it stays in `contents` until a decision names it.

## Alternatives considered

The design rests on [ADR-0019](../adr/0019-export-stability.md), under which the change joins the break `schema: 3` already makes; on [ADR-0020](../adr/0020-structured-sections.md), under which a section recording a document's process leaves `contents` once a decision names it; and on [ADR-0021](../adr/0021-the-rejection-rationale-is-published-as-structured-data.md), which names `Rejection rationale` and keeps the requirement L08 checks.

No other alternatives were weighed.

## Backwards compatibility

The export breaks in the release that carries [RFC-0008](0008-questions.md), at the same `schema: 3`. A consumer of `schema: 2` that renders `contents` stops receiving a rejected document's `Rejection rationale` unless the export is run with `--all-headings`, and finds `Rejection rationale` named in a rejected document's `required_sections` while absent from `contents`. A consumer of `schema: 3` built before this design, validating against `v3.json` as it stood, rejects the new output, and one that does not validate stops receiving the rationale in `contents` with no change of number.

A `Rejection rationale` section on any other document also leaves `contents` and is published in `rejection_rationale`. L08 reports it on an RFC or ADR that is not rejected, and on a ref only where it follows `Sources`, which must be the last level two heading. Nothing reports it on a spec page.

No document in this repository is rejected, so the export of `spec/` gains only an empty `rejection_rationale` on each document. The test comparing the schema's declared properties against the exported types checks the types in its own table, which gains `rejection_rationale_section` and its type, and the test asserting that array fields are never null gains it on a document. `v3.json` is regenerated from the amended embedded schema.

## Open questions

## Changelog
