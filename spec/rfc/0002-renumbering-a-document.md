---
id: RFC-0002
title: Renumbering a document
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0001, ADR-0002]
updates: []
obsoletes: []
---

# RFC-0002: Renumbering a document

## Abstract

A command that changes a numbered document's identifier and follows every reference to it, refusing wherever a frozen document would be left pointing at a file that no longer exists.

## Motivation

`archdoc new` takes the next free number at the moment it runs. Two contributors branching from the same commit both receive the same one, and after both merge two documents carry it. Lint reports the collision. Repairing it means editing the filename, the front matter `id` and the H1, and doing two of the three trades one lint error for another. The same three-place edit is needed to fix a slug typed wrong at creation.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Selector | An identifier or a document path naming the document to renumber |
| Target | The identifier to take, or the next free number when none is given |
| Reference | Any place the old identifier appears: a front matter list, a wiki link, a link destination |

### Components

| Component | Responsibility |
|---|---|
| `repo.Renumber` | Finds every reference, checks nothing frozen is involved, then rewrites |
| `repo.ParseIdentifier` | Reads an identifier back into its type and number |
| `archdoc renumber` | Resolves the selector, builds the frozen predicate, prints the result |

### Steps

1. Resolve the selector. A path resolves directly. An identifier carried by exactly one document resolves to it. An identifier carried by more than one document is refused, naming every candidate.
2. Refuse if the document is frozen, as [ADR-0001](../adr/0001-terminal-documents-are-frozen.md) requires.
3. Resolve the target. With none given, take one greater than the highest number of the type. With one given, it must be an identifier of the same type, not already used, and within the 9999 bound.
4. Find every document referencing the old identifier: in `depends`, `updates`, `obsoletes` and `includes`; as `[[<id>]]`; and as a link destination naming the old file. Refuse if any of them is frozen, naming it.
5. Rewrite the document's `id` and its H1, and rewrite each reference found.
6. Write the document at its new path, keeping the slug, and remove the old file.
7. Print each path changed and note that the index is not regenerated.

Nothing is written until step 5, so a refusal leaves every file as it was.

### Rules

| Rule | Reason stated in the design |
|---|---|
| A path is accepted as well as an identifier | During a collision the identifier names two documents |
| The slug is kept | The number is the identity; the slug is for people |
| The type cannot change | The directory and the required sections follow the type |
| References in editable documents are rewritten; a frozen referrer refuses the whole operation | A reference in a frozen document could never be corrected |
| Bare mentions in prose are left alone | Rewriting a sentence is not this command's business |

### Output

```
rfc/0002-alice.md -> rfc/0003-alice.md
the index is now out of date; run archdoc index
```

### Errors

| Condition | Message names |
|---|---|
| No document has the identifier | the identifier |
| More than one document has it | every candidate path, and that a path must be given |
| The document is frozen | the identifier |
| A frozen document references it | each such document |
| The target is taken | the document holding it |
| The target is of another type | both types |
| The target is beyond 9999 | the bound |

### Out of scope

- Regenerating the index. `archdoc index` does that, and the command says so.
- Rewriting a bare identifier in prose. Not a reference the tooling defines.

## Alternatives considered

This design rests on [ADR-0001](../adr/0001-terminal-documents-are-frozen.md), which is why a frozen document and a frozen referrer are refused, and on [ADR-0002](../adr/0002-identity-is-the-filename-number.md), which is why the filename is the thing renamed and the slug kept.

- **A numberless draft, numbered by an action at merge.** Lint would have to stop requiring `id` to match the filename, and the filename to fit the grammar, for every document in every repository. A numberless document could be neither linked to nor indexed, and the numbering would happen after review.
- **A `proposals/` directory for contributions.** Discovery is by a fixed list of directories, so nothing in it would be examined by any rule. A contributor's document would pass CI with its sections missing and its links unresolved.

No other alternatives were weighed.

## Backwards compatibility

Nothing breaks. The command is new.

## Open questions

## Changelog

## Sources

- [7e1686e] adds the command and sets `created` and `decided`. Its message states the collision and the three-place edit.
- `docs/ARCHDOC.md` at [7e1686ea], section `archdoc renumber`, describes the command including the ambiguous-identifier refusal.
- The alternatives were weighed in a design session on 2026-09-13, not publicly available, and first written down on 2026-09-16.

[7e1686e]: https://github.com/ollieread/archdoc/commit/7e1686edd6835e33a573662771a5e5f5f5728746
[7e1686ea]: https://github.com/ollieread/archdoc/blob/7e1686edd6835e33a573662771a5e5f5f5728746/docs/ARCHDOC.md
