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

`Changelog` becomes an optional section whose entries are read as data: each entry is a list item beginning `- `, optionally dated `YYYY-MM-DD: `, and holds everything CommonMark takes as part of that item. An undated entry still parses, with no date, and a new rule, L21, reports the missing date with an error while the document is editable and a warning once it is frozen. The export publishes the entries as structured data and leaves the section out of the document's contents.

## Motivation

A Changelog records the edits made to an RFC while it is proposed, each as a dated line. The tool reads it only to check that the section is present and in order: the dates are text, their order is unchecked, and an entry dated before the document existed passes. Under [ADR-0020](../adr/0020-structured-sections.md) the section is published as structured data and leaves the export's contents, which needs entries the tool can read.

Entries already written are frozen with the documents that carry them. In this repository [RFC-0007](0007-the-glossary-is-a-type-of-its-own.md) records two, each a list item opening with its date, and that form is the one a reader of its export has to be given.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Entry line | Within a `Changelog` section, outside a fenced code block and an HTML comment, a line whose first non-blank character is `-`, indented by at most three spaces and by less than the content column of any entry it follows, with a space, a tab or the end of the line after the `-` |
| Content column | The column of an entry line's first character after the `-` and the one to four spaces that follow it. Where five or more spaces follow, or nothing does, it is the column two after the `-` |
| Entry | An entry line, and every line CommonMark takes as part of that list item, as set out under Entries |
| Shaped as a date | Four digits, `-`, two digits, `-`, two digits, directly after the whitespace following an entry's `-` |
| Date | Something shaped as a date, followed by `: `, that is a real calendar date. An entry without one is undated |
| Text | An entry's lines as written, with its `-`, the whitespace up to the content column and its date removed from the first, up to the content column of leading whitespace removed from each later line, and blank lines at the end dropped |

### Components

| Component | Responsibility |
|---|---|
| `internal/repo` | An RFC's required sections no longer include `Changelog`. Reads a document's Changelog entries, their dates and text, and the faults in their shape. L21, the lifecycle gate and the export all read these |
| `internal/lint` | L21, which reports those faults. L14 no longer exempts `Changelog` |
| `internal/lifecycle` | `accept` and `reject` refuse a document with any of those faults, and compare its entries' dates with the `decided` date they are about to write |
| `internal/export` | The `changelog` array on each document, `contents` without the `Changelog` section unless every heading is asked for, and its definition in `schema: 3` |
| `cmd/archdoc` | `--all-headings` restores the `Changelog` with the question sections, and `index.json` under `--out` drops each entry's text, copying the array before it does, as it copies `contents` |
| `internal/template` | The RFC template's comment in `Changelog`, which gives the entry form and says the section is optional; in `PROCESS.md`, the list of RFC sections, the `accept` and `reject` sentence and the list of what lint enforces; the same sentence in the authoring and working guides, and in the authoring guide its list of RFC sections and its account of the sections exempt from emptiness; and in the triage guide, its L14 row, the list of rules whose findings soften once frozen, and a new L21 row. The copies `archdoc update` writes, and every published copy of the guides, follow from these |
| Specification pages | `markdown.md`: the RFC's required sections and Changelog entries. `lint.md`: L14's exemptions, L21, and the rules whose findings soften. `lifecycle.md`: the gate. `export.md`: the shape |
| Documentation | The rule count in `README.md`; in `docs/DECISIONS.md`, the list of softened rules and the description of what `accept` and `reject` refuse; and the shape in `docs/EXPORT.md` |

### `Changelog` is optional

An RFC no longer requires `Changelog`. With [RFC-0008](0008-questions.md), its required sections are `Abstract`, `Motivation`, `Proposal`, `Alternatives considered` and `Backwards compatibility`, in that order. The RFC template keeps the heading, with a comment giving the entry form and saying the section is optional.

L08 no longer checks its presence or its order, and L14 no longer needs to exempt it.

### Entries

```markdown
## Changelog

- 2026-09-02: Moved the question on `index.json` to Resolved
  questions, with its answer.
- 2026-09-03: Checked entry dates against `created`.
```

The section is empty, or it holds entries and nothing before the first but blank lines and HTML comments. Entries are read only within a `Changelog` section, as a construct the tool recognises under [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md)'s provision that the set grows when something needs more; elsewhere a line beginning `- ` remains text. The marker is `-` alone, the one every entry in this repository uses.

An entry is a list item, and holds everything CommonMark, which GitHub follows, takes as part of that item. The tool has no Markdown parser under [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md), so the extent is stated here as rules for it to follow. After an entry line, a line belongs to the entry when:

| Line | Belongs to the entry |
|---|---|
| Indented to at least the content column | Yes, whether or not blank lines come before it |
| Blank | Where a later line belongs to the entry |
| Non-blank, indented less than the content column, directly after a line of the entry's paragraph text | Yes, as a lazy continuation, unless it starts a block that interrupts a paragraph: an entry line or any other list item, a heading, a fenced code block, a thematic break, a blockquote, or an HTML block that can interrupt a paragraph |
| Any other line | No, and the entry ends before it |

