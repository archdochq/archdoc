---
id: RFC-0009
title: Changelog entries are structured data
status: draft
created: 2026-10-14
decided:
depends: [ADR-0001, ADR-0006, ADR-0009, ADR-0019, ADR-0020]
updates: [RFC-0004, RFC-0006, RFC-0008]
obsoletes: []
---

# RFC-0009: Changelog entries are structured data

## Abstract

`Changelog` becomes an optional section whose entries are read as data: a line beginning `- `, optionally dated `YYYY-MM-DD: `, with any continuation indented beneath it. A new rule, L21, checks the entries, reporting a missing date on an editable document and only warning once the document is frozen, so a frozen document's undated entries still parse. The export publishes the entries as structured data and leaves the section out of the document's contents.

## Motivation

A Changelog records the edits made to an RFC while it is proposed, each as a dated line. Nothing reads it except to check that the section is present: the dates are text, their order is unchecked, and an entry dated before the document existed passes. Under [ADR-0020](../adr/0020-structured-sections.md) the section is published as structured data and leaves the export's contents, which needs entries the tool can read.

Entries already written are frozen with the documents that carry them. [RFC-0007](0007-the-glossary-is-a-type-of-its-own.md) records two, each a list item opening with its date, and that form is the one a reader of its export has to be given.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Entry | Within a `Changelog` section, a line beginning `- ` at the start of the line, outside a fenced code block and an HTML comment, and the continuation lines after it |
| Date | `YYYY-MM-DD` followed by `: `, directly after an entry's `- `. An entry without one is undated |
| Continuation line | A non-blank line indented by at least two spaces, after an entry or after another continuation line, with only blank lines and HTML comments between them |
| Text | An entry's lines after its `- ` and its date, with the first two columns of each continuation line removed |

### Components

| Component | Responsibility |
|---|---|
| `internal/repo` | An RFC's required sections no longer include `Changelog`. Reads a document's Changelog entries, their dates and text, and the faults in their shape. L21, the lifecycle gate and the export all read these |
| `internal/lint` | L21, which reports those faults. L14 no longer exempts `Changelog` |
| `internal/lifecycle` | `accept` and `reject` refuse a document with any of those faults |
| `internal/export` | The `changelog` array on each document, `contents` without the `Changelog` section unless every heading is asked for, and its definition in `schema: 3` |
| `cmd/archdoc` | `--all-headings` restores the `Changelog` with the question sections, and `index.json` under `--out` drops each entry's text, copying the array before it does, as it copies `contents` |
| `internal/template` | The RFC template's comment in `Changelog`, which gives the entry form and says the section is optional; in `PROCESS.md`, the list of RFC sections and the list of what lint enforces; in the authoring guide, its list of RFC sections and its account of the sections exempt from emptiness; and in the triage guide, the list of rules whose findings soften once frozen and a new L21 row. The copies `archdoc update` writes, and every published copy of the guides, follow from these |
| Specification pages | `markdown.md`: the RFC's required sections and Changelog entries. `lint.md`: L14's exemptions, L21, and the rules whose findings soften. `lifecycle.md`: the gate. `export.md`: the shape |
| Documentation | The rule count in `README.md`; in `docs/DECISIONS.md`, the list of softened rules and the description of what `accept` and `reject` refuse; and the shape in `docs/EXPORT.md` |

### `Changelog` is optional

An RFC no longer requires `Changelog`. With [RFC-0008](0008-questions.md), its required sections are `Abstract`, `Motivation`, `Proposal`, `Alternatives considered` and `Backwards compatibility`, in that order. The RFC template keeps the heading, with a comment giving the entry form and saying the section is optional.

L08 no longer checks its presence or its order, and L14 no longer needs to exempt it. Every RFC that carries one still passes.

### Entries

```markdown
## Changelog

- 2026-10-20: Moved the question on `index.json` to Resolved
  questions, with its answer.
- 2026-10-21: Checked entry dates against `created`.
```

The section is empty, or it holds entries and nothing else but blank lines and HTML comments. Entries are read only within a `Changelog` section, as a construct the tool recognises under [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md)'s provision that the set grows when something needs more; elsewhere a line beginning `- ` remains text. The marker is `-` alone, the one every existing entry uses.

An entry carries a date where it has one. An undated entry still parses, with no date and the whole of its text, so an entry in a frozen document that was written without a date is published rather than lost. A line that begins with something shaped as a date but not followed by `: ` is reported, and is read as undated.

Entries are in ascending order of date, and each date falls on or after the document's `created` and, where it has one, on or before its `decided`.

### L21

L21 checks every `Changelog` section a document carries, whatever its type. It does not check a withdrawn document, as L14 and L20 do not: a withdrawn document is abandoned, and `withdraw` performs no content checks, so a fault in one would stand once it is frozen.

L21 takes its severity from the same function as L16, L17 and L20: an error while the document is editable, whatever its status, and a warning once it is frozen. A Changelog is part of the document's own body, which is what [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md) makes a warning of once no permitted edit could clear the fault. A date is therefore required of every entry a document's author can still edit, and an undated entry in a frozen document is reported as a warning and published undated.

| Finding | Line |
|---|---|
| A line is not an entry, a continuation line, blank, or an HTML comment | That line |
| An entry has no text | The entry |
| An entry has no date | The entry |
| An entry begins with something shaped as a date that is not followed by `: ` | The entry |
| An entry's date is not a real date | The entry |
| An entry's date is earlier than the date of a dated entry before it | The entry |
| An entry's date is before the document's `created`, or after its `decided` | The entry |

An entry reported as beginning with a malformed date is not also reported as undated. An entry whose date is not a real date is not compared with any other date.

### Lifecycle

