---
title: Export
includes: [ADR-0019, ADR-0020, ADR-0021, RFC-0004, RFC-0006, RFC-0007, RFC-0008, RFC-0009, RFC-0011, RFC-0012]
---

# Export

`archdoc export` writes the repository as JSON on stdout. `--out <dir>` writes a tree of files instead. `--no-bodies` omits every body, of contents entries, questions, answers, Changelog entries and rejection rationale sections, leaving the outline. `--all-headings` keeps the question sections, the `Changelog` and `Rejection rationale` in `contents`. `--source` adds each document's file as it is on disk. `--commit <revision>` records the revision the export describes. `--schema` prints the JSON Schema the output conforms to and needs no repository.

## Shape

The output is an object with `schema`, currently `3`; `config`, every setting `archdoc.json` holds; `has_glossary`; `documents`, an array; `groups`, an array; `glossary`, an array present when the glossary page exists; and `commit`, present only when `--commit` was given. The revision is taken as written and is not checked against a repository. `has_glossary` is present even where `glossary` is not, because `index.json` carries no glossary and a consumer reading only that still has to know whether one exists.

Each document carries its front matter, the derived reverse relationships (`updated_by`, `obsoleted_by`, `depended_on_by`, `included_in`, `groups`), the derived facts (`effectively_obsolete`, `implemented`, `stale`), `required_sections`, `contents`, `questions`, `changelog`, `rejection_rationale` and `links`, and `source` where it was asked for. Dates are ISO 8601 strings. Every array field is an array and never null.

`required_sections` lists the H2 headings the document calls for, in order, as objects with `title` and `anchor`. Any of them may be absent from `contents`; the anchor given for an absent one is the anchor it would have had. A rejected document's `Rejection rationale` is absent from `contents` whenever it is present, unless `--all-headings` is given, and is in `rejection_rationale` under the anchor given either way.

`contents` is every heading in document order outside the `Open questions`, `Resolved questions`, `Changelog` and `Rejection rationale` sections, whose headings, and every heading beneath them, are left out unless `--all-headings` is given. Each entry has `level`, `text`, `anchor`, `line` and `body`. An entry's body runs to the next heading of any level, so no entry holds another's prose, and it is the lines as written, so `line` plus one is its first line. A body is absent where bodies were excluded and empty where one heading directly follows another. `level` is carried rather than implied by position, because heading levels need not descend one at a time. Anything before the first heading belongs to no entry and appears only in `source`.

`questions` has `open` and `resolved`, the questions of every section of each name in document order. Each question has `text`, `anchor` and `line`, its heading's, and `body`, its detail as written. A resolved question has `answer`, with `line`, the blockquote's first line, and `body`, its text without the markers, where the question ends with a blockquote.

`changelog` is the entries of every `Changelog` section in document order. Each has `line`, its entry line; `date`, where it has one; and `body`, its text. What either holds is what was parsed, whatever lint reports. A line belonging to no question, answer or entry appears only in `source`, and in `contents` under `--all-headings`.

`rejection_rationale` is every `Rejection rationale` section in document order, whatever the document's status or type, and is empty where there is none. Each has `anchor` and `line`, its heading's, and `body`, the lines after the heading up to the next heading of level one or two as written, subsections included. It holds what was parsed, whatever lint reports.

A contents entry is not a section. A section is an H2 and everything under it, subsections included, which is what the required sections are.

Each entry in `links` has `text`, `type`, `destination` as written, `line`, and where the destination is relative and stays within the repository, `path` resolved against the repository root and `resolves_to` naming the identifier or page there, with `anchor` where one was written. A destination carrying a URI scheme or beginning with `/` has neither `path` nor `resolves_to`. `type` is `inline` for `[text](destination)`, `image` for `![text](destination)` and `definition` for `[label]: destination`, whose `text` is its label. A reference use carries a label rather than a destination and is not listed: rewriting the definition carries its uses with it.

Each glossary term has `name`, `anchor`, `path`, `definition`, `formerly` where the term was renamed, and `named_by` where a document introduced it. A term appears here and nowhere else: it is not published among the documents, so a consumer has one representation of it. The anchor is on the generated `GLOSSARY.md`, which is what every link to a term targets.

`groups` is every group in order of slug, and is empty where there are none. Each group has `slug`, `title`, `path`, `description`, `documents` and `links`. `description` is the body after the group's H1 as written, or the whole body where its first heading is not an H1, and is absent wherever contents bodies are. `documents` is the members as written, whether or not they resolve. `links` is the links in its body, in the shape a document's `links` has; a link to a group's file has `resolves_to` naming its slug. A group appears here and nowhere else: it is not published among the documents. A document's `groups` names the groups listing it, by slug.

Bodies are raw Markdown. Nothing is rendered. `source` is the file exactly as it is on disk, front matter included, and is the only field that reproduces the document.

## `--out`

```
<dir>/
├── index.json          every document, without bodies
├── glossary.json       when there is a glossary
├── groups.json         when there are groups
└── <document path>.json   one per document, with bodies
```

`index.json` omits bodies, including those of questions, answers, Changelog entries and rejection rationale sections, and source, whatever else was asked, keeping the full outline. `index.json` carries `groups` without descriptions, and `groups.json` is the array of every group, with descriptions unless `--no-bodies` is given. Each document's file sits at the document's own path with `.md` replaced by `.json`. Every path written is printed on stdout.

The command also removes what an earlier export left and this one does not write. It reads the `index.json` already in the directory before overwriting it, and takes the previous run's files from it: `index.json` itself, one file per document it lists, `glossary.json` where it recorded a glossary, and `groups.json` where it recorded groups. Each removal is reported on stderr, leaving stdout the list of files that now exist.

It therefore only ever removes a file an ArchDoc export wrote there. Where the directory holds no `index.json`, or holds one that is not an ArchDoc export, nothing is removed and the command says so. A path in that index that is not local to the directory is ignored: the index is input, and input naming somewhere else must not reach a delete. Directories left empty are not removed.

## Schema

The schema is embedded in the binary, published at `schema/export/v3.json` in the source repository, and printed by `--schema`. `v1.json` and `v2.json` stay published: each describes output that was real, and its URL is the one any consumer of that shape holds. The published copy is generated from the embedded one and a test compares them byte for byte. Its `$id` is the published copy's URL. A test compares the schema's declared properties against the exported structs by reflection. Before ArchDoc 1.0, a change that removes a field, or changes a field's type or meaning, moves `schema` to the next integer, at most once between two tagged releases. From 1.0 the shape only gains optional fields.

The output is deterministic and records no generation time.
