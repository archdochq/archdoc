# Export format

`archdoc export` writes a repository as JSON so that a website, a search index
or anything else can render it without reimplementing the front matter schema,
the identifier rules or the reverse-relationship graph.

```
archdoc export                    everything, one document, stdout
archdoc export --out dist/        a tree of files
archdoc export --no-bodies        metadata and structure, no prose
archdoc export --schema           the JSON Schema, no repository needed
```

The schema is the contract. It is embedded in the binary, published at
[`schema/export/v1.json`](../schema/export/v1.json), and printed by `--schema`.
This page is the guided tour; where the two disagree, the schema is right.

## One document

Bodies elided, everything else as it comes out:

```json
{
  "type": "rfc",
  "id": "RFC-0004",
  "number": 4,
  "path": "rfc/0004-database-component-on-postgresql.md",
  "title": "Database component on PostgreSQL",
  "status": "accepted",
  "created": "2026-04-01",
  "decided": "2026-05-02",

  "depends":   ["RFC-0002", "ADR-0001"],
  "updates":   [],
  "obsoletes": ["RFC-0001"],
  "includes":  [],

  "updated_by":     [],
  "obsoleted_by":   [],
  "depended_on_by": ["RFC-0005", "RFC-0007"],
  "included_in":    ["database", "glossary"],

  "effectively_obsolete": false,
  "implemented": true,
  "stale": false,

  "required_sections": [
    {"title": "Abstract",   "anchor": "abstract"},
    {"title": "Motivation", "anchor": "motivation"}
  ],
  "sections": {
    "abstract":   {"title": "Abstract",   "body": "..."},
    "motivation": {"title": "Motivation", "body": "..."}
  },

  "body": "...",
  "links": []
}
```

## The fields you could not compute yourself

Everything above the blank line is in the document. Everything below it is
derived by walking the whole repository, and is the reason this command exists.

| Field | Meaning |
| --- | --- |
| `updated_by`, `obsoleted_by`, `depended_on_by` | Identifiers of the documents naming this one. Nothing inside a document records what happened to it later |
| `included_in` | Names of the spec pages listing this document in `includes` |
| `effectively_obsolete` | An **accepted** document obsoletes this one. A claim by a draft or rejected document does not count |
| `implemented` | An accepted RFC or ADR that some spec page includes, **excluding the glossary**, which names terms rather than describing behaviour |
| `stale` | A spec page including something effectively obsolete |

The example shows the glossary distinction: `included_in` lists both `database`
and `glossary`, and `implemented` is true because of `database` alone.

## Two things that will catch you out

**`sections` is keyed by anchor, not by title.** A heading may repeat, and the
anchor walk numbers repeats the way GitHub does, so two `## Abstract` headings
become `abstract` and `abstract-1`. Keying by title would silently drop one.
Each section carries its original `title` because an anchor is lossy.

**A section in `required_sections` may be missing from `sections`.** Those
sections are checked by lint, not guaranteed, and a draft part-way through being
written is the ordinary case. The anchor is given either way so you can render a
heading and say it is not written yet.

```js
for (const {title, anchor} of doc.required_sections) {
  const section = doc.sections[anchor]
  render(title, section ? section.body : null)
}
```

Every section is an H2 and includes its subsections, so no level is recorded.
Anything not in `required_sections` is a section the author added; the process
permits them.

## Links

Bodies are raw Markdown. ArchDoc has no renderer and is not acquiring one, so
rendering and sanitising are yours. What it does give you is every relative link
already resolved, which is the one job that would otherwise force you to parse
Markdown:

```json
{
  "text": "RFC-0001",
  "destination": "0001-database.md#abstract",
  "path": "rfc/0001-database.md",
  "resolves_to": "RFC-0001",
  "anchor": "abstract",
  "line": 14
}
```

`destination` is as written. `path` is resolved against the repository root, and
`resolves_to` is the identifier, or a spec page's name, of whatever is there.
Both are absent for a destination carrying a URI scheme or beginning with `/`,
which is somewhere else entirely.

## The glossary

Emitted twice: as an ordinary spec page among the documents, and again as
structured terms, so you do not re-parse the page.

```json
{"name": "Binding", "anchor": "binding", "definition": "...", "formerly": ["Registration"]}
```

## `--out`

```
dist/
├── index.json                    every document, no bodies
├── glossary.json                 the terms, when there is a glossary
├── rfc/0004-database-....json    one file per document, bodies included
└── spec/database.json
```

`index.json` never carries bodies, whatever else was asked, because a page
rendering one RFC should not fetch the whole specification. Each document's file
sits at the document's own path with `.md` swapped for `.json`.

## Stability

`schema` is `1`. The shape may only ever gain **optional** fields, so a consumer
written against version 1 keeps working against a later ArchDoc. A field will
not change meaning or type without the version changing.

Every array is an array and never `null`, so `included_in` on a document nothing
references is `[]`. The output is deterministic and records no generation time,
so it can be cached, diffed and committed.