`accept` and `reject` refuse a document with any fault L21 reports, as they refuse one L20 reports, for the same reason: under [ADR-0001](../adr/0001-terminal-documents-are-frozen.md) a terminal document is never modified, and once it is terminal on the configured branch the fault could never be cleared. `withdraw` performs no content checks, unchanged.

### Export

Under [ADR-0020](../adr/0020-structured-sections.md), the `Changelog` is published as structured data and left out of `contents`, as the question sections are under [RFC-0008](0008-questions.md). The two are released together, so the shape breaks once, at the `schema: 3` [RFC-0008](0008-questions.md) introduces. Each document gains `changelog`, an array of entries in document order. Each entry carries `line`, the entry's first line; `date` where it has a real one; and `body`, its text.

```json
"changelog": [
  {
    "line": 141,
    "date": "2026-10-20",
    "body": "Moved the question on `index.json` to Resolved\nquestions, with its answer."
  },
  {
    "line": 143,
    "date": "2026-10-21",
    "body": "Checked entry dates against `created`."
  }
]
```

| Rule | Detail |
|---|---|
| `contents` | Leaves out the `Changelog` section: its heading and every heading beneath it, with their bodies. An RFC's `required_sections` no longer names `Changelog` |
| `--all-headings` | `contents` carries the `Changelog` as written, with every other heading. `changelog` is unchanged |
| Presence | `changelog` is written on every document, empty where there are no entries. `date` is absent where an entry has none or its date is not a real one, as the export leaves out any value it does not have |
| Sections | `changelog` holds the entries of every `Changelog` section, in document order |
| Bodies | An entry's `body` is absent wherever contents bodies are: under `--no-bodies` and in `index.json` |
| What it holds | What was parsed, whatever L21 reports. A line in the section that belongs to no entry appears only in `source`, and in `contents` under `--all-headings` |
| Schema | One definition is added to `schema: 3`: an entry, requiring `line`, with `date` and `body` optional. A document requires `changelog` |

This amends [RFC-0006](0006-the-export-publishes-what-it-parsed.md) and [RFC-0008](0008-questions.md), under which `contents` is every heading outside the question sections: it is every heading outside the question sections and the `Changelog`, and every heading under `--all-headings`. It amends the document object [RFC-0004](0004-exporting-the-repository-as-json.md) describes, which gains `changelog`.

### Errors

| Condition | Result |
|---|---|
| `accept` or `reject` on a document with a fault L21 reports | refusal naming the line and the fault |
| An L21 finding | the document, the line from the findings table, and the fault |

### Out of scope

- **A command that adds an entry.** Writing a dated line is an ordinary edit.

## Alternatives considered

The design rests on [ADR-0001](../adr/0001-terminal-documents-are-frozen.md), whose freeze is why `accept` and `reject` refuse a document L21 reports; on [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md), whose provision for growth admits entries as a construct; on [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md), which sets L21's severity on a frozen document, and so lets a frozen document's undated entries stand; on [ADR-0019](../adr/0019-export-stability.md), under which the shape's break is the one `schema: 3` already makes; and on [ADR-0020](../adr/0020-structured-sections.md), which takes the `Changelog` out of `contents`.

- **A heading per entry.** Each entry a heading carrying its date, with the text beneath, as questions are headings under [RFC-0008](0008-questions.md). Lost because entries already written are list items in frozen documents, and could never be read or rewritten.
- **A date required of every entry.** An undated entry would not parse. Lost because a frozen document's entries can never gain a date, so its undated entries would never be published.
- **`Changelog` as a required section, with its presence recorded in the export.** Lost because the requirement guarantees only that an empty heading exists, as a document accepted the day it was proposed has no entries.
- **Leaving out the check of dates against `created` and `decided`.** Lost because the check catches a mistyped date.

No other alternatives were weighed.

## Backwards compatibility

Nothing checks a Changelog today beyond its presence. Making it optional only loosens: an RFC without one was reported by L08 and refused by the gate, and no longer is, and every RFC that has one still passes. The `accept` and `reject` gates gain the refusal described under Lifecycle.

In this repository, the only entries are the two in [RFC-0007](0007-the-glossary-is-a-type-of-its-own.md), both dated within its `created` and `decided`, so `spec/` reports nothing. The tests do not hold:

- Three RFCs in `testdata/repo`, `0003-cache-expiry.md`, `0004-database-component-on-postgresql.md` and `0005-job-queues.md`, carry an entry dated before their `created`, and that fixture is required to produce no errors, so the dates are corrected.
- `testdata/lint/rfc/0007-decided-before-created.md` carries an entry dated after its `decided`, and gains an L21 finding beside the one it exists for.
- The lint fixtures gain an L21 case, and the registered rule count rises by one more than [RFC-0008](0008-questions.md)'s, to twenty-one.
- `TestRequiredSectionsSurviveTheirAbsence` expects at least seven RFC sections, and with [RFC-0008](0008-questions.md) there are five.
- The gate's test that a required section missing entirely is refused, even an ungated one, uses `Changelog`, which is no longer required.
- The export's schema is compared against its types, which gain the entry's definition.

The export breaks in the release that carries [RFC-0008](0008-questions.md), at the same `schema: 3`. A consumer of `schema: 2` that renders `contents` also stops receiving the `Changelog` heading of every RFC and its entries, unless the export is run with `--all-headings`, and finds `Changelog` gone from an RFC's `required_sections`.

In a repository that upgrades:

- An undated entry, a line that is not an entry, or an entry out of order or outside the document's dates is reported with an error while the document is editable, and with a warning once it is frozen.
- A withdrawn document is not checked.
- A line beginning `- ` outside a `Changelog` section is not read, and nothing about it changes.
- `accept` and `reject` refuse a document they previously moved, where L21 would report it.

## Open questions

## Changelog
