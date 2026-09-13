---
name: archdoc-backfill
description: Use when recording decisions or designs in an ArchDoc repository that were taken before the repository existed, including the front matter shape, the Sources requirement, and the rule against inventing what was not recorded.
---

# Backfilling

Writing up decisions that were taken elsewhere, before this repository existed.
(PROCESS.md, Backfilling)

## The rule that matters most

**Do not invent what was not recorded.**

An alternative nobody weighed, or a rationale reconstructed because it sounds convincing, is
worse than an admission of ignorance, because a later reader cannot tell it from the real
thing. Where the record does not say, write that it does not say.

This is the one rule in the whole process that no lint rule enforces and that an agent will
break by default, because producing fluent plausible prose is effortless and the result looks
exactly like a real recollection. Treat every sentence as needing a source you could point at.

Acceptable:

> `## Alternatives considered`
>
> Not recorded. The pull request discussion references a comparison against Redis but does not
> say what was measured or why Postgres was chosen.

Not acceptable:

> `## Alternatives considered`
>
> Redis was considered but rejected because the operational burden of a second datastore
> outweighed the latency benefit.

The second may even be true. It is still not permitted, because nothing in the record says it
and a later reader cannot tell that.

## How it differs from an ordinary document

A backfilled document is created **directly in a terminal status**. It does not pass through
`draft` and `proposed`, because it was never proposed. Sending it round the lifecycle would
fabricate three events that did not happen.

```
archdoc new adr "Timestamps are stored in UTC" \
  --backfill --status=accepted --created 2024-03-11 --decided 2024-04-02
```

`--status` takes a terminal status: `accepted`, `rejected` or `withdrawn`. `--created` and
`--decided` take the historical dates; omit them and you will be prompted.

Front matter gains one key no other document carries:

```yaml
created: 2024-03-11      when the work began
decided: 2024-04-02      when the decision was taken
backfilled: 2026-09-13   when this document was written
```

The historical dates go in `created` and `decided`; `backfilled` is today. That states plainly
that the document was written on one date about a decision taken on another, rather than
claiming to have existed all along.

## Sources are required

Every backfilled document carries a `Sources` section naming what it was reconstructed from,
for the same reason a ref does: the claim needs something it can be checked against. It must be
the document's last H2.

Name specific, findable things. A pull request number, a commit hash, an issue thread, a
changelog entry, a dated conversation. "Team knowledge" is not a source.

## Numbering and order

Numbers are assigned in creation order, not decision order, so a decision from three years ago
may carry a higher number than one from last week. When backfilling in bulk at the outset,
write the documents in historical order so the two roughly agree. When backfilling later, do
not renumber anything.

## Before the push

A backfilled document is frozen by the same push as any other. Forty of them in one commit
freezes forty documents at once, and a mistake in any of them is then permanent.

Read them before pushing. If you wrote them, you are the worst reviewer of whether you invented
anything, so re-read each one asking only: what is my source for this sentence?
