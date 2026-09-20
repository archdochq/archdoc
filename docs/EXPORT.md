# Export format

`archdoc export` writes a repository as JSON so that a website, a search index
or anything else can render it without reimplementing the front matter schema,
the identifier rules or the reverse-relationship graph.

```
archdoc export                    everything, one document, stdout
archdoc export --out dist/        a tree of files
archdoc export --no-bodies        metadata and structure, no prose
archdoc export --source           adds each document's file as it is on disk
archdoc export --commit <rev>     records the revision this describes
archdoc export --schema           the JSON Schema, no repository needed
```

The schema is the contract. It is embedded in the binary, published at
[`schema/export/v2.json`](../schema/export/v2.json), and printed by `--schema`.
This page is the guided tour; where the two disagree, the schema is right.

## The envelope

```json
{
  "schema": 2,
  "commit": "49a14a5c2d3f9ac640bf7db8d88208254b9af105",
  "config": {"name": "ArchDoc", "branch": "main", "root": ".",
             "strict": true, "ref_stale_days": 180},
  "has_glossary": true,
  "documents": [],
  "glossary": []
}
```

`config` is every setting `archdoc.json` holds, so you can explain what frozen
means, which transitions exist, and when a ref goes stale, rather than choosing
your own answers. The location the configuration was read from is not among
them: it describes the machine that ran the command.

`commit` appears only when `--commit` was given, and is taken as written.
ArchDoc does not ask git, because a revision read from a working tree holding
uncommitted edits would describe something other than the output.

`has_glossary` is present even when `glossary` is not, because `index.json`
carries no glossary and a consumer reading only that still has to know.

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
  "contents": [
    {"level": 1, "text": "RFC-0004: Database component on PostgreSQL",
     "anchor": "rfc-0004-database-component-on-postgresql", "line": 12, "body": "..."},
    {"level": 2, "text": "Abstract",   "anchor": "abstract",   "line": 14, "body": "..."},
    {"level": 3, "text": "The rule",   "anchor": "the-rule",   "line": 20, "body": "..."},
    {"level": 2, "text": "Motivation", "anchor": "motivation", "line": 25, "body": "..."}
  ],

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

## Contents

`contents` is the document as a flat list of its headings, in the order they are
written. Rendering it is the whole job:

```js
for (const e of doc.contents) {
  emit(`<h${e.level} id="${e.anchor}">${escape(e.text)}</h${e.level}>`)
  emit(markdown(e.body))
}
```

Four things about it are worth knowing.

**A body runs to the next heading of any level**, not to the next one of equal
or higher level. So no entry holds another entry's prose, and walking the list
reads every line of the document once. The body is the lines as written and is
not trimmed, so `line` plus one is its first line.

**`level` is carried rather than implied by position.** Heading levels need not
descend one at a time, and a document going from `#` to `###` has no honest
depth. Use `level` for the tag and the nesting; do not infer either from the
order.

**Set your heading ids from `anchor`.** They are computed the way GitHub does,
and a repeat within a document takes a numbered suffix, so two `## Abstract`
headings become `abstract` and `abstract-1`. If you let your own renderer
generate ids, every link the export resolved to an anchor will miss.

**`body` is absent when bodies were excluded, and empty when one heading
directly follows another.** Those are different things, which is why it is
absent rather than `""` in the first case.

**A section in `required_sections` may be missing from `contents`.** Those are
checked by lint, not guaranteed, and a draft part-way through being written is
the ordinary case. The anchor is given either way so you can render a heading
and say it is not written yet.

```js
const byAnchor = new Map(doc.contents.map(e => [e.anchor, e]))
for (const {title, anchor} of doc.required_sections) {
  render(title, byAnchor.get(anchor)?.body ?? null)
}
```

A contents entry is not a section. A section is an H2 and everything under it,
subsections included, which is the unit `required_sections` names and the unit
lint checks. Anything not in `required_sections` is a heading the author added;
the process permits them.

Anything before the first heading belongs to no entry, and appears only in
`source`. In practice that is a blank line.

## Links

Bodies are raw Markdown. ArchDoc has no renderer and is not acquiring one, so
rendering and sanitising are yours. What it does give you is every relative link
already resolved, which is the one job that would otherwise force you to parse
Markdown:

```json
{
  "text": "RFC-0001",
  "type": "inline",
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

`type` is `inline` for `[text](destination)`, `image` for
`![text](destination)`, and `definition` for a link reference definition
`[label]: destination`, whose `text` is the label. Check it before you rewrite:
an image destination becomes an `<img>` source and an inline one becomes an
anchor, and a list of a document's outgoing links should not include its
diagrams.

A reference *use*, `[text][label]`, is not listed. It carries a label rather
than a destination, so rewriting the definition carries every use with it.

`--source` adds the file exactly as it is on disk, front matter included. It is
the only field that reproduces the document, because `contents` discards a
heading's raw spelling and everything before the first heading.

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

`index.json` never carries bodies or source, whatever else was asked, because a
page rendering one RFC should not fetch the whole specification. It keeps the
full outline, so a sidebar or a search index still works from it alone. Each
document's file sits at the document's own path with `.md` swapped for `.json`.

Nothing is removed. Exporting into a directory that already holds an older
export leaves any file this run did not write, so a document deleted from the
repository is still served from the tree.

## Stability

`schema` is `2`. From here the shape may only ever gain **optional** fields, so
a consumer written against version 2 keeps working against a later ArchDoc, and
a field will not change meaning or type without the version changing.

Version 2 is the one break. It replaced the anchor-keyed `sections` object,
which could not carry the order a document was written in and named level two
headings only, and removed a `body` that was neither the structure nor the file.
It was taken while the export was six days old and nothing consumed it.
[`v1.json`](../schema/export/v1.json) stays published: it describes output that
was real, and its URL is the one a consumer of that shape holds.

Every array is an array and never `null`, so `included_in` on a document nothing
references is `[]`. The output is deterministic and records no generation time,
so it can be cached, diffed and committed.
