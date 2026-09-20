---
title: Documents
includes: [ADR-0002, ADR-0004]
---

# Documents

A document is a Markdown file with a YAML front matter block. There are four types: RFC, ADR, spec page and ref. Each type lives in its own directory under `root`.

## Discovery

Every `*.md` file in `rfc/`, `adr/`, `ref/` and `spec/` is a document of that type. Files elsewhere are not documents. Subdirectories are ignored except under `spec/`, where they are permitted and a page is addressed by its path relative to `spec/`.

## Identifiers

RFCs, ADRs and refs are numbered. The identifier is `<PREFIX>-<NNNN>`: the uppercased type, a hyphen, and a four-digit zero-padded number. `RFC-0001`, `ADR-0001` and `REF-0001` are the first of each type. Each type has its own sequence, starting at 1. The next number for a type is one greater than the highest number present in that directory, whatever gaps lie below it. The sequence stops at 9999, and `new` refuses to write a document beyond it.

The identifier is derived from the filename. The `id` in the front matter is checked against it and does not define it. A filename that does not fit the grammar yields no identifier.

The filename is `<NNNN>-<slug>.md`. The slug is derived from the title at creation: lowercased, runs of non-alphanumeric characters collapsed to single hyphens, leading and trailing hyphens trimmed. The tool never renames a file. A title changed after creation leaves the slug as it was.

Spec pages are not numbered. A spec page is identified by its path relative to `spec/` without the extension: `spec/database.md` is `database`, and `spec/http/routing.md` is `http/routing`.

## Front matter

Front matter is a YAML block opened by `---` on the first line and closed by `---`. It is parsed with `gopkg.in/yaml.v3`, first into a node whose mapping keys record which keys are present, then into a typed struct. A key that is present with no value is distinguished from a key that is absent. A key not in the schema for the type is an error.

Terminal statuses are `accepted`, `rejected` and `withdrawn`.

RFC and ADR:

| Key | Type | Rules |
|---|---|---|
| `id` | string | Required. Equals the identifier derived from the type and filename. |
| `title` | string | Required, non-empty. |
| `status` | enum | Required. One of `draft`, `proposed`, `accepted`, `rejected`, `withdrawn`. |
| `created` | date | Required. ISO 8601. |
| `decided` | date or empty | Required key. Set if and only if the status is terminal. |
| `depends` | list of identifier | Required key, may be empty. RFC and ADR identifiers only. |
| `updates` | list of identifier | Required key, may be empty. Same type as this document, accepted only. |
| `obsoletes` | list of identifier | Required key, may be empty. Same type as this document, accepted only. |
| `backfilled` | date | Optional. The date the document was written to record a decision taken earlier. Implies a terminal status. |

`backfilled` is the only optional key. Every other key in the table is required to be present.

Spec:

| Key | Type | Rules |
|---|---|---|
| `title` | string | Required, non-empty. |
| `includes` | list of identifier | Required key, may be empty. RFC and ADR identifiers only. |

Ref:

| Key | Type | Rules |
|---|---|---|
| `id` | string | Required. Equals the derived identifier. |
| `title` | string | Required, non-empty. |
| `verified` | date | Required. ISO 8601. |

A byte order mark before the opening `---` is disregarded, and a carriage return after it. Neither is removed from the file.