Wrapped text, further paragraphs, a nested list, a fenced or indented code block and a link reference definition within the item all belong to the entry. A nested item is text within its entry, whatever it holds, so a date written in one is neither read nor checked. Its text is published as written, so it renders for a consumer as it does within the list item.

An entry carries a date where it has one. An undated entry still parses, with no date and the whole of its text, so an entry in a frozen document that was written without a date is published rather than lost. An entry beginning with something shaped as a date that is not followed by `: `, or that is not a real date, is read as undated, and its text keeps what was written.

Entries are in ascending order of date. Each date falls on or after the document's `created`, where it has a valid one, and on or before its `decided`, where it has one.

Reading an entry differs from rendering it in places:

- A line beginning `*`, `+` or a number renders as a list item, and is not an entry line. It ends the entry before it and belongs to no entry.
- A heading indented by one to three spaces within an entry renders inside the list item, and the tool reads it as a heading that ends the section, which L21 reports.
- A fence indented by four or more spaces within an entry renders as one and is not read as one, so a comment marker inside it opens a comment for the tool, which L21 reports where it does not close within the entry.

### L21

L21 checks every `Changelog` section in an RFC, ADR, spec page or ref. It does not check a withdrawn document, as L14 and L20 do not: a withdrawn document is abandoned, and `withdraw` performs no content checks, so a fault in one would stand once it is frozen.

L21 takes its severity from the same function as L16, L17 and L20: an error while the document is editable, whatever its status, and a warning once it is frozen. A Changelog is part of the document's own body, which is what [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md) makes a warning of once no permitted edit could clear the fault. A date is therefore required of every entry a document's author can still edit, and an undated entry in a frozen document is reported as a warning and published undated.

| Finding | Line |
|---|---|
| The section holds something that belongs to no entry | The first line of each run of such lines that is not blank or a comment |
| A heading indented by one to three spaces ends the section | That heading |
| An HTML comment or fenced code block opened in an entry does not close before the next entry line or the end of the section | The line that opens it |
| An entry has no text | The entry |
| An entry with text has no date | The entry |
| An entry begins with something shaped as a date that is not followed by `: ` | The entry |
| An entry begins with something shaped as a date that is not a real date | The entry |
| An entry's date is earlier than the date of the nearest dated entry before it | The entry |
| An entry's date is before the document's valid `created`, or after its `decided` | The entry |
| The document carries more than one `Changelog` section | Each heading after the first |
| A level two heading differs from `Changelog` only in case, spacing or punctuation | That heading |

The check against `created` and `decided` is skipped where `decided` comes before `created`, which L07 already reports. A `Changelog` section after the first is reported by its row alone, and its entries are checked as the first section's are, in one order with them.

### Lifecycle

`accept` and `reject` refuse a document with any fault L21 reports, as they refuse one L20 reports, for the same reason: under [ADR-0001](../adr/0001-terminal-documents-are-frozen.md) a terminal document is never modified, and once it is terminal on the configured branch the fault could never be cleared. They compare each entry's date with the `decided` date they are about to write rather than with the document's current one, so an entry dated after the day of the transition is refused. `withdraw` performs no content checks, unchanged.

### Export

Under [ADR-0020](../adr/0020-structured-sections.md), the `Changelog` is published as structured data and left out of `contents`, as the question sections are under [RFC-0008](0008-questions.md). The two are released together, so the shape breaks once, at the `schema: 3` [RFC-0008](0008-questions.md) introduces. Each document gains `changelog`, an array of entries in document order. Each entry carries `line`, its entry line; `date` where it has one; and `body`, its text.

```json
"changelog": [
  {
    "line": 141,
    "date": "2026-09-02",
    "body": "Moved the question on `index.json` to Resolved\nquestions, with its answer."
  },
  {
    "line": 143,
    "date": "2026-09-03",
    "body": "Checked entry dates against `created`."
  }
]
```

| Rule | Detail |
|---|---|
| `contents` | Leaves out the `Changelog` section: its heading and every heading beneath it, with their bodies. An RFC's `required_sections` no longer names `Changelog` |
| `--all-headings` | `contents` carries the `Changelog` as written, with every other heading. `changelog` is unchanged |
| Presence | `changelog` is written on every document, empty where there are no entries. `date` is absent where an entry is undated, as the export leaves out any value it does not have |
| Sections | `changelog` holds the entries of every `Changelog` section, in document order |
| Bodies | An entry's `body` is absent wherever contents bodies are: under `--no-bodies` and in `index.json` |
| What it holds | What was parsed, whatever L21 reports. Anything in the section that belongs to no entry appears only in `source`, and in `contents` under `--all-headings` |
| Schema | One definition, `changelog_entry`, is added to `schema: 3`, requiring `line`, with `date` and `body` optional. A document requires `changelog` |

This amends [RFC-0006](0006-the-export-publishes-what-it-parsed.md) and [RFC-0008](0008-questions.md), under which `contents` is every heading outside the question sections: it is every heading outside the question sections and the `Changelog`, and every heading under `--all-headings`. It amends the document object [RFC-0004](0004-exporting-the-repository-as-json.md) describes, which gains `changelog`.

