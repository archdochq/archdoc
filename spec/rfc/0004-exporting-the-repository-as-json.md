---
id: RFC-0004
title: Exporting the repository as JSON
status: accepted
created: 2026-09-14
decided: 2026-09-14
backfilled: 2026-09-16
depends: [ADR-0003, ADR-0004, ADR-0005, ADR-0006]
updates: []
obsoletes: []
---

# RFC-0004: Exporting the repository as JSON

## Abstract

A command that writes every document, its parsed sections, its resolved links and the relationships ArchDoc derives, as JSON conforming to a published schema, so a consumer renders or indexes the repository without reimplementing the front matter schema, the identifier rules or the repository walk.

## Motivation

The index publishes the derived graph, as Markdown links inside table cells. A site that renders a specification has to reconstruct every relationship from those cells or from the documents, and has to parse Markdown to rewrite links into its own URL scheme. Both are the tool's own logic, duplicated by every consumer.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Export | The whole repository as one JSON object |
| Document | One RFC, ADR, spec page or ref, with its front matter, derived fields, sections, links and body |
| Section | One H2 and everything under it, keyed by anchor |
| Link | A relative Markdown link with what it resolves to |
| Term | One glossary entry as data |
| Schema | The JSON Schema the export conforms to, versioned by an integer |

### Components

| Component | Responsibility |
|---|---|
| `internal/export` | The types, `Build`, and the embedded schema |
| `internal/export/schema.json` | The schema, embedded |
| `schema/export/v1.json` | The schema, published, generated from the embedded copy |
| `archdoc export` | Writes one document, a tree, or the schema |

### The export

```json
{
  "schema": 1,
  "name": "<archdoc.json name>",
  "documents": [ ... ],
  "glossary": [ ... ]
}
```

### A document

| Field | Content |
|---|---|
| `type`, `id`, `number`, `page`, `path`, `title`, `status` | From discovery and front matter; `id` and `number` absent for a spec page, `page` absent otherwise, `status` absent where the type has none |
| `created`, `decided`, `backfilled`, `verified` | ISO 8601, each absent where unset |
| `depends`, `updates`, `obsoletes`, `includes` | As written |
| `updated_by`, `obsoleted_by`, `depended_on_by`, `included_in` | Derived, as [ADR-0003](../adr/0003-reverse-relationships-are-derived.md) describes |
| `effectively_obsolete`, `implemented`, `stale` | Derived |
| `required_sections` | The sections the type calls for, in order, each as `{title, anchor}` |
| `sections` | An object keyed by anchor, each `{title, body}` |
| `body` | Raw Markdown |
| `links` | Each `{text, destination, path, resolves_to, anchor, line}` |

### Rules

| Rule | Reason stated in the design |
|---|---|
| `sections` is keyed by anchor | A heading may repeat; the anchor walk numbers repeats, so nothing is lost |
| No section records a level | Every section is an H2 |
| A required section may be absent from `sections` | Lint checks them; a draft part-way through is the ordinary case. The anchor is still given |
| Every array field is an array, never `null` | A consumer handles one shape |
| A link's `path` and `resolves_to` are absent for a destination with a URI scheme or a leading `/` | It is not in the repository |
| Bodies are raw Markdown | There is no renderer, under [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md) |
| The output records no generation time | It is deterministic, cacheable and diffable |
| The shape only ever gains optional fields | The principle of [ADR-0004](../adr/0004-the-front-matter-schema-only-gains-optional-keys.md), applied to a second published shape |

### The glossary

Emitted twice: as a spec page among the documents, and as `glossary`, an array of `{name, anchor, definition, formerly}`.

### `--out`

```
<dir>/
├── index.json           every document, without bodies
├── glossary.json        when there is a glossary
└── <path>.json          one per document, with bodies
```

`index.json` omits bodies whatever else was asked. Each document's file sits at its own path with `.md` replaced by `.json`.

### `--no-bodies`

Omits `body` and every section's `body`, everywhere.

### `--schema`

Prints the embedded schema and exits, without a repository.

### The schema

Its `$id` is the URL of the published copy. A test compares the published copy against the embedded one byte for byte, and a test compares the schema's declared properties against the exported structs by reflection in both directions. Under [ADR-0005](../adr/0005-the-dependency-list-is-closed.md) no validator is added, so the output is not validated against the schema; the reflection check catches a field added on one side and not the other.

### Errors

| Condition | Result |
|---|---|
| No repository, without `--schema` | the usual not-found error |
| `--out` names an unwritable directory | the write error |

### Out of scope

- Rendering Markdown to HTML. No renderer, and the consumer's sanitisation is the consumer's.
- Filtering by type or status. `jq` over the single-document form does it, and `--out` gives a file per document.
- Validating output against the schema. A validator is a dependency.

## Alternatives considered

This design rests on [ADR-0003](../adr/0003-reverse-relationships-are-derived.md), which is what makes the export worth having, on [ADR-0004](../adr/0004-the-front-matter-schema-only-gains-optional-keys.md) as the compatibility rule for the shape, on [ADR-0005](../adr/0005-the-dependency-list-is-closed.md), which rules out a validator, and on [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md), which rules out rendering.

- **`sections` keyed by title.** A repeated heading would overwrite one section with the other and nothing would notice.
- **A single JSON document only.** A page rendering one RFC would fetch the whole specification. `--out` gives a small index and a file per document.

No other alternatives were weighed.

## Backwards compatibility

Nothing breaks. The command is new, and `repo.Anchor` is an addition.

## Open questions

## Changelog

## Sources

- [72df287] adds the command, the package and the embedded schema, and sets `created` and `decided`.
- [285333b] publishes the schema outside `internal/` and adds `docs/EXPORT.md`, on 2026-09-16.
- `docs/ARCHDOC.md` at [72df287a], section `archdoc export`, describes the design.
- The alternatives were weighed in a design session on 2026-09-14, not publicly available, and first written down on 2026-09-16.

[72df287]: https://github.com/ollieread/archdoc/commit/72df287c4fdbc57afb1cb03775b1f4a0f4f99da8
[285333b]: https://github.com/ollieread/archdoc/commit/285333bbc7927b5e12e5707a4e661d056fa8cc2e
[72df287a]: https://github.com/ollieread/archdoc/blob/72df287c4fdbc57afb1cb03775b1f4a0f4f99da8/docs/ARCHDOC.md
