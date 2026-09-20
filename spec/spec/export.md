---
title: Export
includes: [RFC-0004]
---

# Export

`archdoc export` writes the repository as JSON on stdout. `--out <dir>` writes a tree of files instead. `--no-bodies` omits document and section bodies. `--schema` prints the JSON Schema the output conforms to and needs no repository.

## Shape

The output is an object with `schema`, currently `1`; `name`, from `archdoc.json`; `documents`, an array; and `glossary`, an array present when the glossary page exists.

Each document carries its front matter, the derived reverse relationships (`updated_by`, `obsoleted_by`, `depended_on_by`, `included_in`), the derived facts (`effectively_obsolete`, `implemented`, `stale`), `required_sections`, `sections`, `links`, and `body`. Dates are ISO 8601 strings. Every array field is an array and never null.

`required_sections` lists what the type calls for, in order, as objects with `title` and `anchor`. Any of them may be absent from `sections`; the anchor given for an absent one is the anchor it would have had.

`sections` is an object keyed by anchor. Each value has `title` and `body`. Every section is an H2 and includes its subsections.

Each entry in `links` has `text`, `destination` as written, `line`, and where the destination is relative and stays within the repository, `path` resolved against the repository root and `resolves_to` naming the identifier or page there, with `anchor` where one was written. A destination carrying a URI scheme or beginning with `/` has neither `path` nor `resolves_to`.

Each glossary term has `name`, `anchor`, `definition`, and `formerly` where the term was renamed.

`body` is raw Markdown. Nothing is rendered.

## `--out`

```
<dir>/
├── index.json          every document, without bodies
├── glossary.json       when there is a glossary
└── <document path>.json   one per document, with bodies
```

`index.json` omits bodies whatever else was asked. Each document's file sits at the document's own path with `.md` replaced by `.json`. Every path written is printed.

## Schema

The schema is embedded in the binary, published at `schema/export/v1.json` in the source repository, and printed by `--schema`. The published copy is generated from the embedded one and a test compares them byte for byte. Its `$id` is the published copy's URL. A test compares the schema's declared properties against the exported structs by reflection. The shape only ever gains optional fields.

The output is deterministic and records no generation time.
