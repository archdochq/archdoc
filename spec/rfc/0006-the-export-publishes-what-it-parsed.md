---
id: RFC-0006
title: The export publishes what it parsed
status: accepted
created: 2026-09-20
decided: 2026-09-20
depends: []
updates: [RFC-0004]
obsoletes: []
---

# RFC-0006: The export publishes what it parsed

## Abstract

`archdoc export` replaces its `sections` object with `contents`, a flat ordered array of every heading and the prose beneath it. Links gain a type, the envelope gains the revision it describes and the repository's configuration, and `--out` removes the files a previous export wrote and this one does not. The shape moves to `schema: 2`, amending the rule in [RFC-0004](0004-exporting-the-repository-as-json.md) that it only ever gains optional fields.

## Motivation

The export publishes everything ArchDoc derives and almost nothing it parses. Every body is already walked into an ordered list of headings carrying levels, anchors and line numbers, and that walk is discarded. What reaches the JSON is an object keyed by anchor holding level two headings only, each carrying its subsections' prose inside its own.

A consumer rendering a document therefore parses the Markdown a second time, because the export does not tell it what order the document is in. A JSON object has no order, and a Go map marshals with its keys sorted, so a document written Abstract, Motivation, Proposal arrives as `abstract`, `alternatives-considered`, `motivation`. Nor does the export mention any heading below level two, so a page structured entirely in level three headings exports no structure at all, and neither a table of contents nor a link to a subsection can be built from the data.

Re-parsing is not only duplicated effort. Where a section ends depends on fenced blocks and HTML comments having been masked first, which is why the body walk exists and why an empty section is judged from masked text rather than from the text as written. A consumer splitting a body by heading gets that wrong the first time a document contains a `##` inside a fenced block, and nothing reports it. The rule for splitting a document is the tool's knowledge, and publishing the result is the same undertaking as publishing the identifier rules and the relationship graph.

Three narrower gaps have the same character. An image is emitted as an ordinary link, so a consumer rewriting destinations cannot tell one that becomes an `<img>` source from one that becomes an anchor, and a list of a document's outgoing links includes its diagrams. Nothing records which revision an export describes, so a consumer cannot state what it is showing or decide whether a re-ingest is due. `--out` writes files and removes none, so a document deleted from the repository is served from the exported tree for as long as that tree survives.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Contents | A document as a flat ordered list of its headings |
| Entry | One heading: its level, text, anchor, line and body |
| Body | The lines from a heading to the next heading of any level, exactly as written |
| Source | A document as it is on disk, front matter included |
| Prune | Removing a file a previous export wrote and this one does not |

### Components

| Component | Responsibility |
|---|---|
| `internal/repo` | Walks a body into headings, which the export publishes rather than discards |
| `internal/export` | The types, `Build`, and the embedded schema |
| `archdoc export` | Writes one document, a tree, or the schema |

### The envelope

```json
{
  "schema": 2,
  "commit": "49a14a5c2d3f9ac640bf7db8d88208254b9af105",
  "config": {
    "name": "ArchDoc",
    "branch": "main",
    "root": ".",
    "strict": true,
    "ref_stale_days": 180
  },
  "has_glossary": true,
  "documents": [],
  "glossary": []
}
```

| Field | Content |
|---|---|
| `schema` | `2` |
| `commit` | The revision given by `--commit`, absent otherwise |
| `config` | Every setting `archdoc.json` holds |
| `has_glossary` | Whether the repository has a glossary page |
| `documents` | Every document |
| `glossary` | The parsed glossary, absent when the page is |

`name` moves into `config` and is no longer a field of its own. `config` is the file's five settings and not the loaded struct, so the absolute path the configuration was read from is not among them: it describes the machine that ran the command rather than the repository.

`has_glossary` is present in every export. `glossary` answers the same question in the single-document form and is deliberately absent from `index.json`, so pruning, which reads `index.json` to learn what the last run wrote, needs a field that survives there.

### Contents

```json
"contents": [
  { "level": 1, "text": "RFC-0001: Database", "anchor": "rfc-0001-database",
    "line": 12, "body": "\n" },
  { "level": 2, "text": "Abstract", "anchor": "abstract",
    "line": 14, "body": "\nStore data in a database.\n" },
  { "level": 3, "text": "The rule", "anchor": "the-rule",
    "line": 18, "body": "\nOne paragraph.\n" }
]
```

| Rule | Reason stated in the design |
|---|---|
| Entries appear in document order | The order is the document's; an object keyed by anchor cannot carry it |
| Every heading appears, whatever its level | A table of contents and a link to a subsection both need the headings the export omitted |
| `level` is carried rather than inferred from nesting | Heading levels need not descend one at a time, and a document going from level one to level three would render at a level its author did not write |
| A body runs to the next heading of any level | No prose appears twice, and nesting is read from `level` |
| A body is the lines as written, untrimmed | `line` plus one is the first line of `body`, so a position inside an entry needs no further arithmetic |
| Content before the first heading is not carried | It belongs to no heading. `--source` is what recovers it |
| `anchor` is unique within a document | A repeated heading takes a numbered suffix, as it already does |

`required_sections` is unchanged. It names the level two headings a type calls for, which is what a section is everywhere else in this specification, and a contents entry is a different unit.

### Links

Each entry gains `type`.