### Errors

| Condition | Result |
|---|---|
| `accept` or `reject` on a document with a fault L21 reports, or an entry dated after the day of the transition | refusal naming the line and the fault |
| An L21 finding | the document, the line from the findings table, and the fault |

### Out of scope

- **A command that adds an entry.** Writing a dated line is an ordinary edit.

## Alternatives considered

The design rests on [ADR-0001](../adr/0001-terminal-documents-are-frozen.md), whose freeze is why `accept` and `reject` refuse a document L21 reports; on [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md), whose provision for growth admits entries as a construct; on [ADR-0009](../adr/0009-a-lint-error-names-something-the-user-may-change.md), which sets L21's severity on a frozen document, and so lets a frozen document's undated entries stand; on [ADR-0019](../adr/0019-export-stability.md), under which the shape's break is the one `schema: 3` already makes; and on [ADR-0020](../adr/0020-structured-sections.md), which takes the `Changelog` out of `contents`.

- **A heading per entry.** Each entry a heading carrying its date, with the text beneath, as questions are headings under [RFC-0008](0008-questions.md). Lost because entries already written are list items in frozen documents, and could never be read or rewritten.
- **An entry's later lines indented at least two spaces.** ~~Chosen first, with each continuation indented beneath its entry.~~ Struck on 2026-10-14: an entry is a list item, which also runs over unindented lines.
- **Everything up to the next entry line.** ~~Chosen on 2026-10-14, as an entry is a list item.~~ Struck the same day: an entry is everything a Markdown parser takes as part of the list item, so a line the item does not continue, such as an unindented paragraph after a blank line, is not part of it.
- **Reporting a nested item shaped as a dated entry.** It would catch an entry indented by mistake. Lost because a nested item is detail of one change, and a date in it is text, rendered nested as it is read.
- **One line per entry.** Offered on 2026-10-14 to remove the rules continuation lines needed. Lost because an entry is a list item, and a list item may run over several lines.
- **A date required of every entry.** An undated entry would not parse. Lost because a frozen document's entries can never gain a date, so its undated entries would never be published.
- **`Changelog` as a required section, with its presence recorded in the export.** Lost because the requirement guarantees only that an empty heading exists, as a document accepted the day it was proposed has no entries.
- **Leaving out the check of dates against `created` and `decided`.** Lost because the check catches a mistyped year.

No other alternatives were weighed.

## Backwards compatibility

The tool checks a Changelog today only for its presence and its order. Making it optional loosens both: an RFC without one, or with one out of order, was reported by L08 and refused by the gate, and no longer is. The `accept` and `reject` gates gain the refusals described under Lifecycle.

In this repository, the only entries are the two in [RFC-0007](0007-the-glossary-is-a-type-of-its-own.md), both dated within its `created` and `decided`, so `spec/` reports nothing. The tests do not hold:

- Three RFCs in `testdata/repo`, `0003-cache-expiry.md`, `0004-database-component-on-postgresql.md` and `0005-job-queues.md`, carry an entry dated before their `created`, and that fixture is required to produce no errors, so their entries' dates are corrected.
- `testdata/lint/rfc/0007-decided-before-created.md` carries an entry dated after its `decided`, which L21 does not check against inverted dates, so it gains no finding.
- `TestTheDocumentedWorkflowWorksEndToEnd` fills the template's comments with prose, including the one in `Changelog`, and appends a line at the end of the file, inside `Changelog`. Neither belongs to an entry, so lint reports them where the test expects a clean run.
- The lint fixtures gain an L21 case, and the registered rule count rises by one more than [RFC-0008](0008-questions.md)'s, to twenty-one.
- `TestRequiredSectionsSurviveTheirAbsence` expects at least seven RFC sections, and with [RFC-0008](0008-questions.md) and this design there are five.
- The gate's test that a required section missing entirely is refused, even an ungated one, uses `Changelog`, which is no longer required.
- The export's schema is compared against its types, which gain `changelog_entry`.

The export breaks in the release that carries [RFC-0008](0008-questions.md), at the same `schema: 3`. A consumer of `schema: 2` that renders `contents` also stops receiving the `Changelog` heading of every RFC and its entries, unless the export is run with `--all-headings`, and finds `Changelog` gone from an RFC's `required_sections`.

In a repository that upgrades:

- An undated entry, anything in the section that belongs to no entry, an entry out of order or outside the document's dates, or an unclosed comment or fence in an entry is reported with an error while the document is editable, and with a warning once it is frozen.
- Entries written with `*`, `+`, a number or no marker are not read as entries. In a frozen document they are reported as a warning, and leave `changelog` and the default `contents`, appearing in `source` and under `--all-headings`.
- A withdrawn document is not checked.
- A heading misspelling `Changelog`, once reported by L08 as a missing section, is reported by L21.
- A line beginning `- ` outside a `Changelog` section is not read, and nothing about it changes.
- `accept` and `reject` refuse a document they previously moved, where L21 would report it.

## Open questions

## Changelog
