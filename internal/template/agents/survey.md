---
name: archdoc-survey
description: Use when working out what to document about an existing project with ArchDoc, mining git history and pull requests for decisions and their evidence, and producing a plan before any document is written.
---

# Surveying a project

Producing the list of documents a project needs, each with the evidence behind it. Do this
before writing anything.

The output is a plan you get approved. Not documents.

## Why evidence comes first

Backfilling forbids inventing what was not recorded, and requires every backfilled document to
carry a `Sources` section naming what it was reconstructed from. (PROCESS.md, Backfilling)

So the survey is not "what decisions were taken", which invites recollection and invention. It
is **"what decisions can I find a record of"**, which is a search with a verifiable answer.
Gather the evidence first and let it determine what gets written.

## Where to look

```
git log --oneline --no-merges | head -100
git log --merges --format='%h %s'                 merged pull requests
gh pr list --state merged --limit 50 --json number,title,body,mergedAt
git log --follow -p -- go.mod package.json        when dependencies changed, and why
git log --diff-filter=A --name-only               when things first appeared
```

Also read: `CHANGELOG`, `README`, any `docs/` or ad-hoc `adr/` directory, migration files,
infrastructure configuration, and code comments that explain *why* rather than *what*.

Pull request bodies are the richest source, because they usually state the problem and the
alternatives. A commit message rarely does.

## The rule that decides the kind

For each candidate, ask what you can actually support:

| What you have | What to write |
| --- | --- |
| The behaviour, and a record of why it was chosen | A backfilled RFC or ADR, citing that record |
| The behaviour, but no record of why | A **spec page**, describing what is true now |
| Background reading that informed something | A ref, with a `verified` date |
| Nothing but an inference from the code | Nothing. Note it as a gap |

That middle row matters most. A spec page describes present behaviour and needs no historical
source; a backfilled ADR claims a decision was taken and needs one. When you know *what* but
not *why*, the honest artefact is a spec page. Writing a backfilled ADR instead forces you to
invent the rationale, which is the one thing the process forbids.

Classify RFC versus ADR with the test in `classifying.md`.

## What the plan looks like

One row per candidate, and nothing written yet:

```
KIND  TITLE                             EVIDENCE
ADR   Timestamps are stored in UTC      PR #412, commit a3f19c2, migration 20240311_utc.sql
RFC   Job retry policy                  PR #388 (states the problem and two alternatives)
SPEC  Authentication token lifetime     code only; no record of why 15 minutes
REF   Message queue comparison          issue #201 thread
GAP   Why Postgres over MySQL           nothing found; ask the team
```

Present it and get it approved. Expect rows to be merged, dropped or reclassified: that
conversation is cheaper now than after forty documents exist.

## Sequencing

Write in historical order where you can, so numbers roughly track chronology. Numbers are
assigned in creation order, not decision order. (PROCESS.md, Backfilling)

Write and commit in small batches. Every terminal document is frozen by the next push, so a
batch of forty freezes forty at once and a mistake in any of them is permanent. Read each one
before pushing.

## Then

`backfilling.md` for writing them, `authoring.md` for the section requirements, `triage.md`
when lint objects.