| Type | Written as |
|---|---|
| `inline` | `[text](destination)` |
| `image` | `![text](destination)` |
| `definition` | `[label]: destination` |

A definition's `text` is its label. Lint already checks an image's destination resolves, because it reads the same list, so this changes what the export says and not what lint reports.

### Source

`--source` adds `source` to each document: the file exactly as it is on disk, front matter included. It is the only field that reproduces the document, and it is the only place the front matter appears as bytes rather than as parsed fields.

Nothing carries a document's prose more than once. The three modes are:

| Invocation | Prose carried |
|---|---|
| `archdoc export` | Each contents entry's body |
| `archdoc export --no-bodies` | None. Every heading, anchor and line survives |
| `archdoc export --source` | Each entry's body, and the file itself |

`index.json` carries neither bodies nor source, whatever was asked.

### `--commit`

`--commit <revision>` records the revision the export describes, as the envelope's `commit`. The value is taken as given and is not validated against a repository.

The command asks git nothing. A revision read from git would be attached to a working tree that may hold uncommitted edits, stamping the output with a revision that does not describe its contents, which a consumer could not detect. The caller that wants provenance is continuous integration, which holds the revision already.

### `--out`

The tree is unchanged: `index.json` carrying every document without bodies, one file per document beside it, and `glossary.json` when there is a glossary.

Pruning proceeds:

1. Read `index.json` in the target directory. Where it is absent, or does not parse as an ArchDoc export, write the tree and report that nothing was pruned.
2. Derive the previous run's files from it: `index.json`, one `<path>.json` per document it lists, and `glossary.json` where it recorded a glossary.
3. Write this run's tree.
4. Remove each previous file this run did not write, and print each removal.

The command therefore only ever removes a file a previous ArchDoc export wrote into that directory. A mistyped destination removes nothing, because it holds no index to read.

### Errors

| Condition | Result |
|---|---|
| No repository, without `--schema` | The usual not-found error |
| `--out` names an unwritable directory | The write error |
| A previous file cannot be removed | The removal error, after the tree is written |

### Out of scope

- **A document's former identifiers.** `renumber` rewrites every reference inside the repository and none outside it, so a consumer holding published URLs has no redirect to follow. Recording them means the tool keeping history rather than publishing what it already knows, which is a larger design than this one.
- **Reference uses.** `[text][label]` carries a label and not a destination, so a consumer that rewrites definitions carries every use with them. Extracting uses would also make lint report a broken reference link twice, at the use and at the definition.
- **Anything before the first heading, as a field of its own.** It is a line ending in every document that exists, and `--source` recovers it where it is not.
- **Reading the revision from git.** See `--commit`.
- **Rendering.** Unchanged from [RFC-0004](0004-exporting-the-repository-as-json.md): there is no renderer.

## Alternatives considered

- **Shipping `contents` beside `sections`.** The shape would only gain fields and no consumer could break. Every section's prose would then be emitted twice, exclusive under one key and inclusive under the other, and two encodings of one fact are what the index's implemented column and `ImplementedIn` exist to prevent. Nothing consumes the export, which is published six days before this document, so the cost of breaking it is bounded and this is the last point at which it is.
- **An `order` field on each section, keeping the object.** It answers the ordering question and not the others: level three headings stay invisible, so a table of contents is still unavailable. Combined with an outline it would be a second encoding of the same order.
- **Nesting contents instead of carrying `level`.** A tree is what most consumers build, and the tool would build it once rather than each of them building it. Heading levels need not descend one at a time, and a document going from level one to level three has no honest depth: either the level three heading sits at depth two and renders as something its author did not write, or depth and level disagree and a consumer must choose between them. A tree is derivable from the list and the list is not always derivable from the tree.
- **Keeping the existing `body`.** It is neither the structure, which `contents` now carries, nor the document, which it cannot reproduce because it begins after the front matter. `source` is the useful half of it.
- **Pruning every file the run did not write.** One rule, and a mistyped `--out` deletes a directory the user did not mean to name.
- **Refusing `--out` unless the directory is empty.** Safe, and it breaks exporting into a build directory that already exists, which is the ordinary arrangement.

The design rests on [ADR-0005](../adr/0005-the-dependency-list-is-closed.md), which is why the schema is checked against the exported structs by reflection rather than by validating output against it, and on [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md), which is why bodies are raw Markdown.

## Backwards compatibility

The shape breaks. `sections` is removed, `body` is removed, and `name` moves into `config`, so a consumer reading any of them stops working. `schema` moves to `2`, which is what it is for: it "changes only for a break, which the rule above is meant to prevent ever being necessary".

That rule is amended rather than abandoned. The shape gains only optional fields from `schema: 2` onwards, and this break is taken once, while the export is six days old and nothing reads it. A second break would be a decision of a different kind, shaping every published shape rather than this one, and belongs in an ADR rather than in an RFC amending a predecessor.

`sections` is removed rather than left to rot because it is not a smaller version of `contents` but a worse one: level two only, unordered, and carrying its subsections' prose inside its parent.

The command surface only gains flags. `--out`, `--no-bodies` and `--schema` keep their names and meanings, and `--source` and `--commit` are new. `--out` removing files is a behaviour change and is the one part of this that is not additive at the command line.

## Open questions

## Changelog
