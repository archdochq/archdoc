---
title: Export
includes: [RFC-0004, RFC-0006]
---

# Export

`archdoc export` writes the repository as JSON on stdout. `--out <dir>` writes a tree of files instead. `--no-bodies` omits every contents body, leaving the outline. `--source` adds each document's file as it is on disk. `--commit <revision>` records the revision the export describes. `--schema` prints the JSON Schema the output conforms to and needs no repository.

## Shape

The output is an object with `schema`, currently `2`; `config`, every setting `archdoc.json` holds; `has_glossary`; `documents`, an array; `glossary`, an array present when the glossary page exists; and `commit`, present only when `--commit` was given. The revision is taken as written and is not checked against a repository. `has_glossary` is present even where `glossary` is not, because `index.json` carries no glossary and a consumer reading only that still has to know whether one exists.

Each document carries its front matter, the derived reverse relationships (`updated_by`, `obsoleted_by`, `depended_on_by`, `included_in`), the derived facts (`effectively_obsolete`, `implemented`, `stale`), `required_sections`, `contents` and `links`, and `source` where it was asked for. Dates are ISO 8601 strings. Every array field is an array and never null.

`required_sections` lists the H2 headings the type calls for, in order, as objects with `title` and `anchor`. Any of them may be absent from `contents`; the anchor given for an absent one is the anchor it would have had.

`contents` is every heading in document order, each with `level`, `text`, `anchor`, `line` and `body`. An entry's body runs to the next heading of any level, so no entry holds another's prose, and it is the lines as written, so `line` plus one is its first line. A body is absent where bodies were excluded and empty where one heading directly follows another. `level` is carried rather than implied by position, because heading levels need not descend one at a time. Anything before the first heading belongs to no entry and appears only in `source`.

A contents entry is not a section. A section is an H2 and everything under it, subsections included, which is what the required sections are.

Each entry in `links` has `text`, `type`, `destination` as written, `line`, and where the destination is relative and stays within the repository, `path` resolved against the repository root and `resolves_to` naming the identifier or page there, with `anchor` where one was written. A destination carrying a URI scheme or beginning with `/` has neither `path` nor `resolves_to`. `type` is `inline` for `[text](destination)`, `image` for `![text](destination)` and `definition` for `[label]: destination`, whose `text` is its label. A reference use carries a label rather than a destination and is not listed: rewriting the definition carries its uses with it.

Each glossary term has `name`, `anchor`, `definition`, and `formerly` where the term was renamed.

Bodies are raw Markdown. Nothing is rendered. `source` is the file exactly as it is on disk, front matter included, and is the only field that reproduces the document.

## `--out`

```
<dir>/
├── index.json          every document, without bodies
├── glossary.json       when there is a glossary
└── <document path>.json   one per document, with bodies
```

`index.json` omits bodies and source whatever else was asked, keeping the full outline. Each document's file sits at the document's own path with `.md` replaced by `.json`. Every path written is printed on stdout.

The command also removes what an earlier export left and this one does not write. It reads the `index.json` already in the directory before overwriting it, and takes the previous run's files from it: `index.json` itself, one file per document it lists, and `glossary.json` where it recorded a glossary. Each removal is reported on stderr, leaving stdout the list of files that now exist.

It therefore only ever removes a file an ArchDoc export wrote there. Where the directory holds no `index.json`, or holds one that is not an ArchDoc export, nothing is removed and the command says so. A path in that index that is not local to the directory is ignored: the index is input, and input naming somewhere else must not reach a delete. Directories left empty are not removed.

## Schema

The schema is embedded in the binary, published at `schema/export/v2.json` in the source repository, and printed by `--schema`. `v1.json` stays published: it describes output that was real, and its URL is the one any consumer of that shape holds. The published copy is generated from the embedded one and a test compares them byte for byte. Its `$id` is the published copy's URL. A test compares the schema's declared properties against the exported structs by reflection. From `schema: 2` onwards the shape only ever gains optional fields.

The output is deterministic and records no generation time.
