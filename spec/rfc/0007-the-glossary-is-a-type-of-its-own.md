---
id: RFC-0007
title: The glossary is a type of its own
status: proposed
created: 2026-09-22
decided:
depends: [ADR-0003]
updates: [RFC-0004, RFC-0006]
obsoletes: []
---

# RFC-0007: The glossary is a type of its own

## Abstract

The glossary stops being a spec page. Each term becomes one file under `term/`, `GLOSSARY.md` is generated at the repository root beside `INDEX.md`, and the export carries terms only in `glossary`. Generated entries keep an anchor for every former name, so renaming a term cannot orphan a link a frozen document can never have repaired.

## Motivation

The glossary is already a special case everywhere except in the model. It has its own package, its own lint rule, three exclusions in link handling, a constant naming its page, and a top level array in the export. What it does not have is a type, so it is found by spec page name, exported a second time as a spec page, and carries spec page relationships.

Four things follow from that, and the last one is a defect with no repair.

**Its inclusions mean something different.** A spec page's `includes` records implementation. The glossary's records which document named a term, which is why [ADR-0014](../adr/0014-the-glossary-does-not-count-as-implementation.md) had to exclude it from `implemented`. The exclusion is incomplete: `included_in` still lists `glossary` beside real spec pages, so `ADR-0001.included_in` reads `["glossary", "lifecycle"]` and nothing distinguishes a decision described by a page from a term defined on one.

**It must keep what a spec page must drop.** A spec page describes the code as it is. The glossary must retain terms current behaviour no longer uses, because frozen documents reference them and can never be repointed. Both rules cannot hold for one page.

**It is published twice.** Every term ships in `glossary` and again in the `contents` of a `spec` document, so a consumer has two representations of one thing and no statement of which is authoritative.

**Renaming a term can break a document permanently.** `term rename` records the previous name and does not rewrite links. The old anchor is gone, L17 reports every link to it, and the fix is by hand: `archdoc link` expands `[[...]]` and never edits an existing markdown link, and `[[<old name>]]` does not resolve because a former name is not a term. A frozen document cannot be edited by hand either, so the error can never be cleared. `term remove` is the same with nothing recorded. Neither has bitten because no document links to the glossary yet, which is also why the layout can still change without breaking anything.

## Proposal

### Layout

| Path | What it is |
|---|---|
| `term/<slug>.md` | One term. The source of truth. |
| `GLOSSARY.md` | Generated at the repository root, beside `INDEX.md` and `PROCESS.md`. |

`term/` becomes a fifth document directory. `spec/glossary.md` is no longer read, and the name is released back to ordinary spec pages.

### A term

```yaml
---
title: effectively obsolete
formerly: []
named_by: ADR-0003
---
```

`title` is the term as written, and its slug is the filename. `formerly` lists previous names, oldest first, replacing the `Formerly *Old Term*.` line a rename leaves in prose. `named_by` is the document that introduced the term, replacing the page level `includes` that ADR-0014 had to carve an exception for.

A term carries no status. There is no decision in a definition to record, and nothing to transition.

### Generation

`archdoc glossary` writes `GLOSSARY.md` from `term/*.md`, and `archdoc glossary --check` fails when it is out of date, matching `archdoc index`. Entries are written in ascending case-insensitive order of `title`.

Every name in `formerly` gets an anchor in the generated page, so a link written before a rename still resolves. This is what makes rename safe and is not optional.

### Links

`[[Some Term]]` resolves to `GLOSSARY.md#some-term`, as it resolves to `spec/glossary.md#some-term` now. The target stays a single page a reader can follow with every other term around it, and it is generated whenever the index is, so it is never stale. L17 already checks that a relative link resolves to a file and to a heading when it carries an anchor, so a rename that dropped an anchor would be reported rather than discovered later.

### Commands

| Command | Effect |
|---|---|
| `archdoc term add` | Writes `term/<slug>.md`. Ordering stops being the command's business and becomes generation's. |
| `archdoc term list` | Unchanged. |
| `archdoc term show` | Unchanged. |
| `archdoc term rename` | Renames the file, appends the old name to `formerly`. Links keep resolving, because the old anchor is generated. |
| `archdoc term remove` | Deletes the file. Refuses when any frozen document links to the term, as `renumber` refuses a document a frozen document references. |

### Lint

L15 loses the three clauses that exist only because terms share a file: uniqueness becomes filename uniqueness, ordering becomes a property of generation, and every heading being an entry stops being a concept. What it keeps is the one paragraph rule, now applied per file.

### Export

Terms appear only in `glossary`, gaining `path` and `named_by`. No `term` document appears in `documents`. `has_glossary` keeps its meaning.

## Alternatives considered

- **Leaving the glossary a spec page.** The special cases already exist in five places; what is missing is the type that would make them unnecessary rather than exceptional. It also leaves the rename defect, which cannot be fixed while a rename moves an anchor nothing preserves.
- **JSON with inline Markdown.** Every document the tool manages is Markdown with front matter, read by one parser, checked by rules that assume that shape, and governed by [ADR-0007](../adr/0007-masking-never-moves-a-byte.md). A JSON type would be the only thing in the repository that is not a document, unreadable without a renderer, and would need an exception in every rule that reads a body.
- **Resolving `[[...]]` to `term/<slug>.md` instead.** It makes the generated page non load bearing, and a term referenced by a frozen document becomes a file that cannot be renamed or deleted. It was rejected because it sends a reader to a single definition with no surrounding terms, and because generating `GLOSSARY.md` with the index keeps the page as current as the files it comes from, which was the concern.
- **Keeping terms in one file and generating nothing.** Anchors would still move on rename, which is the defect.

## Backwards compatibility

`spec/glossary.md` is no longer read. A repository carrying one keeps a spec page named `glossary` that says nothing to the tool, and its terms stop being terms until they are moved into `term/`. Nothing migrates them automatically.

No document in this repository links to the glossary, so no link breaks here. A repository whose documents do link to `spec/glossary.md#term` would have those reported by L17 once the page stops being the glossary, and any of them in a frozen document could not be repaired. Such a repository should not take this change without moving its terms first.

The export gains `path` and `named_by` on each term and stops emitting the glossary as a document. A consumer reading terms from `documents` rather than `glossary` stops seeing them.

## Open questions

- Whether `archdoc term add` should refuse a slug that collides with an existing term only case-insensitively, or accept it and let generation report the anchor collision.
- Whether a repository that still has `spec/glossary.md` should be reported by lint, or left alone as an ordinary spec page.
- Whether ADR-0014 needs an ADR superseding it once page level `includes` is gone, or whether an exception to a rule that can no longer be reached is better left standing as the record of why it existed.

## Changelog
