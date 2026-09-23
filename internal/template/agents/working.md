---
name: archdoc-working
description: Use when making any change in an ArchDoc specification repository, for the rules that must not be broken and to find the guide covering the task in hand.
---

# Working in this repository

This is a specification repository managed by [ArchDoc](https://github.com/archdochq/archdoc).
It holds RFCs, ADRs, spec pages and refs as Markdown with YAML front matter, and enforces a
fixed process with a linter.

`PROCESS.md` is authoritative. This file summarises it for agents. Where the two disagree,
PROCESS.md is right and this file is wrong.

## Where you are running

Commands in these guides are written as though the working directory is inside this repository.

If you are working from a **different** repository, typically the code this specification
documents, add `-C` and the path to every command:

```
archdoc -C ../spec lint
```

`archdoc` finds its configuration by walking up from the working directory, so from a sibling
directory it finds nothing without it. Prefer `-C` over `cd`: each shell command may start
fresh, so a `cd` has to be repeated anyway, and naming the target every time keeps it obvious
which repository is being changed. `setup.md` covers the arrangement in full.

## Rules that are not negotiable

1. **Never create a document by hand.** Run `archdoc new rfc|adr|ref "Title"`. The number is
   the document's identity and the tool assigns it, one above the highest that exists.
   (PROCESS.md, Identity)

2. **Never edit a document whose status is `accepted`, `rejected` or `withdrawn`.** Those are
   frozen, and lint fails on any change to one. To change what an accepted document says,
   write a new document that `updates` or `obsoletes` it. This holds for typos: the record is
   worth more than the spelling. (PROCESS.md, Lifecycle)

3. **Never edit `INDEX.md`.** It is generated. Run `archdoc index`.

4. **Never invent anything when backfilling.** Where the record does not say, write that it
   does not say. A reconstructed rationale that sounds convincing is worse than an admission
   of ignorance, because a later reader cannot tell it from the real thing.
   (PROCESS.md, Backfilling)

5. **Never argue in a spec page.** The spec describes what is true now. Argument belongs in
   the RFCs and ADRs the page includes. (PROCESS.md, Document types)

6. **Never delete a document.** Withdraw it. Deleting frees its number for reuse and loses the
   record that it existed. (PROCESS.md, Identity)

7. **Run `archdoc lint` before you finish.** Exit code 2 means something is wrong. Warnings do
   not fail the build unless `--strict-warnings` is given.

## Commands

```
archdoc new rfc|adr|ref "Title"   create the next-numbered document, in draft
archdoc propose|accept|reject|withdraw <id>   move it through its lifecycle
archdoc link [--suggest]          resolve [[...]] links, or offer new ones
archdoc term add|rename|remove|list|show      maintain the glossary, one file per term
archdoc renumber <id|path> [new]  change a document's number, following every reference
archdoc lint                      check every rule
archdoc index [--check]           regenerate INDEX.md, or verify it is current
archdoc glossary [--check]        regenerate GLOSSARY.md, or verify it is current
archdoc export                    write the repository as JSON
archdoc update                    refresh PROCESS.md, the workflow and agents/ after upgrading
```

`accept` and `reject` refuse a document with an empty required section or an unresolved
`[[...]]` link, because the next push freezes it and lint would then report a fault nobody is
permitted to repair. Fix what they report rather than working around them.

## Guides

All paths are relative to this directory. Read the one that matches the task before starting.

| File | When |
| --- | --- |
| `setup.md` | Starting out: installing archdoc, finding the spec repository |
| `survey.md` | Working out what an existing project needs documented |
| `classifying.md` | Deciding whether something is an RFC, ADR, spec page or ref |
| `authoring.md` | Writing a new document and moving it through the lifecycle |
| `backfilling.md` | Recording decisions that were taken before this repository existed |
| `contributing.md` | Submitting a document by pull request to a repository you do not maintain |
| `triage.md` | `archdoc lint` reported something |
| `spec-pages.md` | Editing the spec, the glossary, or an `includes` list |
